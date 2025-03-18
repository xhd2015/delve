package runtime

import (
	"runtime"
)

func SetupTrap() {
	runtime.XgoSetTrap(trap)
}

func trap() func() {
	// skip 2: <user func> -> runtime.XgoTrap -> trap
	const SKIP = 2
	stackArgs, pc := runtime.XgoGetCallerArgs(SKIP)

	var cur *StackEntry
	var oldTop *StackEntry
	stack := GetStack()
	if stack != nil {
		stack.MaxID++
		cur = &StackEntry{
			ID: stack.MaxID,
		}
		oldTop = stack.Top
		if oldTop == nil {
			stack.Root = cur
		} else {
			cur.ParentID = oldTop.ID
			oldTop.Children = append(oldTop.Children, cur)
		}
		stack.Top = cur
		if stack.OnEnter != nil {
			stack.OnEnter(cur, pc, stackArgs)
		}
		stack.TopDepth++
	}

	return func() {
		if stack != nil {
			stack.Top = oldTop
			stack.TopDepth--
			if stack.OnExit != nil {
				// TODO: result
				stack.OnExit(cur, pc, nil)
			}
		}
	}
}
