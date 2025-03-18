package proc

import (
	"fmt"
	"go/constant"
	"path/filepath"
	"reflect"
	"strconv"
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

func GetFuncParams(bi *BinaryInfo, pc uintptr) ([]FuncParam, error) {
	args, err := getFuncParams(bi, pc)
	if err != nil {
		return nil, fmt.Errorf("failed to get func args: %w", err)
	}
	params := make([]FuncParam, 0, len(args))
	for _, arg := range args {
		if arg.isret {
			continue
		}
		params = append(params, FuncParam{
			Name: arg.name,
			Type: arg.typ,
		})
	}
	return params, nil
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

// callerPC:
//
//	var callerPCs [1]uintptr
//	runtime.Callers(2, callerPCs[:])
//	callerPC := callerPCs[0]
func RetrieveStackArgs(bi *BinaryInfo, pc uintptr, args []uintptr) ([]*Variable, error) {
	params, err := getFuncParams(bi, pc)
	if err != nil {
		return nil, fmt.Errorf("failed to get func params: %w", err)
	}

	var fnArgs []funcCallArg
	fnArgs = make([]funcCallArg, 0, len(params))
	for _, arg := range params {
		if arg.isret {
			continue
		}
		fnArgs = append(fnArgs, funcCallArg{
			name:  arg.name,
			typ:   arg.typ,
			isret: arg.isret,
		})
	}

	if __xgo_debug_log_enable {
		for _, arg := range fnArgs {
			__xgo_debug_logf("formalArg: %s, %s, isRet=%t", arg.name, arg.typ, arg.isret)
		}
	}

	return parseArgs(bi, args, fnArgs), nil
}

func parseArgs(bi *BinaryInfo, args []uintptr, fnArgs []funcCallArg) []*Variable {
	// Collect processed arguments
	convArgs := make([]*Variable, len(fnArgs))

	// Process each formal argument
	for i, fnArg := range fnArgs {
		convArg := parseMem(bi, args[i], fnArg.name, fnArg.typ)
		convArgs[i] = convArg
	}

	return convArgs
}

func parseMem(bi *BinaryInfo, mem uintptr, name string, typ godwarf.Type) *Variable {
	v := newVariable(name, uint64(uintptr(unsafe.Pointer(mem))), typ, bi, nativeMemory{})

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

type Field struct {
	Name  string
	Value interface{}
}

type MapKeyValue struct {
	Key   interface{}
	Value interface{}
}

type nativeMemory struct {
	// addr uintptr
}

var _ MemoryReadWriter = nativeMemory{}

const INF_SIZE = 1 << 30

// ReadMemory implements MemoryReadWriter.
func (m nativeMemory) ReadMemory(buf []byte, addr uint64) (n int, err error) {
	// debug
	if addr < 128 {
		fmt.Printf("ReadMemory: addr=%x\n", addr)
	}
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
	switch v.Kind {
	case reflect.Bool:
		return constant.BoolVal(v.Value), nil
	case reflect.Int:
		i, _ := constant.Int64Val(v.Value)
		return i, nil
	case reflect.Uint:
		u, _ := constant.Uint64Val(v.Value)
		return u, nil
	case reflect.Float64:
		f, _ := constant.Float64Val(v.Value)
		return f, nil
	case reflect.String:
		return constant.StringVal(v.Value), nil
	case reflect.Slice:
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
		var fields []MapKeyValue
		n := int(v.Len)
		for i := 0; i < n; i += 2 {
			key, err := ExportVariable(&v.Children[i])
			if err != nil {
				return nil, fmt.Errorf("failed to export key: %v", err)
			}
			value, err := ExportVariable(&v.Children[i+1])
			if err != nil {
				return nil, fmt.Errorf("failed to export value: %v", err)
			}
			fields = append(fields, MapKeyValue{
				Key:   key,
				Value: value,
			})
		}
		return fields, nil
	case reflect.Ptr:
		if v.Unreadable != nil {
			return nil, nil
		}
		elem, err := ExportVariable(&v.Children[0])
		if err != nil {
			return nil, fmt.Errorf("failed to export ptr: %v", err)
		}
		return elem, nil
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
		return fields, nil
	case reflect.Func:
		if v.Unreadable != nil {
			return nil, nil
		}
		return FuncVal{
			Name: constant.StringVal(v.Value),
		}, nil
	case reflect.Chan:
		return ChannelVal{
			Type: "chan",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported type: %T", v.RealType)
	}
}

type FuncVal struct {
	Name string
}

type ChannelVal struct {
	Type string
}
