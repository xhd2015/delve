package proc

import (
	"fmt"
	"unsafe"

	"runtime"

	"github.com/go-delve/delve/pkg/dwarf/godwarf"
)

// test all:
//
//	./test-proc.sh
//
// test specific:
//
//	./test-proc.sh -count 1 -v -run TestRuntimeInpsectType
const testBin = "./__debug_bin_test"

var trapArgs []interface{}

func trap() {
	args := runtime.XgoGetCallerArgs(1)
	bi := NewBinaryInfo(runtime.GOOS, runtime.GOARCH)
	err := bi.LoadBinaryInfo(testBin, 0, nil)
	if err != nil {
		panic(fmt.Errorf("failed to load binary info: %v", err))
	}

	var callerPCs [1]uintptr
	runtime.Callers(2, callerPCs[:])

	callerPC := callerPCs[0]

	callerFuncName := runtime.FuncForPC(callerPC).Name()
	fmt.Printf("DEBUG: CallerFuncName: %s\n", callerFuncName)

	fns, err := bi.FindFunction(callerFuncName)
	if err != nil {
		panic(fmt.Errorf("failed to find function: %s %v", callerFuncName, err))
	}

	if len(fns) == 0 {
		panic(fmt.Errorf("found no functions: %s", callerFuncName))
	}
	if len(fns) != 1 {
		panic(fmt.Errorf("found multiple functions: %s", callerFuncName))
	}
	fn := fns[0]

	_, formalArgs, err := funcCallArgs(fn, bi, true)
	if err != nil {
		panic(fmt.Errorf("failed to get caller formal args: %v", err))
	}

	var fnArgs []funcCallArg
	fnArgs = make([]funcCallArg, 0, len(formalArgs))
	for _, arg := range formalArgs {
		if arg.isret {
			continue
		}
		fnArgs = append(fnArgs, funcCallArg{
			name:  arg.name,
			typ:   arg.typ,
			isret: arg.isret,
		})
	}

	for _, arg := range fnArgs {
		fmt.Printf("formalArg: %s, %s, isRet=%t\n", arg.name, arg.typ, arg.isret)
	}

	fmt.Printf("DEBUG: Number of expected args: %d\n", len(fnArgs))
	fmt.Printf("DEBUG: Number of actual args: %d\n", len(args))
	fmt.Printf("DEBUG: Actual args: %#v\n", args)

	convArgs, err := parseArgs(args, fnArgs)
	if err != nil {
		panic(fmt.Errorf("failed to parse arguments: %v", err))
	}

	trapArgs = convArgs
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
		fmt.Printf("DEBUG: Parsed %d\n", i)
		fmt.Printf("DEBUG: Parsed arg %s: %#v\n", fnArg.name, convArg)
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
	fmt.Printf("DEBUG: parseArg for type %s\n", typ)

	switch t := typ.(type) {
	case *godwarf.IntType:
		fmt.Printf("DEBUG: Handling IntType: %#v\n", arg)
		return int(arg.(uint64)), nil
	case *godwarf.BoolType:
		fmt.Printf("DEBUG: Handling BoolType: %#v\n", arg)
		// Boolean values are typically represented as uint8 (0=false, 1=true)
		boolVal := arg.(uint64)
		return boolVal != 0, nil
	case *godwarf.StringType:
		fmt.Printf("DEBUG: Handling StringType: %#v\n", arg)
		return parseStringArg(arg)
	case *godwarf.FuncType:
		fmt.Printf("DEBUG: Handling FuncType: %#v\n", arg)
		// Function pointers are represented as addresses
		funcPC := arg.(uint64)
		// For a function, we return the function pointer/PC
		return uintptr(funcPC), nil
	case *godwarf.StructType:
		fmt.Printf("DEBUG: Handling StructType: %#v\n", arg)
		return parseStructArg(arg, t)
	case *godwarf.SliceType:
		fmt.Printf("DEBUG: Handling SliceType: %#v\n", arg)
		return parseSliceArg(arg, t)
	case *godwarf.InterfaceType:
		fmt.Printf("DEBUG: Handling InterfaceType: %#v\n", arg)
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
	fmt.Printf("DEBUG: Struct raw args: %#v\n", args)

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
	fmt.Printf("DEBUG: Slice raw args: %#v\n", args)
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

	fmt.Printf("DEBUG: sliceHeader: %#v\n", sliceHeader)

	switch typ.ElemType.(type) {
	case *godwarf.IntType:
		fmt.Printf("DEBUG: Returning []int from sliceHeader: %#v\n", sliceHeader)
		return *(*[]int)(unsafe.Pointer(&sliceHeader)), nil
	case *godwarf.StringType:
		fmt.Printf("DEBUG: Returning []string from sliceHeader: %#v\n", sliceHeader)

		ptr := (*[16]byte)(unsafe.Pointer(sliceHeader.data))
		// hexdump 16 bytes from ptr
		fmt.Printf("DEBUG: hexdump 16 bytes from ptr: %x\n", ptr[:16])
		for i := 0; i < 2; i++ {
			for j := 0; j < 8; j++ {
				fmt.Printf("%02x ", ptr[i*8+j])
			}
			fmt.Printf("\n")
		}
		fmt.Printf("\n")

		// fmt.Printf("DEBUG: a: %#v\n", a)

		// str00 := unsafeStringAt(uintptr(unsafe.Pointer(&a)))
		// fmt.Printf("DEBUG: str00: %s\n", str00)

		str0 := unsafeStringAt(sliceHeader.data)
		fmt.Printf("DEBUG: str0: %s\n", str0)
		strSlice := *(*[]string)(unsafe.Pointer(&sliceHeader))
		fmt.Printf("DEBUG: strSlice: %#v\n", strSlice)
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
	fmt.Printf("DEBUG: Interface T:%v raw args: %#v\n", typ, args)

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
	fmt.Printf("DEBUG: inner Type: %T %#v\n", typ.Type, typ.Type)
	if typedDefType, ok := typ.Type.(*godwarf.TypedefType); ok {
		fmt.Printf("DEBUG: inner defed Type: %T %#v\n", typedDefType.Type, typedDefType.Type)

		if structType, ok := typedDefType.Type.(*godwarf.StructType); ok {
			if structType.StructName == "runtime.iface" {
				isIface = true
			}
			// for _, field := range structType.Field {
			// 	fmt.Printf("DEBUG: inner struct field: %s %T %#v\n", field.Name, field.Type, field.Type)
			// }
		}
	}

	if !isIface {
		eh := efaceHeader{
			typeInfo: uintptr(typePtr),
			value:    uintptr(valuePtr),
		}
		eface := *(*efaceHeader)(unsafe.Pointer(&eh))
		fmt.Printf("DEBUG: eface value: %#v\n", eface)
		return *(*interface{})(unsafe.Pointer(&eface)), nil
	}

	// convert iface to eface
	pitab := (*itab)(unsafe.Pointer(uintptr(typePtr)))
	eface := efaceHeader{
		typeInfo: uintptr(pitab._type),
		value:    uintptr(valuePtr),
	}

	val := *(*interface{})(unsafe.Pointer(&eface))
	fmt.Printf("DEBUG: iface value: %#v\n", val)
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
