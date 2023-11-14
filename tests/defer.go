package tests

func LegalDefer() {
	for i := 0; i < 10; i++ { // Allowed
		go func() {
			defer println("deferred")
		}()
	}
}

func Rule015() {
	for i := 0; i < 10; i++ { // Not allowed
		defer println("deferred")
	}
}

var DeferPredictions = map[int]Prediction{
	4:  {4, true},
	12: {12, false},
}
