package main

import (
	"testing"
	"flag"
)
// We pass a flag to indicate which method to benchmark
// This is done because benchcmp will compare the same benchmark method - here BenchmarkEither()
var flagvar = flag.Int("mode", 0, "mode 0 is origin, mode 1 is target")
var src = loadImage("testdata/test_rail.jpg")

// Runs a benchmark on the original method in main
func BenchmarkOrigin(b *testing.B) {
	for i := 0; i < b.N; i++ {
		runOrigin(src);
	}
}

// Runs a benchmark on the new method in main
func BenchmarkTarget(b *testing.B) {
	for i := 0; i < b.N; i++ {
		runTarget(src);
	}
}

// Method used to run benchmarks when comparing the methods in main
func BenchmarkEither(b *testing.B) {
	flag.Parse()
	switch *flagvar {
	case 0:
		for i := 0; i < b.N; i++ {
			runOrigin(src);
		}
	case 1:
		for i := 0; i < b.N; i++ {
			runTarget(src);
		}
	}	
}