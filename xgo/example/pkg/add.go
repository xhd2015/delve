package pkg

import (
	"runtime"
)

func Add(a int, b int) int {
	defer runtime.XgoTrap()()
	check(a, "check a")
	check(b, "check b")
	return a + b
}

func check(a int, msg string) {
	defer runtime.XgoTrap()()
	if a < 0 {
		panic(msg)
	}
}
