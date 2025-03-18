package terminal

import "time"

// a standard datetime string: YYYY-MM-DDTHH:MM:SSZ
// RFC3339
type UTCDateTime string

type Nanoseconds int64

type StackExport struct {
	GoroutineID int64               `json:"goroutine_id"`
	Begin       UTCDateTime         `json:"start"`
	EndNs       Nanoseconds         `json:"end_ns"`
	Roots       []*StackEntryExport `json:"roots"`
}

type StackEntryExport struct {
	ID       int64               `json:"id"`
	FuncName string              `json:"func_name"`
	File     string              `json:"file"`
	Line     int                 `json:"line"`
	BeginNs  Nanoseconds         `json:"start_ns"`
	EndNs    Nanoseconds         `json:"end_ns"`
	Args     string              `json:"args"`
	Results  []string            `json:"results"`
	Children []*StackEntryExport `json:"children"`
}

func (c *Stack) Export() *StackExport {
	return &StackExport{
		GoroutineID: c.GoroutineID,
		Begin:       UTCDateTime(c.Begin.Format(time.RFC3339)),
		EndNs:       c.End,
		Roots:       exportStackEntries(c.Roots),
	}
}

func exportStackEntries(list []*StackEntry) []*StackEntryExport {
	if list == nil {
		return nil
	}
	exportList := make([]*StackEntryExport, len(list))
	for i, c := range list {
		exportList[i] = c.Export()
	}
	return exportList
}

func (c *StackEntry) Export() *StackEntryExport {
	return &StackEntryExport{
		ID:       c.ID,
		FuncName: c.FuncName,
		File:     c.File,
		Line:     c.Line,
		BeginNs:  c.Begin,
		EndNs:    c.End,
		Args:     c.Args,
		Results:  c.Results,
		Children: exportStackEntries(c.Children),
	}
}
