package util

import (
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/ast/astutil"
)

// Function to insert goroutines into for loops that are already known to be safe to refactor
func MakeLoopConcurrent(astFile *ast.File, fset *token.FileSet, loopPos token.Pos, checker types.Checker) {
	astutil.Apply(astFile, func(cursor *astutil.Cursor) bool {
		node := cursor.Node()
		// first half makes sure it's a for statement, second makes sure it's the one in the correct position
		if forLoop, ok := node.(*ast.ForStmt); ok && fset.Position(forLoop.Pos()).Offset == fset.Position(loopPos).Offset {
			// add import for sync and waitgroup
			astutil.AddImport(fset, astFile, "sync")
			wgIdent := ast.NewIdent("wg")
			wgType := &ast.SelectorExpr{
				X:   ast.NewIdent("sync"),
				Sel: ast.NewIdent("WaitGroup"),
			}

			wgDecl := &ast.DeclStmt{
				Decl: &ast.GenDecl{
					Tok: token.VAR,
					Specs: []ast.Spec{
						&ast.ValueSpec{
							Names: []*ast.Ident{wgIdent},
							Type:  wgType,
						},
					},
				},
			}
			cursor.InsertBefore(wgDecl)
			block := &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.DeferStmt{
						Call: &ast.CallExpr{
							Fun: &ast.SelectorExpr{
								X:   wgIdent,
								Sel: ast.NewIdent("Done"),
							},
						},
					},
				},
			}
			// append all the statements in the for Loop to the body of the goroutine
			block.List = append(block.List, forLoop.Body.List...)

			// get the type of the variable being assigned to in the init statement
			//forLoop.Init.(*ast.AssignStmt)
			l := len(forLoop.Init.(*ast.AssignStmt).Rhs) - 1

			var typeList []*ast.Field

			// for each ident in the lhs of the for Loop init
			// create a field with the type from the rhs

			for i := 0; i < len(forLoop.Init.(*ast.AssignStmt).Lhs); i++ {
				typ := checker.TypeOf(forLoop.Init.(*ast.AssignStmt).Rhs[l])
				ident := forLoop.Init.(*ast.AssignStmt).Lhs[i].(*ast.Ident)
				typeList = append(typeList, &ast.Field{
					Type:  ast.NewIdent(typ.String()),
					Names: []*ast.Ident{ast.NewIdent(ident.Name)},
				})
			}

			goStmt := &ast.GoStmt{
				// create a goroutine
				Call: &ast.CallExpr{
					Fun: &ast.FuncLit{
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								// insert the list of types created above
								List: typeList,
							},
						},
						Body: block,
					},
					Args: forLoop.Init.(*ast.AssignStmt).Lhs,
				},
			}

			// Adding one to the wait group per goroutine
			wgAddCall := &ast.ExprStmt{
				X: &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   wgIdent,
						Sel: ast.NewIdent("Add"),
					},
					Args: []ast.Expr{
						ast.NewIdent("1"),
					},
				},
			}

			// Place the go stmt in the for Loop
			forLoop.Body.List = []ast.Stmt{wgAddCall, goStmt}

			wgWaitCall := &ast.ExprStmt{
				X: &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   wgIdent,
						Sel: ast.NewIdent("Wait"),
					},
				},
			}
			cursor.InsertAfter(wgWaitCall)

			return false
		}
		return true
	}, nil)
}

// FindSafeLoopsForRefactoring finds loops that can be refactored to be concurrent
// It returns a list of Loop positions pointing to for and range loops
func FindSafeLoopsForRefactoring(forLoops []*ast.ForStmt, f *token.FileSet) []token.Pos {
	// A map to store Loop variable usage information
	loopVarUsage := make(map[*ast.Ident]bool)

	// The first predicate is that the Loop does not assign any values used within the Loop
	// The Loop should be able to write to a variable it doesn't use - right? If the writing doesn't mind the context... though maybe it wants the last index it goes through?
	// - but that's pretty poor design. Should be enough to acknowledge that this is a weakness, and that a better tool would take this into account
	// Can we check if any of our variables are assigned to in a goroutine? because we'd want to avoid any of those. But then, is it different?
	// So long as it's not a side effect of the Loop itself, the program might change, but it can still be done safely. This might
	// be one of those "we can do this, but it changes behaviour" refactorings.

	// Problem: what about assigning to an array or map, where we're assigning to an index corresponding to the main Loop variable?
	// Solution: Check if the assign is to an index of an array or map, and if so, check if the index is the Loop variable

	// Collect all for Loop variables
	for _, loop := range forLoops {
		FindAssignmentsInLoop(loop, loopVarUsage)
	}

	// Collect all range Loop variables

	// list of loops that can be made concurrent
	var concurrentLoops []token.Pos

	// Now that we have the information, we can filter out the loops that can be made concurrent
	for _, loop := range forLoops {
		// The check we're doing is if the Loop does not write to a variable outside the Loop
		// Thus, if that doesn't trigger, we assume it's safe to refactor
		// add to list of loops that can be made concurrent
		if LoopCanBeConcurrent(loop, loopVarUsage, f) {
			concurrentLoops = append(concurrentLoops, loop.Pos())
		}
	}
	return concurrentLoops
}

func LoopCanBeConcurrent(loop *ast.ForStmt, loopVarUsage map[*ast.Ident]bool, f *token.FileSet) bool {
	canMakeConcurrent := true
	ast.Inspect(loop, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if _, exists := loopVarUsage[ident]; exists {
				if loopVarUsage[ident] {
					// This is a good candidate for a unit test
					println("Cannot make Loop at line", f.Position(loop.Pos()).Line, "concurrent because it writes to '"+ident.Name+"' declared outside the Loop")
					canMakeConcurrent = false
					// no need to look into subtrees of this node
					return false
				}
			}
		}
		return true
	})
	return canMakeConcurrent
}
