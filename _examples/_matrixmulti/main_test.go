package main

import "testing"

func BenchmarkRunProgram(b *testing.B) {
	for i := 0; i < b.N; i++ {
		main()
	}
}
