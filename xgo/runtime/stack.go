package runtime

type Stack struct {
	MaxID int

	Root     *StackEntry
	Top      *StackEntry
	TopDepth int

	OnEnter func(entry *StackEntry, pc uintptr, args StackArgs)
	OnExit  func(entry *StackEntry, pc uintptr, results []Field)
}

type StackEntryData map[interface{}]interface{}

type StackEntry struct {
	ID       int
	ParentID int
	Children []*StackEntry
	Data     StackEntryData
}

type stackKeyType struct{}

var stackKey = stackKeyType{}

// AttachStack attaches a stack for recording
func AttachStack(stack *Stack) {
	if stack == nil {
		panic("requires stack")
	}
	curg := GetG()
	st := curg.Get(stackKey)
	if st != nil {
		panic("stack already attached")
	}

	curg.Set(stackKey, stack)
}

func GetStack() *Stack {
	stack := GetG().Get(stackKey)
	if stack == nil {
		return nil
	}
	return stack.(*Stack)
}

func DetachStack() {
	GetG().Delete(stackKey)
}

type Field struct {
	Name  string
	Value interface{}
}

func (s *StackEntry) SetData(key, value interface{}) {
	if s.Data == nil {
		s.Data = make(StackEntryData)
	}
	s.Data[key] = value
}

func (s *StackEntry) GetData(key interface{}) interface{} {
	return s.Data[key]
}

type StackArgs []interface{}
