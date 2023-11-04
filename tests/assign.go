package tests

// this file contains loops which use assignment in legal or illegal ways
// relevant conditions: rule 011, 012

func LegalAssignment() {
	// legal assignment
	for i := 0; i < 10; i++ { // Allowed
		for j := 0; j < 10; j++ { // Allowed
			k := j
			println(k + j)
		}
	}
}

func Rule011() {
	// rule 011: cannot modify the loop variable
	var j = 100
	// modifies a variable declared outside the loop
	for i := 0; i < j; i++ { // Not allowed
		j--
	}
	// assigns to a variable declared outside the loop
	for i := 0; i < j; i++ { // Not allowed
		j = 100
	}
	// assigns to a variable declared outside the loop
	for i := 0; i < 10; i++ { // Not allowed
		for j := 0; j < 10; j++ { // Not allowed
			i = j
			println(i + j)
		}
	}
}

func Rule012() {
	// rule 012: writing to an unsupported expression, such as a selector expression
	type g struct {
		a int
	}
	for i := 0; i < 10; i++ { // Not allowed
		g := g{a: 10}
		g.a = 5
	}
}

var AssignPredictions = map[int]Prediction{
	28: {28, false},
	29: {29, false},
	41: {41, false},
	8:  {8, true},
	9:  {9, true},
	20: {20, false},
	24: {24, false},
}
