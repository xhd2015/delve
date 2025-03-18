package main

import (
	"fmt"
)

// usage:
//
//	dlv trace 'main\..*' -e __debug_bin_trace_with --trace-with main.startTrace
func main() {
	res := add(1, 2)
	fmt.Printf("add(1, 2) = %d\n", res)

	startTrace(Config{
		OutputFile: "trace.json",
	}, func() {
		res := add(3, 4)
		fmt.Printf("add(3, 4) = %d\n", res)
	})

	startTrace(Config{
		OutputFile: "trace.json",
	}, func() {
		res := add(4, 5)
		fmt.Printf("add(4, 5) = %d\n", res)
	})
}

func add(a, b int) int {
	check(a, "a is less than 0")
	check(b, "b is less than 0")
	return a + b
}

func check(a int, msg string) {
	if a < 0 {
		panic(msg)
	}
}

type Config struct {
	OutputFile string
}

func startTrace(config Config, fn func()) {
	fmt.Printf("start trace: %s\n", config.OutputFile)
	fn()
}
