package proc

import (
	"encoding/json"
	"fmt"
	"go/constant"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unsafe"

	"runtime"

	"github.com/go-delve/delve/pkg/dwarf/godwarf"
)

// test all:
//
//	./test-proc.sh -v -run TestRuntimeInpsectPC

const __xgo_debug_log_enable = false

// const __xgo_debug_log_enable = true

func __xgo_debug_logf(format string, args ...interface{}) {
	if !__xgo_debug_log_enable {
		return
	}
	pc, file, line, ok := runtime.Caller(1)
	if ok {
		// baseFnName := runtime.FuncForPC(pc).Name()
		// lastSlash := strings.LastIndex(baseFnName, "/")
		// if lastSlash != -1 {
		// 	baseFnName = baseFnName[lastSlash+1:]
		// }
		_ = pc
		lineS := strconv.Itoa(line)
		// %-3s: left-align the string with a width of 3 characters
		fmt.Printf("%s:%-3s [DEBUG] ", filepath.Base(file), lineS)
	}
	fmt.Printf(format, args...)
	fmt.Println()
}

func LoadBinary(binary string) (*BinaryInfo, error) {
	bi := NewBinaryInfo(runtime.GOOS, runtime.GOARCH)
	err := bi.LoadBinaryInfo(binary, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load binary info: %v", err)
	}
	return bi, nil
}

type FuncParam struct {
	Name string
	Type godwarf.Type
}

func GetFuncParams(bi *BinaryInfo, pc uintptr) ([]FuncParam, []FuncParam, error) {
	args, err := getFuncParams(bi, pc)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get func args: %w", err)
	}
	var params []FuncParam
	var results []FuncParam
	for _, arg := range args {
		fnParam := FuncParam{
			Name: arg.name,
			Type: arg.typ,
		}
		if arg.isret {
			results = append(results, fnParam)
		} else {
			params = append(params, fnParam)
		}
	}
	return params, results, nil
}

func getFuncParams(bi *BinaryInfo, pc uintptr) ([]funcCallArg, error) {
	callerFuncName := runtime.FuncForPC(pc).Name()
	__xgo_debug_logf("CallerFuncName: %s", callerFuncName)

	fns, err := bi.FindFunction(callerFuncName)
	if err != nil {
		return nil, fmt.Errorf("failed to find function: %s %w", callerFuncName, err)
	}

	if len(fns) == 0 {
		return nil, fmt.Errorf("found no function: %s", callerFuncName)
	}
	if len(fns) != 1 {
		return nil, fmt.Errorf("found multiple functions: %s", callerFuncName)
	}

	fn := fns[0]

	_, formalArgs, err := funcCallArgs(fn, bi, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get caller formal args: %w", err)
	}
	return formalArgs, nil
}

const NON_VARIADIC_MAX = 6

// callerPC:
//
//	var callerPCs [1]uintptr
//	runtime.Callers(2, callerPCs[:])
//	callerPC := callerPCs[0]
func RetrieveStackArgs(bi *BinaryInfo, params []FuncParam, args []uintptr, variadicPtr uintptr) []*Variable {
	return parseArgs(bi, params, args, variadicPtr)
}

func parseArgs(bi *BinaryInfo, fnArgs []FuncParam, args []uintptr, variadicPtr uintptr) []*Variable {
	// Collect processed arguments
	convArgs := make([]*Variable, len(fnArgs))

	for i, arg := range args {
		convArgs[i] = parseMem(bi, arg, fnArgs[i].Name, fnArgs[i].Type)
	}

	if variadicPtr != 0 {
		p := variadicPtr
		n := len(args)
		for i := n; i < len(fnArgs); i++ {
			fnArg := fnArgs[i]
			typ := fnArg.Type
			size := typ.Size()
			align := typ.Align()
			// align p on boundary
			p = (p + uintptr(align) - 1) &^ (uintptr(align) - 1)
			convArgs[i] = parseMem(bi, p, fnArg.Name, typ)
			p += uintptr(size)
		}
	}

	return convArgs
}

func parseMem(bi *BinaryInfo, mem uintptr, name string, typ godwarf.Type) *Variable {
	v := newVarilableOpts(name, uint64(uintptr(unsafe.Pointer(mem))), typ, bi, nativeMemory{}, true)

	v.loadValue(LoadConfig{
		FollowPointers:     true,
		MaxVariableRecurse: 10,
		MaxStringLen:       10 * 1024 * 1024, // 10K
		MaxArrayValues:     1000,
		MaxStructFields:    1000,
		MaxMapBuckets:      1000,
	})

	return v
}

type ifaceHeader struct {
	itab uintptr
	data uintptr
}

type efaceHeader struct {
	typeInfo uintptr
	data     uintptr
}
type itab struct {
	_interface uintptr // Type of the interface
	_type      uintptr // Type of the concrete implementation
}

// dlv's load module data is problematic, so we manually parse interface here.
// it is much easier.
// since we cannot return concrete interface type, so we convert iface to eface
func parseInterface(addr uintptr, typ *godwarf.InterfaceType) (interface{}, uintptr, error) {
	__xgo_debug_logf("Interface Type:%v raw args: %#v", typ, addr)
	header := (*efaceHeader)(unsafe.Pointer(addr))
	if header.typeInfo == 0 || header.data == 0 {
		return nil, 0, nil
	}

	var isIface bool
	__xgo_debug_logf("inner Type: %T %#v", typ.Type, typ.Type)
	if typedDefType, ok := typ.Type.(*godwarf.TypedefType); ok {
		__xgo_debug_logf("inner defed Type: %T %#v", typedDefType.Type, typedDefType.Type)

		if structType, ok := typedDefType.Type.(*godwarf.StructType); ok {
			if structType.StructName == "runtime.iface" {
				isIface = true
			}
			// for _, field := range structType.Field {
			// 	fmt.Printf("inner struct field: %s %T %#v", field.Name, field.Type, field.Type)
			// }
		}
	}

	if !isIface {
		return *(*interface{})(unsafe.Pointer(header)), header.data, nil
	}

	// convert iface to eface
	pitab := (*itab)(unsafe.Pointer(header.typeInfo))
	if pitab == nil {
		return nil, 0, nil
	}
	eface := efaceHeader{
		typeInfo: uintptr(pitab._type),
		data:     uintptr(header.data),
	}

	val := *(*interface{})(unsafe.Pointer(&eface))
	// __xgo_debug_logf("iface value: %#v", val)
	return val, eface.data, nil
}

type StructValue []Field

type Field struct {
	Name  string
	Value interface{}
}

func (c StructValue) MarshalJSON() ([]byte, error) {
	fields := make([]string, len(c))
	for i, v := range c {
		val, err := json.Marshal(v.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal field %s: %w", v.Name, err)
		}
		fields[i] = fmt.Sprintf("%q: %s", v.Name, val)
	}
	return []byte(fmt.Sprintf("{%s}", strings.Join(fields, ","))), nil
}

type MapValue []KeyValue

type KeyValue struct {
	Key   interface{}
	Value interface{}
}

type ValueDescription struct {
	Type string
	Name string
}

func (c MapValue) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})
	for _, v := range c {
		m[fmt.Sprintf("%v", v.Key)] = v.Value
	}
	return json.Marshal(m)
}

type nativeMemory struct {
	// addr uintptr
}

var _ MemoryReadWriter = nativeMemory{}

const INF_SIZE = 1 << 30

// ReadMemory implements MemoryReadWriter.
// NOTE: panics when reading module data
func (m nativeMemory) ReadMemory(buf []byte, addr uint64) (n int, err error) {
	ptr := (*[INF_SIZE]byte)(unsafe.Pointer(uintptr(addr)))
	return copy(buf, ptr[:]), nil
}

// WriteMemory implements MemoryReadWriter.
func (m nativeMemory) WriteMemory(addr uint64, data []byte) (written int, err error) {
	ptr := (*[INF_SIZE]byte)(unsafe.Pointer(uintptr(addr)))
	return copy(ptr[:], data), nil
}

func ExportVariables(v []*Variable) ([]interface{}, error) {
	if v == nil {
		return nil, nil
	}
	exported := make([]interface{}, len(v))
	for i, v := range v {
		exportedVal, err := ExportVariable(v)
		if err != nil {
			return nil, fmt.Errorf("failed to export variable: %v", err)
		}
		exported[i] = exportedVal
	}
	return exported, nil
}

func ExportVariable(v *Variable) (interface{}, error) {
	if v.Unreadable != nil {
		return nil, nil
	}
	switch v.Kind {
	case reflect.Bool:
		return constant.BoolVal(v.Value), nil
	case reflect.Int:
		i, _ := constant.Int64Val(v.Value)
		switch v.DwarfType.Size() {
		case 8:
			return int64(i), nil
		case 4:
			return int32(i), nil
		case 2:
			return int16(i), nil
		case 1:
			return int8(i), nil
		default:
			return int(i), nil
		}
	case reflect.Uint:
		u, _ := constant.Uint64Val(v.Value)
		switch v.DwarfType.Size() {
		case 8:
			return u, nil
		case 4:
			return uint32(u), nil
		case 2:
			return uint16(u), nil
		case 1:
			return uint8(u), nil
		default:
			return uint(u), nil
		}
	case reflect.Float64:
		f, _ := constant.Float64Val(v.Value)
		return f, nil
	case reflect.String:
		return constant.StringVal(v.Value), nil
	case reflect.Slice, reflect.Array:
		slice := make([]interface{}, 0, len(v.Children))
		n := len(v.Children)
		for i := 0; i < n; i++ {
			child := &v.Children[i]
			childVal, err := ExportVariable(child)
			if err != nil {
				return nil, fmt.Errorf("failed to export child: %v", err)
			}
			slice = append(slice, childVal)
		}
		return slice, nil
	case reflect.Map:
		var keyValues []KeyValue
		n := 2 * int(v.Len)
		for i := 0; i < n; i += 2 {
			key, err := ExportVariable(&v.Children[i])
			if err != nil {
				return nil, fmt.Errorf("failed to export key: %v", err)
			}
			value, err := ExportVariable(&v.Children[i+1])
			if err != nil {
				return nil, fmt.Errorf("failed to export value: %v", err)
			}
			keyValues = append(keyValues, KeyValue{
				Key:   key,
				Value: value,
			})
		}
		return MapValue(keyValues), nil
	case reflect.Ptr:
		if len(v.Children) == 0 {
			return nil, nil
		}
		return ExportVariable(&v.Children[0])
	case reflect.Struct:
		fields := make([]Field, 0, len(v.Children))
		for _, child := range v.Children {
			fieldVal, err := ExportVariable(&child)
			if err != nil {
				return nil, fmt.Errorf("failed to export field: %v", err)
			}
			fields = append(fields, Field{
				Name:  child.Name,
				Value: fieldVal,
			})
		}
		return StructValue(fields), nil
	case reflect.Interface:
		if len(v.Children) == 0 {
			return nil, nil
		}
		if v.Native {
			return v.Children[0].InterfaceValue, nil
		}
		return ExportVariable(&v.Children[0])
	case reflect.Func:
		var name string
		if !v.Native {
			name = constant.StringVal(v.Value)
		}
		return ValueDescription{
			Type: "Func",
			Name: name,
		}, nil
	case reflect.Chan:
		return ValueDescription{
			Type: "chan",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported type: %T", v.RealType)
	}
}
