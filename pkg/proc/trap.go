package proc

import "fmt"

// TrapHandler defines the interface for trap breakpoint handlers.
type TrapHandler interface {
	// HandleTrap is called when a trap breakpoint is hit.
	// It receives the current thread, target, and target group to extract necessary information.
	HandleTrap(thread Thread, target *Target) error
}

// DefaultTrapHandler is the default implementation of TrapHandler.
type DefaultTrapHandler struct {
	useSet bool // if true, use SetVariable, otherwise use low-level implementation
}

// NewDefaultTrapHandler creates a new DefaultTrapHandler.
func NewDefaultTrapHandler() *DefaultTrapHandler {
	return &DefaultTrapHandler{
		useSet: false, // default to low-level implementation
	}
}

// HandleTrap implements the TrapHandler interface.
func (h *DefaultTrapHandler) HandleTrap(thread Thread, target *Target) error {
	// Get stack trace with 2 frames (current frame + caller)
	frames, err := ThreadStacktrace(target, thread, 2)
	if err != nil {
		return fmt.Errorf("could not get stacktrace: %v", err)
	}

	if len(frames) < 2 {
		return fmt.Errorf("could not get caller frame")
	}

	// First frame is trap(), second frame is the caller
	trapFrame := frames[0]
	callerFrame := frames[1]

	// Print caller information
	fmt.Printf("Trap called from %s at %s:%d\n",
		callerFrame.Call.Fn.Name,
		callerFrame.Call.File,
		callerFrame.Call.Line)

	// Get goroutine info
	g, _ := GetG(thread)

	// Print caller's arguments
	callerScope := FrameToScope(target, thread.ProcessMemory(), g, thread.ThreadID(), callerFrame)
	callerVars, err := callerScope.FunctionArguments(LoadConfig{
		FollowPointers:     true,
		MaxVariableRecurse: 1,
		MaxStringLen:       64,
		MaxArrayValues:     64,
		MaxStructFields:    -1,
	})

	if err != nil {
		fmt.Printf("Could not get caller arguments: %v\n", err)
	} else if len(callerVars) > 0 {
		fmt.Println("Caller Arguments:")
		for _, v := range callerVars {
			if v.Value != nil {
				fmt.Printf("  %s = %v\n", v.Name, v.Value)
			} else {
				fmt.Printf("  %s = <unreadable>\n", v.Name)
			}
		}
	} else {
		fmt.Println("No caller arguments")
	}

	// Create scope for trap function
	scope := FrameToScope(target, thread.ProcessMemory(), g, thread.ThreadID(), trapFrame)

	// First print current args
	vars, err := scope.FunctionArguments(LoadConfig{
		FollowPointers:     true,
		MaxVariableRecurse: 1,
		MaxStringLen:       64,
		MaxArrayValues:     64,
		MaxStructFields:    -1,
	})

	if err != nil {
		return fmt.Errorf("could not get arguments: %v", err)
	}

	if len(vars) > 0 {
		fmt.Println("Original Arguments:")
		for _, v := range vars {
			if v.Value != nil {
				fmt.Printf("  %s = %v\n", v.Name, v.Value)
			} else {
				fmt.Printf("  %s = <unreadable>\n", v.Name)
			}
		}
	} else {
		fmt.Println("No arguments")
	}

	// Modify args using either SetVariable or low-level implementation
	if h.useSet {
		err = scope.SetVariable("args", "[]interface{}{true}")
		if err != nil {
			return fmt.Errorf("could not modify args: %v", err)
		}
	} else {
		err = h.UpdateTrapArgs(thread, target)
		if err != nil {
			return fmt.Errorf("could not modify args: %v", err)
		}
	}

	// Print modified args
	vars, err = scope.FunctionArguments(LoadConfig{
		FollowPointers:     true,
		MaxVariableRecurse: 1,
		MaxStringLen:       64,
		MaxArrayValues:     64,
		MaxStructFields:    -1,
	})

	if err != nil {
		return fmt.Errorf("could not get modified arguments: %v", err)
	}

	if len(vars) > 0 {
		fmt.Println("Modified Arguments:")
		for _, v := range vars {
			if v.Value != nil {
				fmt.Printf("  %s = %v\n", v.Name, v.Value)
			} else {
				fmt.Printf("  %s = <unreadable>\n", v.Name)
			}
		}
	}

	return nil
}
