package terminal

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-delve/delve/service/api"
)

// this file is extended by user
// TODO: complete this file

// doc:
//  search for trace breakpoints set
//     addrs, err := client.FunctionReturnLocations

var isTraceCmd bool = true
var outputJSON bool = true
var jsonInfoWriter io.Writer
var jsonTraceWriter io.Writer

var recordStartingPoint string = "main.main"

func init() {
	if isTraceCmd && outputJSON {
		file, err := os.OpenFile("trace-log.txt", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			fmt.Printf("failed to open trace.txt: %s\n", err)
			os.Exit(1)
		}
		jsonInfoWriter = file

		traceJSONFile, err := os.OpenFile("trace.json", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			fmt.Printf("failed to open trace.json: %s\n", err)
			os.Exit(1)
		}
		jsonTraceWriter = traceJSONFile
	}
}

var goroutineStacks sync.Map

// assume same goroutine

type Stack struct {
	GoroutineID int64
	Begin       time.Time
	End         Nanoseconds
	MaxEntryID  int64
	Roots       []*StackEntry
	Chain       []*StackEntry
	Data        map[interface{}]interface{}
}

type StackEntry struct {
	ID       int64
	FuncName string
	Args     string
	File     string
	Line     int
	Results  []string
	Children []*StackEntry
	Begin    Nanoseconds
	End      Nanoseconds
	Data     map[interface{}]interface{}
}

func (s *Stack) NextID() int64 {
	s.MaxEntryID++
	return s.MaxEntryID
}

func (s *Stack) Push(entry *StackEntry) {
	n := len(s.Chain)
	if n == 0 {
		s.Roots = append(s.Roots, entry)
	} else {
		s.Chain[n-1].Children = append(s.Chain[n-1].Children, entry)
	}
	s.Chain = append(s.Chain, entry)
}

func (s *Stack) Pop() *StackEntry {
	n := len(s.Chain)
	if n == 0 {
		panic("pop on empty stack")
	}
	top := s.Chain[n-1]
	s.Chain = s.Chain[:n-1]
	return top
}

func (s *Stack) Depth() int {
	return len(s.Chain)
}

func getGoroutineStack(goid int64) *Stack {
	stack, ok := goroutineStacks.Load(goid)
	if !ok {
		st := &Stack{
			GoroutineID: goid,
		}
		goroutineStacks.Store(goid, st)
		return st
	}
	return stack.(*Stack)
}

func (c *Stack) SetData(key, value interface{}) {
	if c.Data == nil {
		c.Data = make(map[interface{}]interface{}, 1)
	}
	c.Data[key] = value
}

func (c *Stack) GetData(key interface{}) interface{} {
	return c.Data[key]
}

func (c *StackEntry) SetData(key, value interface{}) {
	if c.Data == nil {
		c.Data = make(map[interface{}]interface{}, 1)
	}
	c.Data[key] = value
}

func (c *StackEntry) GetData(key interface{}) interface{} {
	return c.Data[key]
}

type traceStartedKeyType struct{}

var traceStartedKey traceStartedKeyType

func printTracepointJSON(t *Term, th *api.Thread, fn *api.Function, args string) {
	goID := th.GoroutineID
	stack := getGoroutineStack(goID)

	if stack.GetData(traceStartedKey) == nil {
		funcName := fn.Name()
		if funcName != recordStartingPoint {
			return
		}
		stack.SetData(traceStartedKey, true)
		stack.Begin = time.Now()
	}

	bp := th.Breakpoint

	if bp.Tracepoint {
		startNano := time.Now().UnixNano() - stack.Begin.UnixNano()
		prefix := strings.Repeat(" ", stack.Depth())
		cur := &StackEntry{
			ID:       stack.NextID(),
			FuncName: fn.Name(),
			File:     th.File,
			Line:     th.Line,
			Args:     args,
			Begin:    Nanoseconds(startNano),
		}
		stack.Push(cur)

		fmt.Fprint(jsonInfoWriter, prefix)
		fmt.Fprintf(jsonInfoWriter, "call: %s(%s)\n", fn.Name(), args)
	}
	if bp.TraceReturn {
		cur := stack.Pop()

		results := make([]string, 0, len(th.ReturnValues))
		for _, v := range th.ReturnValues {
			results = append(results, v.SinglelineString())
		}
		cur.Results = results
		endNano := time.Now().UnixNano() - stack.Begin.UnixNano()
		cur.End = Nanoseconds(endNano)

		var suffix string
		if len(results) > 0 {
			suffix = fmt.Sprintf(" => (%s)", strings.Join(results, ","))
		}

		prefix := strings.Repeat(" ", stack.Depth())
		fmt.Fprint(jsonInfoWriter, prefix)
		fmt.Fprintf(jsonInfoWriter, "return: %s%s\n", cur.FuncName, suffix)

		if stack.Depth() == 0 {
			fmt.Fprintf(jsonInfoWriter, "root returned\n")
			// export := stack.Export()
			export := ConvertStack(stack)
			err := json.NewEncoder(jsonTraceWriter).Encode(export)
			if err != nil {
				fmt.Fprintf(jsonInfoWriter, "failed to encode trace: %s\n", err)
			}
		}
	}
}
