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
}

func Rule001() {
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

}

var LoopvarPredictions = map[int]Prediction{
	22: {22, false},
	28: {28, false},
	9:  {9, true},
	17: {17, false},
}
