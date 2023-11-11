package tests

// this file contains methods with loops which use arrays in legal or illegal ways
// relevant conditions: rule 002, 006, 007, 008, 009

func ArrayLegal() {
	// this method should pass because it uses an array in a legal way
	var arr [10]int
	for i := 0; i < 10; i++ { // Allowed
		arr[i] = i
	}
}

func Rule002() {
	// rule 002: reads to and writes from the same array
	var arr [10]int
	for i := 0; i < 10; i++ { // Not allowed
		arr[i] = arr[i] + 1
	}
}

func Rule006() {
	// rule 006: base of index expression is not an identifier
	var arr [10]int
	for i := 0; i < 10; i++ { // Not allowed
		arr[1] = i
	}

	// This also breaks this rule, but would technically be okay. We're just being overprotective here
	for i := 0; i < 10; i++ { // Not allowed
		arr[i+0] = i
	}
}

func Rule007() {
	// rule 007: could not determine type of indexed variable
	// For example, if something happened to the info struct, this would trigger
	// Not expected to be triggered normally
}

func Rule008() {
	// rule 008: only supports arrays and slices
	var m = make(map[int]int)
	for i := 0; i < 10; i++ { // Not allowed
		m[i] = i
	}
}

func Rule009() {
	// rule 009: writes to array using non-loop variable as the index
	var arr [10]int
	for i := 0; i < 10; i++ { // Not allowed
		j := i
		arr[j] = i
	}
}

var ArrayPredictions = map[int]Prediction{
	9:  {9, true},
	17: {17, false},
	25: {25, false},
	30: {30, false},
	44: {44, false},
	52: {52, false},
}
