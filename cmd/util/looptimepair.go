package util

import "go/ast"

type LoopTimePair struct {
	Loop *ast.ForStmt
	Time int64
}

type LoopTimeArray []LoopTimePair

func (l LoopTimeArray) Len() int {
	return len(l)
}

func (l LoopTimeArray) Less(i, j int) bool {
	// we want to sort greatest first
	return l[i].Time > l[j].Time
}

func (l LoopTimeArray) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}
