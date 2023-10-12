package util

import (
	"errors"
	"fmt"
	"github.com/owenrumney/go-sarif/sarif"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"reflect"
)

type Loop struct {
	For     *ast.ForStmt
	Range   *ast.RangeStmt
	Body    *ast.BlockStmt
	Pos     token.Pos
	End     token.Pos
	Line    int
	EndLine int
}

// FindSafeLoopsForRefactoring finds loops that can be refactored to be concurrent
// It returns a list of Loop positions pointing to for and range loops
func FindSafeLoopsForRefactoring(forLoops []Loop, f *token.FileSet, run *sarif.Run, fpath string, acceptMap map[string]int, info *types.Info, out io.Writer) []Loop {
	// The first predicate is that the Loop does not assign any values used within the Loop.
	// The Loop should be able to write to a variable it doesn't use - right? If the writing doesn't mind the context... though maybe it wants the last index it goes through?
	// - but that's pretty poor design. Should be enough to acknowledge that this is a weakness, and that a better tool would take this into account
	// Can we check if any of our variables are assigned to in a goroutine? because we'd want to avoid any of those. But then, is it different?
	// So long as it's not a side effect of the Loop itself, the program might change, but it can still be done safely. This might
	// be one of those "we can do this, but it changes behaviour" refactorings.

	if acceptMap == nil {
		acceptMap = make(map[string]int)
	}
	if out == nil {
		println("out is nil")
	}
	// Problem: what about assigning to an array or map, where we're assigning to an index corresponding to the main Loop variable?
	// Solution: Check if the assign is to an index of an array or map, and if so, check if the index is the Loop variable

	// list of loops that can be made concurrent
	var concurrentLoops []Loop

	// Now that we have the information, we can filter out the loops that can be made concurrent
	for _, loop := range forLoops {
		// The check we're doing is if the Loop does not write to a variable outside the Loop
		// Thus, if that doesn't trigger, we assume it's safe to refactor
		// add to list of loops that can be made concurrent
		if LoopCanBeConcurrent(loop, f, run, nil, acceptMap, info, out) {
			concurrentLoops = append(concurrentLoops, loop)
		}
	}
	return concurrentLoops
}

func LoopCanBeConcurrent(loop Loop, fileSet *token.FileSet, run *sarif.Run, fileLocation *sarif.PhysicalLocation, acceptMap map[string]int, info *types.Info, out io.Writer) bool {
	// Conditions:
	// - Loop variable is unique for every iteration
	// 		- Make sure by checking that it is present in Init, Cond and Post
	// 		- Make sure it is not written to
	// - No writing to variables declared outside the for-loop
	// 		- exception made for arrays accessed through using loop var as index
	// - No reading from arrays that are written to in the loop body
	// 		- Potential issue: What if we read from an array that is not written to by the current loop,
	// 			but is written to by an outer loop that is also turned into a goroutine? - should be caught by the outer loop refactoring
	// - No method calls with receivers that are not declared in the same scope
	// 		- No function calls that include a struct from an outer scope
	// 		- Allow certain types to be used in both these scenarios - e.g. allow "image.Image", manually specified.
	//			Maybe also matched with variable name? To allow a specific combo?
	// - No return statements in non-function children
	// - No break and goto that would break out of the loop in question
	// - No defer calls that use loop-local variables, or whose execution depends on control flow altering elements

	// Make sure that the Loop variable is unique for every iteration
	var loopVars []*ast.Ident

	if loop.For != nil {
		loopVars = findLoopVars(loop.For)
	} else if loop.Range != nil {
		loopVars = findRangeLoopVars(loop.Range)
	} else {
		// This should never happen
		_, _ = fmt.Fprintf(out, "Error: Loop at line", fileSet.Position(loop.Pos).Line, "is neither a for-loop nor a range-loop\n")
		return false
	}
	if loopVars == nil {
		// This loop cannot be made concurrent
		_, _ = fmt.Fprintf(out, "Rejected:", fileSet.Position(loop.Pos).Line, "; it does not have a unique Loop variable\n")
		return false
	}

	// - what labels exist within the for-loop?
	foundLabels := findLabels(loop.Body)

	// - what arrays are written to?
	arraysWrittenTo, err := findLHSIndexExpr(loop.Body, fileSet)
	if err != nil {
		_, _ = fmt.Fprintf(out, err.Error())
		return false
	}
	// - what arrays are read from?
	arraysReadFrom, err := findRHSIndexExpr(loop.Body, fileSet)
	if err != nil {
		_, _ = fmt.Fprintf(out, err.Error())
		return false
	}

	// ensure that no arrays are both read from and written to
	for arr, _ := range arraysWrittenTo {
		// TODO: This won't work, because the map keys are not the same
		// To fix this, need to use... object location?
		if _, exists := arraysReadFrom[arr]; exists {
			// the array is both read from and written to; this is not allowed
			_, _ = fmt.Fprintf(out, "Rejected: %d ; it reads from and writes to the same array: %s\n", fileSet.Position(loop.Pos).Line, arr.Name)
			return false
		}
	}

	// - what variables are written to?
	//assignedTo := FindAssignedIdentifiers(loop)

	canMakeConcurrent := true

	// Things to keep track of as we enter:
	// - are we in another for-loop?
	// - are we in a switch or select?
	// - are we in a function?
	// external vars as we delve deep?

	// in order to keep track of what kind of scope we're in, the easy solution is to use a stack
	// However, special care needs to be taken because ast.Inspect has no built-in way of doing something once
	// we move on from a node's scope. Because of this, whenever we enter a new node, we need to check if we're
	// still in the same scope, and if not, we need to pop the stack until we are, or the stack is empty
	// The stack contains only the constructs we care about, and not the for-loop itself
	stack := make([]ast.Node, 0)

	// When we return true in ast.Inspect, it will first visit all child nodes, and then call f(nil)
	// This means that we can use this to pop the stack
	// However, this also means that we can only return true for elements we add *to* the stack, and not other nodes
	// This is because we don't want to pop the stack when we're still in the same scope
	// Need to categorise, see which types of nodes that have scopes

	ast.Inspect(loop.Body, func(n ast.Node) bool {
		if n == nil {
			// pop the stack
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return true
		}
		switch n.(type) {
		case *ast.ForStmt:
			// push the for-loop onto the stack
			stack = append(stack, n)
			return true
		case *ast.RangeStmt:
			// push the range statement onto the stack
			stack = append(stack, n)
			return true
		case *ast.SwitchStmt:
			// push the switch statement onto the stack
			stack = append(stack, n)
			return true
		case *ast.TypeSwitchStmt:
			// push the type switch statement onto the stack
			stack = append(stack, n)
			return true
		case *ast.SelectStmt:
			// push the select statement onto the stack
			stack = append(stack, n)
			return true
		case *ast.FuncDecl:
			// push the function declaration onto the stack
			stack = append(stack, n)
			return true
		case *ast.BlockStmt:
			// push the block statement onto the stack
			stack = append(stack, n)
			return true
		case *ast.CaseClause:
			// push the case clause onto the stack
			stack = append(stack, n)
			return true
		case *ast.CommClause:
			// push the comm clause onto the stack
			stack = append(stack, n)
			return true
		case *ast.LabeledStmt:
			// push the label onto the stack
			stack = append(stack, n)
			return true
		case *ast.IfStmt:
			// push the if statement onto the stack
			stack = append(stack, n)
			return true
		// Above this line are all the nodes that have scopes; they get added to the stack
		// Below this line are other nodes; this is where we look for things that are not allowed
		case *ast.ReturnStmt:
			if !stackContains(stack, reflect.TypeOf(&ast.FuncDecl{})) {
				// return statement found without an enclosing function; this is not allowed
				canMakeConcurrent = false
				_, _ = fmt.Fprintf(out, "Rejected: %d; it contains a return statement outside a function\n", fileSet.Position(loop.Pos).Line)
			}
		// BranchStmt is the common denominator for break, continue, goto, and fallthrough
		// fallthrough is only used in switch; we don't care about those
		case *ast.BranchStmt:
			b := n.(*ast.BranchStmt)
			if b.Label != nil {
				// make sure that the label we want to goto is in the stack
				isInStack := false
				for ident, _ := range foundLabels {
					if b.Label.Name == ident.Name {
						isInStack = true
						break
					}
				}
				if !isInStack {
					// the label we want to goto is not in the stack; this is not allowed
					canMakeConcurrent = false
					_, _ = fmt.Fprintf(out, "Rejected: %d ; it contains a goto statement to a label outside the Loop\n", fileSet.Position(loop.Pos).Line)
				}
			}
			switch b.Tok {
			case token.BREAK:
				if !stackContains(stack,
					reflect.TypeOf(&ast.ForStmt{}),
					reflect.TypeOf(&ast.RangeStmt{}),
					reflect.TypeOf(&ast.SelectStmt{}),
					reflect.TypeOf(&ast.SwitchStmt{}),
					reflect.TypeOf(&ast.TypeSwitchStmt{})) {
					// break statement found without an enclosing construct; this is not allowed
					canMakeConcurrent = false
					_, _ = fmt.Fprintf(out, "Rejected: %d ; it contains a break statement trying to break the outer loop\n", fileSet.Position(loop.Pos).Line)
				}
			}
		}
		return false
	})

	if !canMakeConcurrent {
		return false
	}

	// The above inspect looks at control flow statements
	// The below inspect looks at other nodes; specifically assignments and function/method calls

	ast.Inspect(loop.Body, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		switch n.(type) {
		case *ast.AssignStmt:
			// check if the assignment is to an index of an array or map
			// in an assign, we care that identifiers on the left are 1. local, or 2. arrays accessed by loop index
			for _, lhs := range n.(*ast.AssignStmt).Lhs {
				switch lhs.(type) {
				// an assignment's lhs must be addressable, meaning variable, pointer, or slice index operation; a field selector; an array indexing operation
				case *ast.IndexExpr:
					// check if the indexExpr contains the Loop variable
					indexExpr := lhs.(*ast.IndexExpr)
					// if it is an index expression, it cannot be a map type
					base := getBaseOfIndex(indexExpr)
					if base == nil {
						canMakeConcurrent = false
						_, _ = fmt.Fprintf(out, "Rejected: %d ; base of index expression is not an identifier\n", fileSet.Position(loop.Pos).Line)
						return false
					}
					typeof := info.TypeOf(base)
					if typeof == nil {
						canMakeConcurrent = false
						_, _ = fmt.Fprintf(out, "Rejected: %d ; cannot determine type of indexed variable %s\n", fileSet.Position(loop.Pos).Line, base.Name)
						return false
					}
					typeof = getUnderlying(typeof)
					// only allowed if it's a slice or array underneath
					_, slice := typeof.(*types.Slice)
					_, arr := typeof.(*types.Array)
					if !slice && !arr {
						canMakeConcurrent = false
						_, _ = fmt.Fprintf(out, "Rejected: %d ; only slices and arrays are allowed; typeof: %s\n", fileSet.Position(loop.Pos).Line, typeof.String())
						return false
					}

					if indexContainsLoopVar(indexExpr, loopVars) {
						// the indexExpr contains the Loop variable; this is allowed
						continue
					}
					// mark as invalid
					canMakeConcurrent = false
					_, _ = fmt.Fprintf(out, "Rejected: %d ; it writes to an array using a non-Loop variable as the index\n", fileSet.Position(loop.Pos).Line)
					return false
				case *ast.Ident:
					ident := lhs.(*ast.Ident)
					// check if the identifier is the Loop variable
					for _, i := range loopVars {
						if i.Obj == ident.Obj {
							// the identifier is the Loop variable; this is not allowed
							canMakeConcurrent = false
							_, _ = fmt.Fprintf(out, "Rejected: %d ; it writes to the Loop variable\n", fileSet.Position(loop.Pos).Line)
							return false
						}
					}
					// check if the identifier is declared within the Loop
					if ident.Obj.Pos() >= loop.Pos && ident.Obj.Pos() <= loop.End {
						// the identifier is declared within the Loop; this is allowed
						continue
					}
					// mark as invalid
					canMakeConcurrent = false
					_, _ = fmt.Fprintf(out, "Rejected: %d ; it writes to '%s' declared outside the Loop\n", fileSet.Position(loop.Pos).Line, ident.Name)
				default:
					// unsupported assignment type
					canMakeConcurrent = false
					_, _ = fmt.Fprintf(out, "Rejected: %d ; it writes to an unsupported expression\n", fileSet.Position(loop.Pos).Line)
				}
			}
		case *ast.CallExpr:
			// check if the call is a method call
			if call, ok := n.(*ast.CallExpr); ok {
				if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
					// check if the receiver is an identifier
					if ident, ok := selector.X.(*ast.Ident); ok {
						// check if the identifier is declared within the Loop
						if ident.Obj != nil && (ident.Obj.Pos() < loop.Pos || ident.Obj.Pos() > loop.End) {
							// check if this is an accepted identifier
							if line, exists := acceptMap[ident.Name]; exists && line == fileSet.Position(loop.Pos).Line {
								// this has been manually approved
								_, _ = fmt.Fprintf(out, "Manually approved")
								return canMakeConcurrent
							}
							// mark as invalid
							canMakeConcurrent = false
							_, _ = fmt.Fprintf(out, "Rejected: %d ; it calls a method on '%s' declared outside the Loop\n", fileSet.Position(loop.Pos).Line, ident.Name)
						}
						// the identifier is declared within the Loop; this is allowed
					}
				}
			}
			// it's a function call, not a method call
			//
		}

		//if ident, ok := n.(*ast.Ident); ok {
		//	if _, exists := assignedTo[ident]; exists {
		//		if assignedTo[ident] {
		//			// This is a good candidate for a unit test
		//			//println("Rejected:", fileSet.Position(loop.Pos()).Line, "; it writes to '"+ident.Name+"' declared outside the Loop")
		//			if run != nil {
		//				addRunResult(run, "PERFACTOR_RULE_001", "Cannot make Loop ; it writes to '"+ident.Name+"' declared outside the Loop", fpath, loop.Pos(), fileSet)
		//			}
		//			canMakeConcurrent = false
		//			// no need to look into subtrees of this node
		//			return false
		//		}
		//	}
		//}
		return canMakeConcurrent
	})

	return canMakeConcurrent
}

func getUnderlying(typeof types.Type) types.Type {
	if typeof.Underlying() != typeof {
		return getUnderlying(typeof.Underlying())
	}
	return typeof
}

func findRangeLoopVars(loop *ast.RangeStmt) []*ast.Ident {
	if loop.Key == nil && loop.Value == nil {
		return nil
	}
	idents := make([]*ast.Ident, 0)

	if loop.Key != nil {
		if id, ok := loop.Key.(*ast.Ident); ok {
			idents = append(idents, id)
		}
	}
	if loop.Value != nil {
		if id, ok := loop.Value.(*ast.Ident); ok {
			idents = append(idents, id)
		}
	}
	return idents
}

func findLoopVars(loop *ast.ForStmt) []*ast.Ident {
	if loop.Init == nil || loop.Cond == nil || loop.Post == nil {
		return nil
	}

	// find the idents declared in the init
	idents := make([]*ast.Ident, 0)
	for _, i := range loop.Init.(*ast.AssignStmt).Lhs {
		if id, ok := i.(*ast.Ident); ok {
			idents = append(idents, id)
		}
	}
	// make sure they're all altered in the post
	switch loop.Post.(type) {
	case *ast.IncDecStmt:
		// Only one thing is being incremented; keep it in the list, but remove everything else
		for _, j := range idents {
			if ident, ok := loop.Post.(*ast.IncDecStmt).X.(*ast.Ident); ok {
				if j.Obj == ident.Obj {
					idents = []*ast.Ident{j}
					break
				}
			}
		}
	case *ast.AssignStmt:
	outer:
		for _, j := range loop.Post.(*ast.AssignStmt).Lhs {
			if id, ok := j.(*ast.Ident); ok {
				// check if the identifier is in the list
				for _, ident := range idents {
					if ident.Obj == id.Obj {
						continue outer
					}
				}
				// if we get here, the identifier is not in the list
				// remove it
				for _, ident := range idents {
					if ident.Obj == id.Obj {
						idents = append(idents[:ident.Obj.Pos()], idents[ident.Obj.Pos()+1:]...)
					}
				}
			}
		}
	}

	// at this point, we have all the idents declared in the init, and removed any that are not altered in the post
	// now, at least one needs to be used in the condition
	found := false
	ast.Inspect(loop.Cond, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if ident, ok := n.(*ast.Ident); ok {
			for _, i := range idents {
				if ident.Obj == i.Obj {
					found = true
					return false
				}
			}
		}
		return true
	})
	if !found {
		return nil
	}
	return idents
}

func indexContainsLoopVar(expr *ast.IndexExpr, loopVars []*ast.Ident) bool {
	// assume here that all we have to deal with are identifiers and index expressions
	// this is wrong, but it is all we support for now
	if ident, ok := expr.Index.(*ast.Ident); ok {
		for _, i := range loopVars {
			if i.Obj == ident.Obj {
				return true
			}
		}
	}
	if index, ok := expr.X.(*ast.IndexExpr); ok {
		if indexContainsLoopVar(index, loopVars) {
			return true
		}
	}
	return false
}

func stackContains(stack []ast.Node, typeOf ...reflect.Type) bool {
	for _, node := range stack {
		for _, t := range typeOf {
			if reflect.TypeOf(node) == t {
				return true
			}
		}
	}
	return false
}

func findLabels(loop ast.Node) map[*ast.Ident]token.Pos {
	identMap := make(map[*ast.Ident]token.Pos)
	ast.Inspect(loop, func(node ast.Node) bool {
		if label, ok := node.(*ast.LabeledStmt); ok {
			identMap[label.Label] = label.Pos()
		}
		return true
	})
	return identMap
}

func findLHSIndexExpr(loop ast.Node, fileSet *token.FileSet) (map[*ast.Ident]bool, error) {
	return findXHSIndexExpr(loop, fileSet, true)
}

func findRHSIndexExpr(loop ast.Node, fileSet *token.FileSet) (map[*ast.Ident]bool, error) {
	return findXHSIndexExpr(loop, fileSet, false)
}

func findXHSIndexExpr(loop ast.Node, fileSet *token.FileSet, left bool) (map[*ast.Ident]bool, error) {
	// a map that marks all the arrays (indexable identifiers) within the for-loop which are read from
	//		- and also used for writing, in an assignment
	identMap := make(map[*ast.Ident]bool)
	var err error
	ast.Inspect(loop, func(n ast.Node) bool {
		// filter for assignments
		if assignment, ok := n.(*ast.AssignStmt); ok {
			// we're only looking at one side at a time
			var arrs []ast.Expr
			if left {
				arrs = assignment.Lhs
			} else {
				arrs = assignment.Rhs
			}
			for _, expr := range arrs {
				ast.Inspect(expr, func(node ast.Node) bool {
					// We're only interested once we get to an index expression
					if index, ok := node.(*ast.IndexExpr); ok {
						// Traverse the index expr, adding the identifier to the map
						// This also traverses multiple index expressions
						// This does *not* handle other expressions - like an array literal, or a function call that returns an array
						// If any of these are found, it will abort
						result := traverseExpr(expr, identMap)
						if !result {
							pos := fileSet.Position(index.Pos())
							err = errors.New("Unhandled expression at " + string(rune(pos.Line)) + ":" + string(rune(pos.Column)) + " in " + pos.Filename)
							return false
						}
					}
					return true
				})
				if err != nil {
					return false
				}
			}
		}
		return true
	})
	return identMap, err
}

func traverseExpr(expr ast.Expr, identMap map[*ast.Ident]bool) bool {
	// This function traverses an expression, adding any identifiers it finds to the identMap
	switch expr.(type) {
	case *ast.IndexExpr:
		index := expr.(*ast.IndexExpr)
		return traverseExpr(index.X, identMap)
	case *ast.Ident:
		ident := expr.(*ast.Ident)
		identMap[ident] = true
		return true
	case *ast.ParenExpr:
		paren := expr.(*ast.ParenExpr)
		return traverseExpr(paren.X, identMap)
	case *ast.SelectorExpr:
		selector := expr.(*ast.SelectorExpr)
		identMap[selector.Sel] = true
		// Is this always correct? Are there times where we should be traversing the X?
		return true //traverseExpr(selector.X, identMap)
	case *ast.IndexListExpr:
		index := expr.(*ast.IndexListExpr)
		return traverseExpr(index.X, identMap)
	case *ast.SliceExpr:
		slice := expr.(*ast.SliceExpr)
		return traverseExpr(slice.X, identMap)
	case *ast.StarExpr:
		star := expr.(*ast.StarExpr)
		return traverseExpr(star.X, identMap)
	case *ast.UnaryExpr:
		unary := expr.(*ast.UnaryExpr)
		return traverseExpr(unary.X, identMap)
	case *ast.BinaryExpr:
		binary := expr.(*ast.BinaryExpr)
		return traverseExpr(binary.X, identMap) && traverseExpr(binary.Y, identMap)
	case *ast.KeyValueExpr:
		keyValue := expr.(*ast.KeyValueExpr)
		return traverseExpr(keyValue.Key, identMap) && traverseExpr(keyValue.Value, identMap)
	case *ast.CallExpr:
		call := expr.(*ast.CallExpr)
		for _, arg := range call.Args {
			if !traverseExpr(arg, identMap) {
				return false
			}
		}
		return true
	default:
		// we have something that's not directly indexing an identifier - we don't handle this currently
		// This includes function calls
		println("Unhandled expression: ", reflect.TypeOf(expr).String())
		return false
	}
}

func getBaseOfIndex(index ast.Expr) *ast.Ident {
	if x, ok := index.(*ast.Ident); ok {
		return x
	}
	if x, ok := index.(*ast.IndexExpr); ok {
		return getBaseOfIndex(x.X)
	}
	return nil
}

func addRunResult(run *sarif.Run, ruleID, messageText, filePath string, pos token.Pos, f *token.FileSet) {
	run.AddResult(ruleID).
		WithLocation(sarif.NewLocationWithPhysicalLocation(sarif.NewPhysicalLocation().
			WithArtifactLocation(sarif.NewArtifactLocation().
				WithUri(filePath)).
			WithRegion(sarif.NewRegion().
				WithStartLine(f.Position(pos).Line).
				WithStartColumn(f.Position(pos).Column)))).
		WithMessage(sarif.NewMessage().WithText(messageText))
}
