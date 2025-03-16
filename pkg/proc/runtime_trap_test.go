package proc

import (
	"fmt"
	"runtime"
	"testing"
	"unsafe"
)

func TestRuntimeTrap_InpsectType(t *testing.T) {
	bi := NewBinaryInfo(runtime.GOOS, runtime.GOARCH)
	err := bi.LoadBinaryInfo(testBin, 0, nil)

	if err != nil {
		t.Fatalf("failed to load binary info: %v", err)
	}

	fn, err := bi.FindFunction("main.main")
	if err != nil {
		t.Fatalf("failed to find function: %v", err)
	}

	for _, f := range fn {
		fmt.Printf("function: %s\n", f.Name)
	}

	if false {
		tp, err := bi.findType("main.fnMapComplex")
		if err != nil {
			t.Fatalf("failed to find type: %v", err)
		}

		fmt.Printf("type: %s\n", tp.String())
	}

	if false {
		fn, err := bi.FindFunction("main.fnMapComplex")
		if err != nil {
			t.Fatalf("failed to find type: %v", err)
		}

		for _, f := range fn {
			fmt.Printf("function: %s\n", f.Name)

			_, formalArgs, err := funcCallArgs(f, bi, true)
			if err != nil {
				t.Fatalf("failed to get formal args: %v", err)
			}

			for _, arg := range formalArgs {
				fmt.Printf("formalArg: %s, %s, isRet=%t\n", arg.name, arg.typ, arg.isret)
			}
		}
	}
}

func TestRuntimeTrap_Struct(t *testing.T) {
	ts := TestTrapStruct{
		Name: "John",
		Age:  30,
	}
	trapArgs := fnTrapStruct(ts)
	t.Logf("trapArgs: %v", trapArgs)
}

func TestRuntimeTrap_Interface(myT *testing.T) {
	t = myT
	ts := TestTrapInterfaceImpl{
		Value: "test value",
	}
	var its TestTrapInterface = &ts

	itfIface := *(*ifaceHeader)(unsafe.Pointer(&its))
	t.Logf("itfIface: %#v", itfIface)
	trapArgs := fnTrapInterface(its)

	if len(trapArgs) != 1 {
		t.Fatalf("expected 1 argument, got %d", len(trapArgs))
	}
	trapArgs0, ok := trapArgs[0].([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", trapArgs[0])
	}
	if len(trapArgs0) != 2 {
		t.Fatalf("expected 2 arguments, got %d", len(trapArgs0))
	}
	typeInfo, ok := trapArgs0[0].(uint64)
	if !ok {
		t.Fatalf("expected uintptr, got %T", trapArgs0[0])
	}
	value, ok := trapArgs0[1].(uint64)
	if !ok {
		t.Fatalf("expected uintptr, got %T", trapArgs0[1])
	}
	if typeInfo != uint64(itfIface.itab) {
		t.Fatalf("expected typeInfo to be %d, got %d", itfIface.itab, typeInfo)
	}
	if value != uint64(itfIface.value) {
		t.Fatalf("expected value to be %d, got %d", uintptr(unsafe.Pointer(&ts)), value)
	}

	createdIfaceHeader := ifaceHeader{
		itab:  uintptr(typeInfo),
		value: uintptr(value),
	}
	createdIface := *(*TestTrapInterface)(unsafe.Pointer(&createdIfaceHeader))
	if createdIface.GetValue() != ts.Value {
		t.Fatalf("expected value to be %s, got %s", ts.Value, createdIface.GetValue())
	}
}

func TestRuntimeTrap_EmptyInterface(myT *testing.T) {
	t = myT
	ts := TestTrapInterfaceImpl{
		Value: "test value",
	}
	trapArgs := fnTrapEmptyInterface(ts)
	t.Logf("trapArgs: %v", trapArgs)
}

type TestTrapStruct struct {
	Name string
	Age  int
}

func fnTrapStruct(ts TestTrapStruct) []interface{} {
	return runtime.XgoGetCallerArgs(0)
}

type TestTrapInterface interface {
	GetValue() string
}
type TestTrapInterfaceImpl struct {
	Value string
}

func (t *TestTrapInterfaceImpl) GetValue() string {
	return t.Value
}

func fnTrapInterface(ts TestTrapInterface) []interface{} {
	itfIface := (*ifaceHeader)(unsafe.Pointer(&ts))
	t.Logf("itfIface inside: %#v", itfIface)
	return runtime.XgoGetCallerArgs(0)
}

func fnTrapEmptyInterface(ts interface{}) []interface{} {
	return runtime.XgoGetCallerArgs(0)
}

func trapDebug() {
	args := runtime.XgoGetCallerArgs(1)
	t.Logf("args: %v", args)
	bi := NewBinaryInfo(runtime.GOOS, runtime.GOARCH)
	err := bi.LoadBinaryInfo(testBin, 0, nil)
	if err != nil {
		t.Fatalf("failed to load binary info: %v", err)
	}

	var pc2 [1]uintptr
	runtime.Callers(2, pc2[:])
	t.Logf("pc2: %x", pc2[0])

	fnNamePC2 := runtime.FuncForPC(pc2[0]).Name()
	t.Logf("fnNamePC2: %s", fnNamePC2)

	pcFn2 := bi.PCToFunc(uint64(pc2[0]))
	t.Logf("pcFn2: %v", pcFn2)

	// pcFn := bi.PCToFunc(uint64(pc))
	// t.Logf("pcFn: %v", pcFn)

	fns, err := bi.FindFunction("github.com/go-delve/delve/pkg/proc.add")
	if err != nil {
		t.Fatalf("failed to find function: %v", err)
	}

	if len(fns) == 0 {
		t.Fatalf("failed to find function")
	}
	if len(fns) != 1 {
		t.Fatalf("found multiple functions")
	}
	fn := fns[0]

	t.Logf("fn.Entry: %x", fn.Entry)

	fn2 := bi.PCToFunc(uint64(fn.Entry))
	t.Logf("fn2: %v", fn2)

	_, formalArgs, err := funcCallArgs(fn, bi, true)
	if err != nil {
		t.Fatalf("failed to get formal args: %v", err)
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
	convArgs, err := parseArgs(args, fnArgs)
	if err != nil {
		t.Fatalf("failed to parse arguments: %v", err)
	}

	t.Logf("convArgs: %v", convArgs)
	trapArgs = convArgs
}
