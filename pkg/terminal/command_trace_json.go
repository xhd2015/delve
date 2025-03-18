package terminal

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-delve/delve/service/api"
)

// per-goroutine stack: goid(int64) -> *Stack
var goroutineStacks sync.Map

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

func clearStack(goid int64) {
	goroutineStacks.Delete(goid)
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

type traceWithConfigFileKeyType struct{}

var traceWithConfigFileKey traceWithConfigFileKeyType

var traceDebugWriter io.Writer = io.Discard

// var traceDebugWriter io.Writer = os.Stderr

func traceWithJSON(t *Term, th *api.Thread, fn *api.Function, traceWith string, args string) {
	goID := th.GoroutineID
	stack := getGoroutineStack(goID)

	if stack.GetData(traceStartedKey) == nil {
		funcName := fn.Name()
		if funcName != traceWith {
			return
		}

		var conf *api.Variable
		for _, arg := range th.BreakpointInfo.Arguments {
			if arg.Name == "config" {
				conf = &arg
				break
			}
		}
		if conf == nil || conf.Kind != reflect.Struct {
			return
		}
		var outputFile string
		for _, child := range conf.Children {
			switch child.Name {
			case "OutputFile":
				outputFile = child.Value
			}
		}
		if outputFile == "" {
			return
		}

		// read first arg's config
		stack.SetData(traceStartedKey, true)
		stack.SetData(traceWithConfigFileKey, outputFile)
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

		fmt.Fprint(traceDebugWriter, prefix)
		fmt.Fprintf(traceDebugWriter, "call: %s(%s)\n", fn.Name(), args)
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
		fmt.Fprint(traceDebugWriter, prefix)
		fmt.Fprintf(traceDebugWriter, "return: %s%s\n", cur.FuncName, suffix)

		if stack.Depth() == 0 {
			fmt.Fprintf(traceDebugWriter, "root returned\n")

			outputFileData := stack.GetData(traceWithConfigFileKey)
			if outputFileData == nil {
				fmt.Fprintf(traceDebugWriter, "no output file\n")
				return
			}
			outputFile, _ := outputFileData.(string)
			if outputFile == "" {
				fmt.Fprintf(traceDebugWriter, "output file is empty\n")
				return
			}

			// append to file
			file, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(traceDebugWriter, "failed to open output file: %s\n", err)
				return
			}
			defer file.Close()

			// export := stack.Export()
			export := ConvertStack(stack)
			err = json.NewEncoder(file).Encode(export)
			if err != nil {
				fmt.Fprintf(traceDebugWriter, "failed to encode trace: %s\n", err)
			}
			clearStack(goID)
		}
	}
}
