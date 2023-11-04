package tests

// this file contains loops which use method call in legal or illegal ways
// relevant conditions: rule 013

// basic struct to test method calls
type g_method struct {
	a int
}

// basic method to test method calls
func (g *g_method) set_to_5() {
	g.a = 5
}

// Method call is allowed if the struct is declared inside the loop
func LegalMethodCall() {
	for i := 0; i < 10; i++ { // Allowed
		g := g_method{a: 10}
		g.set_to_5()
	}
}

// Method call is not allowed if the struct is declared outside the loop
func Rule013() {
	// rule 013: cannot call a method on a struct declared outside the loop
	g := g_method{a: 10}
	for i := 0; i < 10; i++ { // Not allowed
		g.set_to_5()
	}
}

var MethodcallPredictions = map[int]Prediction{
	18: {18, true},
	28: {28, false},
}
