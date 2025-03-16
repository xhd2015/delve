package proc

import (
	"fmt"
	"path/filepath"
	"strconv"
	"unsafe"

	"runtime"

	"github.com/go-delve/delve/pkg/dwarf/godwarf"
)

// test all:
//
//	./test-proc.sh -v -run TestRuntimeTrap -run TestRuntimeInpsectPC
//
// test specific:
//
//	./test-proc.sh -count 1 -v -run TestRuntimeInpsectType

const __xgo_debug_log_enable = false

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
func RetrieveStackArgs(bi *BinaryInfo, pc uintptr, stackArgs []interface{}) ([]interface{}, error) {
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

	__xgo_debug_logf("Number of expected args: %d", len(fnArgs))
	__xgo_debug_logf("Number of actual args: %d", len(stackArgs))
	__xgo_debug_logf("Actual args: %#v", stackArgs)

	return parseArgs(stackArgs, fnArgs)
}

func parseArgs(args []interface{}, fnArgs []funcCallArg) ([]interface{}, error) {
	// Collect processed arguments
	convArgs := make([]interface{}, len(fnArgs))

	// Process each formal argument
	for i, fnArg := range fnArgs {
		convArg, err := parseArg(args[i], fnArg.typ)
		if err != nil {
			return nil, fmt.Errorf("failed to parse argument: %s %v", fnArg.name, err)
		}

		__xgo_debug_logf("Parsed %d", i)
		__xgo_debug_logf("Parsed arg %s: %#v", fnArg.name, convArg)
		convArgs[i] = convArg
	}

	return convArgs, nil
}

type stringHeader struct {
	data uintptr
	len  uint64
}

// sliceHeader represents the runtime representation of a slice
type sliceHeader struct {
	data uintptr
	len  int
	cap  int
}

func parseArg(arg interface{}, typ godwarf.Type) (interface{}, error) {
	if arg == nil {
		return nil, fmt.Errorf("argument is nil")
	}
	__xgo_debug_logf("parseArg for type %s", typ)

	switch t := typ.(type) {
	case *godwarf.IntType:
		__xgo_debug_logf("handling IntType: %#v", arg)
		return int(arg.(uint64)), nil
	case *godwarf.BoolType:
		__xgo_debug_logf("handling BoolType: %#v", arg)
		// Boolean values are typically represented as uint8 (0=false, 1=true)
		boolVal := arg.(uint64)
		return boolVal != 0, nil
	case *godwarf.StringType:
		__xgo_debug_logf("handling StringType: %#v", arg)
		return parseStringArg(arg)
	case *godwarf.FuncType:
		__xgo_debug_logf("handling FuncType: %#v", arg)
		// Function pointers are represented as addresses
		funcPC := arg.(uint64)
		// For a function, we return the function pointer/PC
		return uintptr(funcPC), nil
	case *godwarf.StructType:
		__xgo_debug_logf("handling StructType: %#v", arg)
		return parseStructArg(arg, t)
	case *godwarf.SliceType:
		__xgo_debug_logf("handling SliceType: %#v", arg)
		return parseSliceArg(arg, t)
	case *godwarf.InterfaceType:
		__xgo_debug_logf("handling InterfaceType: %#v", arg)
		return parseInterfaceArg(arg, t)
	default:
		// We should not get here for types that need multiple arguments
		// Those should be handled by parseComplexArg
		return nil, fmt.Errorf("unsupported single-argument type: %T, %s", t, t)
	}
}

func parseStringArg(arg interface{}) (string, error) {
	if arg == nil {
		return "", fmt.Errorf("argument is nil")
	}

	argSlice, ok := arg.([]interface{})
	if !ok {
		return "", fmt.Errorf("expected []interface{}, got %T", arg)
	}
	if len(argSlice) != 2 {
		return "", fmt.Errorf("expected 2 args, got %d", len(argSlice))
	}

	dataPtr, ok := argSlice[0].(uint64)
	if !ok {
		return "", fmt.Errorf("expected uint64 for string data pointer, got %T", argSlice[0])
	}

	length, ok := argSlice[1].(uint64)
	if !ok {
		return "", fmt.Errorf("expected uint64 for string length, got %T", argSlice[1])
	}

	return string(unsafe.String((*byte)(unsafe.Pointer(uintptr(dataPtr))), length)), nil
}

// parseStructArg processes a struct's fields from flattened runtime arguments
func parseStructArg(args interface{}, typ *godwarf.StructType) (interface{}, error) {
	__xgo_debug_logf("Struct raw args: %#v", args)

	argsSlice, ok := args.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected []interface{}, got %T", args)
	}
	if len(argsSlice) != len(typ.Field) {
		return nil, fmt.Errorf("expected %d args, got %d", len(typ.Field), len(argsSlice))
	}

	// Create a map to hold the struct fields
	result := make(map[string]interface{}, len(typ.Field))

	// Process each field in the struct
	for i, field := range typ.Field {
		fieldValue, err := parseArg(argsSlice[i], field.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to parse field %s: %v", field.Name, err)
		}
		result[field.Name] = fieldValue
	}

	return result, nil
}

func parseSliceArg(args interface{}, typ *godwarf.SliceType) (interface{}, error) {
	__xgo_debug_logf("Slice raw args: %#v", args)
	argsSlice, ok := args.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected []interface{}, got %T", args)
	}

	if len(argsSlice) != 3 {
		return nil, fmt.Errorf("expected 3 args, got %d", len(argsSlice))
	}

	dataPtr, ok := argsSlice[0].(uint64)
	if !ok {
		return nil, fmt.Errorf("expected uint64 for slice data pointer, got %T", argsSlice[0])
	}

	length, ok := argsSlice[1].(uint64)
	if !ok {
		return nil, fmt.Errorf("expected uint64 for slice length, got %T", argsSlice[1])
	}

	capacity, ok := argsSlice[2].(uint64)
	if !ok {
		return nil, fmt.Errorf("expected uint64 for slice capacity, got %T", argsSlice[2])
	}

	sliceHeader := sliceHeader{
		data: uintptr(dataPtr),
		len:  int(length),
		cap:  int(capacity),
	}

	__xgo_debug_logf("sliceHeader: %#v", sliceHeader)

	switch typ.ElemType.(type) {
	case *godwarf.IntType:
		__xgo_debug_logf("Returning []int from sliceHeader: %#v", sliceHeader)
		return *(*[]int)(unsafe.Pointer(&sliceHeader)), nil
	case *godwarf.StringType:
		__xgo_debug_logf("Returning []string from sliceHeader: %#v", sliceHeader)

		ptr := (*[16]byte)(unsafe.Pointer(sliceHeader.data))
		// hexdump 16 bytes from ptr
		if __xgo_debug_log_enable {
			__xgo_debug_logf("hexdump 16 bytes from ptr: %x\n", ptr[:16])
			for i := 0; i < 2; i++ {
				for j := 0; j < 8; j++ {
					fmt.Printf("%02x ", ptr[i*8+j])
				}
				fmt.Println()
			}
		}

		// __xgo_debug_logf("a: %#v", a)

		// str00 := unsafeStringAt(uintptr(unsafe.Pointer(&a)))
		// __xgo_debug_logf("str00: %s", str00)

		str0 := unsafeStringAt(sliceHeader.data)
		__xgo_debug_logf("str0: %s", str0)
		strSlice := *(*[]string)(unsafe.Pointer(&sliceHeader))
		__xgo_debug_logf("strSlice: %#v", strSlice)
		return *(*[]string)(unsafe.Pointer(&sliceHeader)), nil
	default:
		return nil, fmt.Errorf("unsupported slice element type: %s", typ.ElemType)
	}
}

type ifaceHeader struct {
	itab  uintptr
	value uintptr
}

type efaceHeader struct {
	typeInfo uintptr
	value    uintptr
}

// since we cannot return concrete interface type, so we convert iface to eface
func parseInterfaceArg(args interface{}, typ *godwarf.InterfaceType) (interface{}, error) {
	__xgo_debug_logf("Interface Type:%v raw args: %#v", typ, args)

	argsSlice, ok := args.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected []interface{}, got %T", args)
	}
	if len(argsSlice) != 2 {
		return nil, fmt.Errorf("expected 2 args, got %d", len(argsSlice))
	}

	typePtr, ok := argsSlice[0].(uint64)
	if !ok {
		return nil, fmt.Errorf("expected uint64 for interface type pointer, got %T", argsSlice[0])
	}

	valuePtr, ok := argsSlice[1].(uint64)
	if !ok {
		return nil, fmt.Errorf("expected uint64 for interface value pointer, got %T", argsSlice[1])
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
		eh := efaceHeader{
			typeInfo: uintptr(typePtr),
			value:    uintptr(valuePtr),
		}
		eface := *(*efaceHeader)(unsafe.Pointer(&eh))
		__xgo_debug_logf("eface value: %#v", eface)
		return *(*interface{})(unsafe.Pointer(&eface)), nil
	}

	// convert iface to eface
	pitab := (*itab)(unsafe.Pointer(uintptr(typePtr)))
	eface := efaceHeader{
		typeInfo: uintptr(pitab._type),
		value:    uintptr(valuePtr),
	}

	val := *(*interface{})(unsafe.Pointer(&eface))
	__xgo_debug_logf("iface value: %#v", val)
	return val, nil
}

type itab struct {
	_interface uintptr // Type of the interface
	_type      uintptr // Type of the concrete implementation
}

func unsafeStringAt(ptr uintptr) string {
	return *(*string)(unsafe.Pointer(ptr))
}

func toStringHeader(str string) stringHeader {
	return *(*stringHeader)(unsafe.Pointer(&str))
}
