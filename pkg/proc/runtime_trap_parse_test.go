package proc

import (
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

const testBin = "./__debug_bin_test"

var bi *BinaryInfo

var t *testing.T

var trapArgs []interface{}

func init() {
	bi = NewBinaryInfo(runtime.GOOS, runtime.GOARCH)
	err := bi.LoadBinaryInfo(testBin, 0, nil)
	if err != nil {
		panic(fmt.Errorf("failed to load binary info: %v", err))
	}
}

func trap() {
	var args []uintptr
	pc := runtime.XgoGetCallerArgs(1, func(addr uintptr, size uint) {
		args = append(args, addr)
	})

	for i, arg := range args {
		fmt.Printf("arg[%d]: %x\n", i, arg)
	}

	var err error
	convArgs, err := RetrieveStackArgs(bi, pc, args)
	if err != nil {
		panic(fmt.Errorf("failed to retrieve stack args: %w", err))
	}

	trapArgs, err = ExportVariables(convArgs)
	if err != nil {
		panic(fmt.Errorf("failed to export variables: %w", err))
	}
}

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

func TestRuntimeInpsectPC_String(_t *testing.T) {
	t = _t

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
func TestRuntimeInpsectPC_StrInt(_t *testing.T) {
	t = _t

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

func TestRuntimeInpsectPC_SliceInt(_t *testing.T) {
	t = _t

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

func TestRuntimeInpsectPC_Bool(_t *testing.T) {
	t = _t

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

func TestRuntimeInpsectPC_Func(_t *testing.T) {
	t = _t

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

func TestRuntimeInpsectPC_Struct(_t *testing.T) {
	t = _t

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

	arg0, ok := trapArgs[0].([]Field)
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", trapArgs[0])
	}
	checkFields(t, arg0, []Field{{Name: "Name", Value: "John"}, {Name: "Age", Value: int64(30)}})
}

func TestRuntimeInpsectPC_EmptyInterface(_t *testing.T) {
	t = _t

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

func TestRuntimeInpsectPC_Interface(_t *testing.T) {
	t = _t

	var ts TestInterface = &TestInterfaceImpl{Value: "test value"}

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

func TestRuntimeInpsectPC_Map(_t *testing.T) {
	t = _t

	m := map[string]int{"a": 1, "b": 2, "c": 3}
	result := fnMap(m)
	expect := "Map value: map[a:1 b:2 c:3]"
	if result != expect {
		t.Fatalf("expected '%s', got '%s'", expect, result)
	}

	if len(trapArgs) != 1 {
		t.Errorf("expected 1 argument, got %d", len(trapArgs))
	}
	arg0, ok := trapArgs[0].([]MapKeyValue)
	if !ok {
		t.Fatalf("expected []MapKeyValue, got %T", trapArgs[0])
	}
	checkExpectMap(t, arg0, []MapKeyValue{{"a", 1}, {"b", 2}, {"c", 3}})
}

func checkExpectMap(t *testing.T, arg0 []MapKeyValue, expect []MapKeyValue) {
	if len(arg0) != len(expect) {
		t.Errorf("expected %d keys, got %d", len(expect), len(arg0))
		return
	}
	for i, kv := range arg0 {
		if kv.Key != expect[i].Key {
			t.Errorf("expected 'a', got '%s'", arg0[0].Key)
		}
		if kv.Value != expect[i].Value {
			t.Errorf("expected 1, got %d", arg0[0].Value)
		}
	}
}

func checkFields(t *testing.T, arg0 []Field, expect []Field) {
	if len(arg0) != len(expect) {
		t.Errorf("expected %d keys, got %d", len(expect), len(arg0))
		return
	}
	for i, kv := range arg0 {
		if kv.Name != expect[i].Name {
			t.Errorf("expected '%s', got '%s'", expect[i].Name, kv.Name)
		}
		if kv.Value != expect[i].Value {
			t.Errorf("expected '%v', got '%v'", expect[i].Value, kv.Value)
		}
	}
}

func TestRuntimeInpsectPC_Ptr(_t *testing.T) {
	t = _t

	i := 10
	p := fnPtr(&i)
	expect := "Pointer value: 10"
	if p != expect {
		t.Fatalf("expected '%s', got '%s'", expect, p)
	}
	if len(trapArgs) != 1 {
		t.Fatalf("expected 1 argument, got %d", len(trapArgs))
	}
	arg0, ok := trapArgs[0].(int)
	if !ok {
		t.Fatalf("expected int, got %T", trapArgs[0])
	} else {
		expect := 10
		if arg0 != expect {
			t.Errorf("expected %d, got %d", expect, arg0)
		}
	}
}

const NUM_COMPLEX_STRUCT_FIELDS = 11

type ComplexStruct struct {
	Str                string
	Int                int
	Bool               bool
	StrAfterBool       string
	TestStruct         TestStruct
	StrAfterTestStruct string
	Int32              int32
	Int16              int16
	IntArray           [3]int
	StrSlice           []string
	Map                map[string]int
}

func TestRuntimeInpsectPC_PtrStruct(_t *testing.T) {
	t = _t

	testStruct := ComplexStruct{
		Str:                "Hello",
		Int:                10,
		Bool:               true,
		StrAfterBool:       "World",
		TestStruct:         TestStruct{}, // nested
		StrAfterTestStruct: "x1",
		Int32:              123,
		Int16:              456,
		IntArray:           [3]int{1, 2, 3},
		StrSlice:           []string{"a", "b", "c"},
		Map:                map[string]int{"a": 1, "b": 2, "c": 3},
	}
	fnPtrStruct(&testStruct)
	if len(trapArgs) != 1 {
		t.Fatalf("expected 1 argument, got %d", len(trapArgs))
	}
	arg0, ok := trapArgs[0].([]Field)
	if !ok {
		t.Fatalf("expected []Field, got %T", trapArgs[0])
	}
	if len(arg0) != NUM_COMPLEX_STRUCT_FIELDS {
		t.Errorf("expected %d fields, got %d", NUM_COMPLEX_STRUCT_FIELDS, len(arg0))
	}
	if arg0[0].Name != "Str" {
		t.Errorf("expected 'Str', got '%s'", arg0[0].Name)
	}
	if arg0[0].Value != "Hello" {
		t.Errorf("expected 'Hello', got '%v'", arg0[0].Value)
	}
	if arg0[1].Name != "Int" {
		t.Errorf("expected 'Int', got '%s'", arg0[1].Name)
	}
	if arg0[1].Value != int64(10) {
		t.Errorf("expected 10, got %v", arg0[1].Value)
	}
	if arg0[2].Name != "Bool" {
		t.Errorf("expected 'Bool', got '%s'", arg0[2].Name)
	}
	if arg0[2].Value != true {
		t.Errorf("expected true, got %v", arg0[2].Value)
	}
	if arg0[3].Name != "StrAfterBool" {
		t.Errorf("expected 'StrAfterBool', got '%s'", arg0[3].Name)
	}
	if arg0[3].Value != "World" {
		t.Errorf("expected 'World', got '%v'", arg0[3].Value)
	}
	if arg0[5].Name != "StrAfterTestStruct" {
		t.Errorf("expected 'StrAfterTestStruct', got '%s'", arg0[5].Name)
	}
	if arg0[5].Value != "x1" {
		t.Errorf("expected 'x1', got '%v'", arg0[5].Value)
	}

	if arg0[6].Name != "Int32" {
		t.Errorf("expected 'Int32', got '%s'", arg0[6].Name)
	}
	if arg0[6].Value != int32(123) {
		t.Errorf("expected 123, got %v", arg0[6].Value)
	}
	if arg0[7].Name != "Int16" {
		t.Errorf("expected 'Int16', got '%s'", arg0[7].Name)
	}
	if arg0[7].Value != int16(456) {
		t.Errorf("expected 456, got %v", arg0[7].Value)
	}
	if arg0[8].Name != "IntArray" {
		t.Errorf("expected 'IntArray', got '%s'", arg0[8].Name)
	}
	if !reflect.DeepEqual(arg0[8].Value, []interface{}{int64(1), int64(2), int64(3)}) {
		t.Errorf("expected [1, 2, 3], got %v", arg0[8].Value)
	}
	if arg0[9].Name != "StrSlice" {
		t.Errorf("expected 'StrSlice', got '%s'", arg0[9].Name)
	}
	if !reflect.DeepEqual(arg0[9].Value, []interface{}{"a", "b", "c"}) {
		t.Errorf("expected ['a', 'b', 'c'], got %v", arg0[9].Value)
	}
	if arg0[10].Name != "Map" {
		t.Errorf("expected 'Map', got '%s'", arg0[10].Name)
	}
	if !reflect.DeepEqual(arg0[10].Value, []interface{}{"a", 1, "b", 2, "c", 3}) {
		t.Errorf("expected ['a', 1, 'b', 2, 'c', 3], got %v", arg0[10].Value)
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

func fnMap(m map[string]int) string {
	trap()
	return fmt.Sprintf("Map value: %v", m)
}

func fnPtr(p *int) string {
	trap()
	return fmt.Sprintf("Pointer value: %v", *p)
}

func fnPtrStruct(p *ComplexStruct) {
	trap()

	fmt.Fprint(io.Discard, p)
}
