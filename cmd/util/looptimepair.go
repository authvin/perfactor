package util

// pairing up a for loop with its total Time
type LoopInfo struct {
	Loop Loop
	Time int64
}

type LoopInfoArray []LoopInfo

func (l LoopInfoArray) Len() int {
	return len(l)
}

func (l LoopInfoArray) Less(i, j int) bool {
	// we want to sort greatest first
	return l[i].Time > l[j].Time
}

func (l LoopInfoArray) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l LoopInfoArray) AddLines(loop Loop) {
	// modify the array, adding line numbers that have been inserted by the loop passed as argument
	for _, lt := range l {
		// if this loop is before the loop passed as argument, skip it
		if lt.Loop.EndLine <= loop.Line {
			continue
		} else if lt.Loop.Line <= loop.Line {
			// this loop surrounds the loop being modified - add 7, but only to the endline
			lt.Loop.EndLine += 7
		} else if lt.Loop.Line <= loop.EndLine {
			// we are inside the loop being modified - add 4 to the line numbers
			lt.Loop.Line += 4
			lt.Loop.EndLine += 4
		} else {
			// we are after the loop being modified - add 7 to the line numbers
			lt.Loop.Line += 7
			lt.Loop.EndLine += 7
		}
	}
}
