package main

import "fmt"

// simple fibonacci program

func fib() {
	for i := 0; i < 10; i++ {
		fmt.Println(fibonacci(i))
	}
}

func fibonacci(i int) int {
	if i == 0 {
		return 0
	}
	if i == 1 {
		return 1
	}
	return fibonacci(i-1) + fibonacci(i-2)
}
