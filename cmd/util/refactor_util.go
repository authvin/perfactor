package util

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"math/rand"
	"time"

	"github.com/owenrumney/go-sarif/sarif"
	"golang.org/x/tools/go/ast/astutil"
)

// Steps:
//1 Add the import
//
//2 Add the waitgroup declaration
//3 create a block for the goroutine
//4 create a defer Done call in the block
//5 place the for-loop's statements inside the goroutine statements
//6 add the for-loop's loop variable as an argument to the goroutine with the same name - deliberate shadowing
// 	- do this for all accessed non-const, non-reference values?
//7 create a wg.Add(1) call
//8 empty the for-loop's list of statements and add the wg.Add(1) statement and goroutine statement to it
//9 add a wait-call after the for-loop

func GetConcurrentLoop(n *ast.ForStmt, fset *token.FileSet, checker types.Checker) []ast.Stmt {
	stmts := make([]ast.Stmt, 0)
	// Instead of checking if "wg" exists, we add a four-digit number to the end of it. Not ideal, but mostly functional. Known issue.
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	wgIdent := ast.NewIdent("wg" + fmt.Sprintf("%04d", r.Intn(10000)))

	//-2- insert the waitgroup right before the for-loop
	stmts = append(stmts, makeWaitgroupDecl(wgIdent))
	//-3- && -4- Create the block for the goroutine, and add the wg.Done() call to a deferred call
	block := makeGoroutineBlock(wgIdent)
	//-5- append all the statements in the for Loop to the body of the goroutine
	block.List = append(block.List, n.Body.List...)

	//-6- set up the go stmt with fields with the types from the rhs of loop var assign statements
	goStmt := makeGoStmt(n, checker, block)

	//-7- Adding one to the wait group per goroutine
	wgAddCall := makeAddCall(wgIdent)

	//-8- Place the go stmt in the for Loop
	newForStmt := &ast.ForStmt{
		Cond: n.Cond,
		Post: n.Post,
		Init: n.Init,
		Body: &ast.BlockStmt{
			List:   []ast.Stmt{wgAddCall, goStmt},
			Lbrace: n.Body.Lbrace,
			Rbrace: n.Body.Rbrace,
		},
		For: n.For,
	}

	stmts = append(stmts, newForStmt)

	//-9- place the wait call after the for-loop
	wgWaitCall := makeWaitCall(wgIdent)

	stmts = append(stmts, wgWaitCall)

	return stmts
}

// Function to insert goroutines into for loops that are already known to be safe to refactor
func MakeLoopConcurrent(astFile *ast.File, fset *token.FileSet, loopPos token.Pos, checker types.Checker) {
	// add import for sync and waitgroup
	//-1- Is this the right place to do this? This requires the full astFile, which is not ideal
	astutil.AddImport(fset, astFile, "sync")
	astutil.Apply(astFile, func(cursor *astutil.Cursor) bool {
		node := cursor.Node()
		// first half makes sure it's a for statement, second makes sure it's the one in the correct position
		if forLoop, ok := node.(*ast.ForStmt); ok && fset.Position(forLoop.Pos()).Offset == fset.Position(loopPos).Offset {
			stmts := GetConcurrentLoop(forLoop, fset, checker)
			cursor.InsertBefore(stmts[0])
			cursor.Replace(stmts[1])
			cursor.InsertAfter(stmts[2])
			return false
		}
		return true
	}, nil)
}

func makeWaitgroupDecl(wgIdent *ast.Ident) ast.Stmt {
	wgType := &ast.SelectorExpr{
		X:   ast.NewIdent("sync"),
		Sel: ast.NewIdent("WaitGroup"),
	}

	return &ast.DeclStmt{
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
}

func makeGoroutineBlock(wgIdent *ast.Ident) *ast.BlockStmt {
	return &ast.BlockStmt{
		List: []ast.Stmt{
			//-4- add defer wg.Done()
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
}

func makeGoStmt(forLoop *ast.ForStmt, checker types.Checker, block *ast.BlockStmt) *ast.GoStmt {
	// get the type of the variable being assigned to in the init statement
	l := len(forLoop.Init.(*ast.AssignStmt).Rhs) - 1

	var typeList []*ast.Field

	// for each ident in the lhs of the for Loop init

	for i := 0; i < len(forLoop.Init.(*ast.AssignStmt).Lhs); i++ {
		typ := checker.TypeOf(forLoop.Init.(*ast.AssignStmt).Rhs[l])
		ident := forLoop.Init.(*ast.AssignStmt).Lhs[i].(*ast.Ident)
		typeList = append(typeList, &ast.Field{
			Type:  ast.NewIdent(typ.String()),
			Names: []*ast.Ident{ast.NewIdent(ident.Name)},
		})
	}

	return &ast.GoStmt{
		// create a goroutine
		Call: &ast.CallExpr{
			Fun: &ast.FuncLit{
				Type: &ast.FuncType{
					Params: &ast.FieldList{
						// insert the list of types created above
						List: typeList,
					},
				},
				// insert the block created above
				Body: block,
			},
			// add the loop variable as an argument
			Args: forLoop.Init.(*ast.AssignStmt).Lhs,
		},
	}
}

func makeAddCall(wgIdent *ast.Ident) *ast.ExprStmt {
	return &ast.ExprStmt{
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
}

func makeWaitCall(wgIdent *ast.Ident) *ast.ExprStmt {
	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   wgIdent,
				Sel: ast.NewIdent("Wait"),
			},
		},
	}
}

// FindSafeLoopsForRefactoring finds loops that can be refactored to be concurrent
// It returns a list of Loop positions pointing to for and range loops
func FindSafeLoopsForRefactoring(forLoops []*ast.ForStmt, f *token.FileSet, run *sarif.Run, fpath string) []token.Pos {
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

	// list of loops that can be made concurrent
	var concurrentLoops []token.Pos

	// Now that we have the information, we can filter out the loops that can be made concurrent
	for _, loop := range forLoops {
		// The check we're doing is if the Loop does not write to a variable outside the Loop
		// Thus, if that doesn't trigger, we assume it's safe to refactor
		// add to list of loops that can be made concurrent
		if LoopCanBeConcurrent(loop, loopVarUsage, f, run, fpath) {
			concurrentLoops = append(concurrentLoops, loop.Pos())
		}
	}
	return concurrentLoops
}

func LoopCanBeConcurrent(loop *ast.ForStmt, loopVarUsage map[*ast.Ident]bool, f *token.FileSet, run *sarif.Run, fpath string) bool {
	canMakeConcurrent := true
	ast.Inspect(loop, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if _, exists := loopVarUsage[ident]; exists {
				if loopVarUsage[ident] {
					// This is a good candidate for a unit test
					//println("Cannot make Loop at line", f.Position(loop.Pos()).Line, "concurrent because it writes to '"+ident.Name+"' declared outside the Loop")
					if run != nil {
						run.AddResult("PERFACTOR_RULE_001").
							WithLocation(sarif.NewLocationWithPhysicalLocation(sarif.NewPhysicalLocation().
								WithArtifactLocation(sarif.NewArtifactLocation().
									WithUri(fpath)).
								WithRegion(sarif.NewRegion().
									WithStartLine(f.Position(loop.Pos()).Line).
									WithStartColumn(f.Position(loop.Pos()).Column)))).
							WithMessage(sarif.NewMessage().WithText("Cannot make Loop concurrent because it writes to '" + ident.Name + "' declared outside the Loop"))
					}
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
