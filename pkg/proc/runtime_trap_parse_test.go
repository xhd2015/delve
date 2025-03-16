package proc

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

var t *testing.T

// TestStruct is a simple struct for testing purposes
type TestStruct struct {
	Name string
	Age  int
}

// TestInterface is used for interface testing
type TestInterface interface {
	GetValue() string
}

// TestInterfaceImpl implements TestInterface
type TestInterfaceImpl struct {
	Value string
}

func (t *TestInterfaceImpl) GetValue() string {
	return t.Value
}

func TestRuntimeInpsectPC_String(myT *testing.T) {
	t = myT

	s := fnString("hello")
	expect := "hello"
	if s != expect {
		t.Fatalf("expected '%s', got '%s'", expect, s)
	}
	expectTrapArgs := []interface{}{"hello"}
	if !reflect.DeepEqual(trapArgs, expectTrapArgs) {
		t.Fatalf("expected %v, got %v", expectTrapArgs, trapArgs)
	}
}
func TestRuntimeInpsectPC_StrInt(myT *testing.T) {
	t = myT

	s := fnStrInt("hello", 1)
	expect := "hello, 1"
	if s != expect {
		t.Fatalf("expected '%s', got '%s'", expect, s)
	}
	expectTrapArgs := []interface{}{"hello", 1}
	if !reflect.DeepEqual(trapArgs, expectTrapArgs) {
		t.Fatalf("expected %v, got %v", expectTrapArgs, trapArgs)
	}
}

func TestRuntimeInpsectPC_SliceInt(myT *testing.T) {
	t = myT

	nums := []int{1, 2, 3}
	s := fnSliceInt(nums)
	expect := "[1 2 3]"
	if s != expect {
		t.Fatalf("expected '%s', got '%s'", expect, s)
	}
	expectTrapArgs := []interface{}{nums}
	if !reflect.DeepEqual(trapArgs, expectTrapArgs) {
		t.Fatalf("expected %v, got %v", expectTrapArgs, trapArgs)
	}
}

// possible reason is: when the function returned
// the stack is destroyed, so the string is not valid anymore
// anything on stack is not persistent
func TestRuntimeInpsectPC_SliceOneString(myT *testing.T) {
	t = myT

	strs := []string{
		"hello",

		// , "world", "test"
	}

	sliceHeader1 := (*sliceHeader)(unsafe.Pointer(&strs))
	t.Logf("sliceHeader1: %#v", sliceHeader1)
	// hexdump(uintptr(unsafe.Pointer(sliceHeader1.data)), 2)

	// ptrToStr0 := &strs[0]
	str0Header := toStringHeader(strs[0])
	t.Logf("str0Header: %#v", str0Header)

	str0Unsafe := unsafeStringAt(uintptr(unsafe.Pointer(&str0Header)))
	t.Logf("str0Unsafe: %s", str0Unsafe)

	s := fnSliceString(strs)
	expect := "hello"
	if s != expect {
		t.Fatalf("expected '%s', got '%s'", expect, s)
	}
	ptr := unsafe.Pointer(&trapArgs[0])
	sliceHeader := (*sliceHeader)(ptr)
	t.Logf("sliceHeader.data: %x", sliceHeader.data)
	t.Logf("sliceHeader.len: %d", sliceHeader.len)
	t.Logf("sliceHeader.cap: %d", sliceHeader.cap)
	expectTrapArgs := []interface{}{[]string{"hello"}}
	if !reflect.DeepEqual(trapArgs, expectTrapArgs) {
		t.Fatalf("expected %v, got %v", expectTrapArgs, trapArgs)
	}
}

func TestRuntimeInpsectPC_SliceMultiString(myT *testing.T) {
	t = myT

	strs := []string{
		"hello",
		"world",
		"test",
	}

	sliceHeader1 := (*sliceHeader)(unsafe.Pointer(&strs))
	t.Logf("sliceHeader1: %#v", sliceHeader1)
	// hexdump(uintptr(unsafe.Pointer(sliceHeader1.data)), 2)

	// ptrToStr0 := &strs[0]
	str0Header := toStringHeader(strs[0])
	t.Logf("str0Header: %#v", str0Header)

	str0Unsafe := unsafeStringAt(uintptr(unsafe.Pointer(&str0Header)))
	t.Logf("str0Unsafe: %s", str0Unsafe)

	s := fnSliceString(strs)
	expect := "hello,world,test"
	if s != expect {
		t.Fatalf("expected '%s', got '%s'", expect, s)
	}
	expectTrapArgs := []interface{}{[]string{"hello", "world", "test"}}
	if !reflect.DeepEqual(trapArgs, expectTrapArgs) {
		t.Fatalf("expected %v, got %v", expectTrapArgs, trapArgs)
	}
}

// func hexdump(ptr uintptr, group int) {
// 	bytesPtr := (*[1000]byte)(unsafe.Pointer(ptr))
// 	for i := 0; i < group; i++ {
// 		for j := 0; j < 8; j++ {
// 			fmt.Printf("%02x ", bytesPtr[i*8+j])
// 		}
// 		fmt.Printf("\n")
// 	}
// 	fmt.Printf("\n")
// }

func TestRuntimeInpsectPC_Bool(myT *testing.T) {
	t = myT

	b := fnBool(true)
	expect := "Value: true"
	if b != expect {
		t.Fatalf("expected '%s', got '%s'", expect, b)
	}
	expectTrapArgs := []interface{}{true}
	if !reflect.DeepEqual(trapArgs, expectTrapArgs) {
		t.Fatalf("expected %v, got %v", expectTrapArgs, trapArgs)
	}
}

func TestRuntimeInpsectPC_Func(myT *testing.T) {
	t = myT

	testFunc := func() string { return "test" }
	result := fnFunc(testFunc)
	expect := "Got a function"
	if result != expect {
		t.Fatalf("expected '%s', got '%s'", expect, result)
	}

	// We can't directly compare functions with DeepEqual, so we just check that trapArgs has one element
	if len(trapArgs) != 1 {
		t.Fatalf("expected 1 argument, got %d", len(trapArgs))
	}
}

func TestRuntimeInpsectPC_Struct(myT *testing.T) {
	t = myT

	testStruct := TestStruct{
		Name: "John",
		Age:  30,
	}
	result := fnStruct(testStruct)
	expect := "Name: John, Age: 30"
	if result != expect {
		t.Fatalf("expected '%s', got '%s'", expect, result)
	}

	if len(trapArgs) != 1 {
		t.Fatalf("expected 1 argument, got %d", len(trapArgs))
	}
	arg0, ok := trapArgs[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", trapArgs[0])
	}
	if len(arg0) != 2 {
		t.Errorf("expected 2 keys, got %d", len(arg0))
	}
	if arg0["Name"] != "John" {
		t.Errorf("expected 'John', got '%s'", arg0["Name"])
	}
	if arg0["Age"] != 30 {
		t.Errorf("expected 30, got %d", arg0["Age"])
	}
}

func TestRuntimeInpsectPC_EmptyInterface(myT *testing.T) {
	t = myT
	// t.Skip("TODO fix")

	var testInterface TestInterface = &TestInterfaceImpl{Value: "test value"}
	result := fnEmptyInterface(testInterface)
	expect := "EmptyInterface value: &{test value}"
	if result != expect {
		t.Fatalf("expected '%s', got '%s'", expect, result)
	}

	// Since interfaces are complex types, we just check that we have a value
	if len(trapArgs) != 1 {
		t.Fatalf("expected 1 argument, got %d", len(trapArgs))
	} else {
		arg0, ok := trapArgs[0].(TestInterface)
		if !ok {
			t.Errorf("expected TestInterface, got %T", trapArgs[0])
		}
		expect := "test value"
		if arg0.GetValue() != expect {
			t.Errorf("expected '%s', got '%s'", expect, arg0.GetValue())
		}
	}
}

func TestRuntimeInpsectPC_Interface(myT *testing.T) {
	t = myT

	var ts TestInterface = &TestInterfaceImpl{Value: "test value"}

	itfIface := *(*ifaceHeader)(unsafe.Pointer(&ts))
	t.Logf("begin iface: %#v", itfIface)
	result := fnInterface(ts)
	expect := "Interface value: test value"
	if result != expect {
		t.Fatalf("expected '%s', got '%s'", expect, result)
	}

	if len(trapArgs) != 1 {
		t.Errorf("expected 1 argument, got %d", len(trapArgs))
	} else {
		arg0, ok := trapArgs[0].(TestInterface)
		if !ok {
			t.Errorf("expected TestInterface, got %T", trapArgs[0])
		} else {
			expect := "test value"
			if arg0.GetValue() != expect {
				t.Errorf("expected '%s', got '%s'", expect, arg0.GetValue())
			}
		}
	}
}

func fnString(s string) string {
	trap()
	return s
}

func fnStrInt(s string, b int) string {
	trap()
	return fmt.Sprintf("%s, %d", s, b)
}

func fnSliceInt(nums []int) string {
	trap()
	return fmt.Sprintf("%v", nums)
}

func fnSliceString(strs []string) string {
	trap()
	return strings.Join(strs, ",")
}

func fnBool(b bool) string {
	trap()
	return fmt.Sprintf("Value: %v", b)
}

func fnFunc(fn func() string) string {
	trap()
	return "Got a function"
}

func fnStruct(s TestStruct) string {
	trap()
	return fmt.Sprintf("Name: %s, Age: %d", s.Name, s.Age)
}

func fnEmptyInterface(i interface{}) string {
	trap()
	return fmt.Sprintf("EmptyInterface value: %v", i)
}

func fnInterface(i TestInterface) string {
	trap()
	return fmt.Sprintf("Interface value: %s", i.GetValue())
}
