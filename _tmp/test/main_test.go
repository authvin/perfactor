package main

import "testing"

const length = 6000000
const max = 5000000
const seed int64 = 30
const seedsToCheck int64 = 10

func BenchmarkRunProgram(b *testing.B) {
	for i := 0; i < b.N; i++ {
		runProgram(length, max, seed, seedsToCheck)
	}
}

func BenchmarkGenerateArray(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateArray(length, max, seed)
	}
}

func BenchmarkFindLargestStreak(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		var arr = generateArray(length, max, seed)
		b.StartTimer()
		findLargestStreak(arr)
	}
}

func BenchmarkSortArray(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		var arr = generateArray(length, max, seed)
		b.StartTimer()
		sortArray(arr)
	}
}
