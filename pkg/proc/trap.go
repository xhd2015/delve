package proc

import "fmt"

// TrapHandler defines the interface for trap breakpoint handlers.
type TrapHandler interface {
	// HandleTrap is called when a trap breakpoint is hit.
	// It receives the current thread and target to extract necessary information.
	HandleTrap(thread Thread, target *Target) error
}

// DefaultTrapHandler is the default implementation of TrapHandler.
type DefaultTrapHandler struct{}

// NewDefaultTrapHandler creates a new DefaultTrapHandler.
func NewDefaultTrapHandler() *DefaultTrapHandler {
	return &DefaultTrapHandler{}
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
	callerFrame := frames[1]

	// Print caller information
	fmt.Printf("Trap called from %s at %s:%d\n",
		callerFrame.Call.Fn.Name,
		callerFrame.Call.File,
		callerFrame.Call.Line)

	g, _ := GetG(thread)
	scope := FrameToScope(target, thread.ProcessMemory(), g, thread.ThreadID(), callerFrame)

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
		fmt.Println("Arguments:")
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

	return nil
}
