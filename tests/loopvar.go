package tests

// this file contains methods with loops which use loop variables in legal or illegal ways
// relevant conditions: rule 001, 010
func Functional(size int) {
	// this method passes because it has all three;
	//	a declared variable, using this in the condition, and incrementing it
	result := make(chan int, size)
	for i := 0; i < size; i++ { // Allowed
		result <- i
	}
	// the i is being shadowed, in which case it is allowed to assign to it
	for i := 0; i < 10; i++ { // Allowed
		go func() {
			i := 5
			i++
		}()
	}
}

func Rule001() {
	// rule 001: must declare, use, and increment the loop variable
	// this method fails because it doesn't increment the variable, and is an infinite loop
	result := make(chan int, 10)
	for i, j := 0, 0; i < 10; j++ { // Not allowed
		result <- i
	}
	// this method fails because it doesn't use the variable in the condition
	result = make(chan int, 10)
	for i := 0; true; i++ { // Not allowed
		result <- i
	}
	// this method fails because it doesn't declare the variable
	result = make(chan int, 10)
	var i = 0
	for i < 10 { // Not allowed
		i++
		result <- i
	}
}

func Rule010() {
	// rule 010: cannot assign to the loop variable
	for i := 0; i < 10; i++ { // Not allowed
		i = 5
	}
	for i := 0; i < 10; i++ { // Not allowed
		i++
	}
}

var LoopvarPredictions = map[int]Prediction{
	9:  {9, true},
	13: {13, true},
	25: {25, false},
	30: {30, false},
	36: {36, false},
	44: {44, false},
	47: {47, false},
}
