package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-delve/delve/pkg/proc"
	"github.com/xhd2015/xgo/example/pkg"
	"github.com/xhd2015/xgo/runtime"
	"github.com/xhd2015/xgo/trace"
)

var bi *proc.BinaryInfo

func Init(binary string) {
	var err error
	bi, err = proc.LoadBinary(binary)
	if err != nil {
		panic(fmt.Errorf("failed to load binary: %w", err))
	}
	runtime.SetupTrap()
}

// run:
//
//	(cd xgo && ../with-go-devel.sh go run ./cmd/xgo -v ./example)
func main() {
	fmt.Printf("before set trap\n")
	pkg.Add(1, 2)
	Init(os.Args[0])

	fmt.Printf("after set trap\n")

	detach := SetupStack()
	defer detach()

	pkg.Add(1, 2)
}

func SetupStack() func() {
	stack := &runtime.Stack{}
	runtime.AttachStack(stack)
	stack.OnEnter = func(entry *runtime.StackEntry, pc uintptr, args runtime.StackArgs) {
		fields := trace.RetrieveArgsViaPC(bi, pc, args)
		var printArgs []string
		for _, field := range fields {
			printArgs = append(printArgs, fmt.Sprintf("%s: %s", field.Name, field.Value))
		}

		goID := runtime.GetG().GoID()
		funcName := shortFuncName(runtime.FuncForPC(pc).Name())

		// indent
		fmt.Print(strings.Repeat(" ", stack.TopDepth))
		fmt.Printf("call [go_%d] %s(%s)\n", goID, funcName, strings.Join(printArgs, ", "))
	}
	stack.OnExit = func(entry *runtime.StackEntry, pc uintptr, results []runtime.Field) {
		goID := runtime.GetG().GoID()
		funcName := shortFuncName(runtime.FuncForPC(pc).Name())
		fmt.Print(strings.Repeat(" ", stack.TopDepth))
		fmt.Printf("return [go_%d] %s\n", goID, funcName)
	}
	return func() {
		runtime.DetachStack()
	}
}

func shortFuncName(name string) string {
	lastIndex := strings.LastIndex(name, "/")
	if lastIndex == -1 {
		return name
	}
	return name[lastIndex+1:]
}
