package tests

// this file contains loops which use break and goto in legal or illegal ways
// relevant conditions: rule 004, 005

func LegalBreak() {
	// both loops should be allowed, because it only breaks out of the switch
	for i := 0; i < 10; i++ { // Allowed
		for j := 0; j < 10; j++ { // Allowed
			switch j {
			case 5:
				break
			default:
				println(i + j)
			}
		}
	}
}

func LegalGoto() {
	// legal goto
	for i := 0; i < 10; i++ { // Allowed
		for j := 0; j < 10; j++ { // Allowed
			if j == 5 {
				goto end
			}
			println(i + j)
		end:
		}
	}
}

func Rule004() {
	// rule 004: cannot goto to a label from outside the loop
	// illegal goto for inner loop
	for i := 0; i < 10; i++ { // Allowed
		for j := 0; j < 10; j++ { // Not allowed
			if j == 5 {
				goto end2
			}
			println(i + j)
		}
	end2:
	}
start:
	// illegal goto for outer loop
	for i := 0; i < 10; i++ { // Not allowed
		if i == 5 {
			goto start
		}
		for j := 0; j < 10; j++ { // Allowed
			println(i + j)
		}
	}
}

func Rule005() {
	// rule 005: cannot break out to a label from outside the loop
	// the outer loop should be allowed, but the inner loop should not
	for i := 0; i < 10; i++ { // Allowed
		for j := 0; j < 10; j++ { // Not allowed
			if j == 5 {
				break
			}
			println(i + j)
		}
	}
	// the outer loop should not be allowed, but the inner loop should be
	for i := 0; i < 10; i++ { // Not allowed
		if i == 5 {
			break
		}
		for j := 0; j < 10; j++ { // Allowed
			println(i + j)
		}
	}
}

// The integer values are specific int values for this file; changing the file means necessitating changing this
// for this reason, it is placed at the bottom, and any additional code should be added below existing code to minimize changes

var BranchPredictions = map[int]Prediction{
	9:  {9, true},
	23: {23, true},
	51: {51, true},
	60: {60, true},
	61: {61, false},
	69: {69, false},
	8:  {8, true},
	22: {22, true},
	36: {36, true},
	37: {37, false},
	47: {47, false},
	73: {73, true},
}
