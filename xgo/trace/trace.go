package trace

import (
	"encoding/json"
	"fmt"

	"github.com/go-delve/delve/pkg/proc"
	"github.com/xhd2015/xgo/runtime"
)

func RetrieveArgsViaPC(bi *proc.BinaryInfo, pc uintptr, stackArgs runtime.StackArgs) []runtime.Field {
	params, err := proc.GetFuncParams(bi, pc)
	if err != nil {
		panic(fmt.Errorf("failed to get func params: %w", err))
	}

	convArgs, err := proc.RetrieveStackArgs(bi, pc, stackArgs)
	if err != nil {
		panic(fmt.Errorf("failed to retrieve stack args: %w", err))
	}

	fields := make([]runtime.Field, len(convArgs))
	for i, arg := range convArgs {
		jsonArgs, err := json.Marshal(arg)
		if err != nil {
			panic(fmt.Errorf("failed to marshal args: %w", err))
		}
		fields[i] = runtime.Field{
			Name:  params[i].Name,
			Value: string(jsonArgs),
		}
	}
	return fields
}
