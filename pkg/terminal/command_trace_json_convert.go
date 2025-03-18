package terminal

import (
	"time"

	"github.com/go-delve/delve/test/xgo_trace"
)

func ConvertStack(stack *Stack) *xgo_trace.Stack {
	return &xgo_trace.Stack{
		Format:   "stack",
		Begin:    stack.Begin.Format(time.RFC3339),
		Children: ConvertStackEntries(stack.Roots),
	}
}

func ConvertStackEntries(entries []*StackEntry) []*xgo_trace.StackEntry {
	if entries == nil {
		return nil
	}
	list := make([]*xgo_trace.StackEntry, len(entries))
	for i, entry := range entries {
		list[i] = ConvertStackEntry(entry)
	}
	return list
}

func ConvertStackEntry(entry *StackEntry) *xgo_trace.StackEntry {
	if entry == nil {
		return nil
	}
	return &xgo_trace.StackEntry{
		FuncInfo: ConvertFuncInfo(entry),
		BeginNs:  int64(entry.Begin),
		EndNs:    int64(entry.End),
		Args:     entry.Args,
		Results:  entry.Results,
		Children: ConvertStackEntries(entry.Children),
	}
}

func ConvertFuncInfo(entry *StackEntry) *xgo_trace.FuncInfo {
	if entry == nil {
		return nil
	}
	return &xgo_trace.FuncInfo{
		Name: entry.FuncName,
		File: entry.File,
		Line: entry.Line,
	}
}
