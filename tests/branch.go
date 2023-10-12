package tests

func LegalBreak() {
	// the outer loop should be allowed, but the inner loop should not
	for i := 0; i < 10; i++ { // allowed
		for j := 0; j < 10; j++ { // not allowed
			if j == 5 {
				break
			}
			println(i + j)
		}
	}
	// the outer loop should not be allowed, but the inner loop should be
	for i := 0; i < 10; i++ { // not allowed
		if i == 5 {
			break
		}
		for j := 0; j < 10; j++ { // allowed
			println(i + j)
		}
	}
	// both loops should be allowed, because it only breaks out of the switch
	for i := 0; i < 10; i++ { // allowed
		for j := 0; j < 10; j++ { // allowed
			switch j {
			case 5:
				break
			default:
				println(i + j)
			}
		}
	}
}

func Goto() {
	// legal goto
	for i := 0; i < 10; i++ { // allowed
		for j := 0; j < 10; j++ { // allowed
			if j == 5 {
				goto end
			}
			println(i + j)
		end:
		}
	}
	// illegal goto for inner loop
	for i := 0; i < 10; i++ { // allowed
		for j := 0; j < 10; j++ { // not allowed
			if j == 5 {
				goto end2
			}
			println(i + j)
		}
	end2:
	}
start:
	// illegal goto for outer loop
	for i := 0; i < 10; i++ { // not allowed
		if i == 5 {
			goto start
		}
		for j := 0; j < 10; j++ { // allowed
			println(i + j)
		}
	}
}

// The integer values are specific int values for this file; changing the file means necessitating changing this
// for this reason, it is placed at the bottom, and any additional code should be added below existing code to minimize changes
var BranchPredictions = map[int]Prediction{
	5:  {5, true},
	6:  {6, false},
	14: {14, false},
	18: {18, true},
	23: {23, true},
	24: {24, true},
	37: {37, true},
	38: {38, true},
	47: {47, true},
	48: {48, false},
	58: {58, false},
	62: {62, true},
}
