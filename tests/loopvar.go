package tests

func Functional(size int) {
	// this method passes because it has all three;
	//	a declared variable, using this in the condition, and incrementing it
	result := make(chan int, size)
	for i := 0; i < size; i++ { // Allowed
		result <- i
	}
}

func NotIncrementing() {
	// this method fails because it doesn't increment the variable, and is an infinite loop
	result := make(chan int, 10)
	for i, j := 0, 0; i < 10; j++ { // Not allowed
		result <- i
	}
}

func NotUsingVariable() {
	// this method fails because it doesn't use the variable in the condition
	result := make(chan int, 10)
	for i := 0; true; i++ { // Not allowed
		result <- i
	}
}

var LoopvarPredictions = map[int]Prediction{
	7:  {7, true},
	15:  {15, false},
	23:  {23, false},
}
