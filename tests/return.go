package tests

func LegalReturn() {
	// a return is only legal if it is inside a function declaration
	for i := 0; i < 10; i++ { // Allowed
		if i == 5 {
			go func() {
				return
			}()
		}
	}
}

func Rule003() {
	for i := 0; i < 10; i++ { // Not allowed
		if i == 5 {
			return
		}
	}
}

var ReturnPredictions = map[int]Prediction{
	5:  {5, true},
	15: {15, false},
}
