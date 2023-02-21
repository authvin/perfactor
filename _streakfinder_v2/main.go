package main

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"sync"
)

func main() {
	var args = os.Args
	var length = 600
	var max = 500
	var seed int64 = 32
	var seedsToCheck int64 = 1
	var err error
	if len(args) > 1 {
		length, err = strconv.Atoi(args[1])
	}
	if err != nil {
		fmt.Println(err.Error())
	}
	if len(args) > 2 {
		max, err = strconv.Atoi(args[2])
	}
	if err != nil {
		fmt.Println(err.Error())
	}
	if len(args) > 3 {
		seed, err = strconv.ParseInt(args[3], 10, 64)
	}
	if err != nil {
		fmt.Println(err.Error())
	}
	if len(args) > 4 {
		seedsToCheck, err = strconv.ParseInt(args[4], 10, 64)
	}
	if err != nil {
		fmt.Println(err.Error())
	}
	var result = runProgram(length, max, seed, seedsToCheck)

	for i := int64(0); i < seedsToCheck; i++ {
		fmt.Printf("%d: %d - %v\n", seed+i, len(result[i]), result[i])
	}
}

func runProgram(length int, max int, seed int64, seedsToCheck int64) [][]int {
	var streaks = make([][]int, seedsToCheck)
	var arrs = make([][]int, seedsToCheck)
	for i := int64(0); i < seedsToCheck; i++ {
		arrs[i] = generateArray(length, max, seed+i)
	}
	var wg sync.WaitGroup
	wg.Add(int(seedsToCheck))
	for i := int64(0); i < seedsToCheck; i++ {
		go func(n int64) {
			defer wg.Done()
			sortArray(arrs[n])
		}(i)
	}
	wg.Wait()
	for i := int64(0); i < seedsToCheck; i++ {
		streaks[i] = findLargestStreak(arrs[i])
	}
	return streaks
}

func generateArray(length int, max int, seed int64) []int {
	rand.Seed(seed)
	slice := make([]int, length)
	for i := 1; i < length; i++ {
		slice[i] = rand.Intn(max)
	}
	return slice
}

func sortArray(arr []int) {
	sort.Ints(arr)
}

func findLargestStreak(arr []int) []int {
	var streak = make([]int, 0)
	var best = make([]int, 0)
	var prev = arr[0]
	for i := 1; i < len(arr); i++ {
		if arr[i] == prev+1 {
			streak = append(streak, arr[i])
		} else {
			if len(streak) > len(best) {
				best = streak
			}
			streak = make([]int, 0)
		}
		prev = arr[i]
	}
	return best
}
