package main

import "fmt"

func main() {
	res := add(1, 2)
	fmt.Printf("add(1, 2) = %d\n", res)
}

func add(a, b int) int {
	trap()
	check(a, "a is less than 0")
	check(b, "b is less than 0")
	return a + b
}

func check(a int, msg string) {
	trap()
	if a < 0 {
		panic(msg)
	}
}

func trap() {
	// print()
	// return
}
