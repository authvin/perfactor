package tests

// this file contains loops which use return in legal or illegal ways
// relevant conditions: rule 003

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
	// rule 003: cannot return from inside the loop
	for i := 0; i < 10; i++ { // Not allowed
		if i == 5 {
			return
		}
	}
}

var ReturnPredictions = map[int]Prediction{
	8:  {8, true},
	19: {19, false},
}
