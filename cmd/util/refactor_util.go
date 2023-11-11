package util

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"math/rand"
	"strings"
	"time"

	"golang.org/x/tools/go/ast/astutil"
)

// Steps:
//1 Add the import
//
//2 Add the WaitGroup declaration
//3 create a block for the goroutine
//4 create a defer Done call in the block
//5 place the for-loop's statements inside the goroutine statements
//6 add the for-loop's loop variable as an argument to the goroutine with the same name - deliberate shadowing
// 	- do this for all accessed non-const, non-reference values?
//7 create a wg.Add(1) call
//8 empty the for-loop's list of statements and add the wg.Add(1) statement and goroutine statement to it
//9 add a wait-call after the for-loop

func GetConcurrentLoop(n *ast.ForStmt, fset *token.FileSet, info *types.Info) []ast.Stmt {
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
	block.List = append(block.List, handleContinueStatements(n.Body.List)...)

	//-6- set up the go stmt with fields with the types from the rhs of loop var assign statements
	goStmt := makeGoStmt(n, info, block)

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

func GetConcurrentRangeLoop(n *ast.RangeStmt, fset *token.FileSet, info *types.Info) []ast.Stmt {
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
	block.List = append(block.List, handleContinueStatements(n.Body.List)...)

	//-6- set up the go stmt with fields with the types from the rhs of loop var assign statements
	goStmt := makeGoStmtForRange(n, info, block)

	//-7- Adding one to the wait group per goroutine
	wgAddCall := makeAddCall(wgIdent)

	//-8- Place the go stmt in the for Loop
	newForStmt := &ast.RangeStmt{
		Key:   n.Key,
		Value: n.Value,
		X:     n.X,
		Body: &ast.BlockStmt{
			List:   []ast.Stmt{wgAddCall, goStmt},
			Lbrace: n.Body.Lbrace,
			Rbrace: n.Body.Rbrace,
		},
		For:    n.For,
		Tok:    n.Tok,
		TokPos: n.TokPos,
	}

	stmts = append(stmts, newForStmt)

	//-9- place the wait call after the for-loop
	wgWaitCall := makeWaitCall(wgIdent)

	stmts = append(stmts, wgWaitCall)

	return stmts
}

func handleContinueStatements(list []ast.Stmt) []ast.Stmt {
	// traverse - if we find a continue, replace it with a return
	for i := 0; i < len(list); i++ {
		ast.Inspect(list[i], func(n ast.Node) bool {
			if n == nil {
				return false
			}
			if _, ok := n.(*ast.ForStmt); ok {
				// we don't need to look into this; any continue will be referring to the inner loop
				return false
			}
			if _, ok := n.(*ast.RangeStmt); ok {
				return false
			}
			if stmt, ok := n.(*ast.BranchStmt); ok {
				if stmt.Tok == token.CONTINUE {
					list[i] = &ast.ReturnStmt{Return: stmt.Pos()}
				}
			}
			return true
		})
	}
	return list
}

func MakeLoopConcurrent(astFile *ast.File, fset *token.FileSet, line int, info *types.Info) {
	// Function to insert goroutines into for loops that are already known to be safe to refactor
	// add import for sync and waitgroup
	//-1- Is this the right place to do this? This requires the full astFile, which is not ideal
	astutil.AddImport(fset, astFile, "sync")
	astutil.Apply(astFile, func(cursor *astutil.Cursor) bool {
		node := cursor.Node()
		// first half makes sure it's a for statement, second makes sure it's the one in the correct position
		if forLoop, ok := node.(*ast.ForStmt); ok && fset.Position(forLoop.Pos()).Line == line {
			stmts := GetConcurrentLoop(forLoop, fset, info)
			cursor.InsertBefore(stmts[0])
			cursor.Replace(stmts[1])
			cursor.InsertAfter(stmts[2])
			return false
		}
		if rangeLoop, ok := node.(*ast.RangeStmt); ok && fset.Position(rangeLoop.Pos()).Line == line {
			stmts := GetConcurrentRangeLoop(rangeLoop, fset, info)
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

func makeGoStmt(forLoop *ast.ForStmt, info *types.Info, block *ast.BlockStmt) *ast.GoStmt {
	// get the type of the variable being assigned to in the init statement
	l := len(forLoop.Init.(*ast.AssignStmt).Rhs) - 1

	var typeList []*ast.Field

	// for each ident in the lhs of the for Loop init

	for i := 0; i < len(forLoop.Init.(*ast.AssignStmt).Lhs); i++ {

		typ := info.TypeOf(forLoop.Init.(*ast.AssignStmt).Rhs[l])
		ident := forLoop.Init.(*ast.AssignStmt).Lhs[i].(*ast.Ident)
		if ident.Name == "_" {
			println("found _")
		}
		typeList = append(typeList, &ast.Field{
			Type:  ast.NewIdent(cleanType(typ.String())),
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

func makeGoStmtForRange(loop *ast.RangeStmt, info *types.Info, block *ast.BlockStmt) *ast.GoStmt {
	// add the key and val to a list, if they exist
	var typeList []*ast.Field
	var args []ast.Expr

	if loop.Key != nil {
		typ := info.TypeOf(loop.Key)
		ident := loop.Key.(*ast.Ident)
		if ident.Name != "_" {
			typeList = append(typeList, &ast.Field{
				Type:  ast.NewIdent(cleanType(typ.String())),
				Names: []*ast.Ident{ast.NewIdent(ident.Name)},
			})
			args = append(args, loop.Key)
		}
	}
	if loop.Value != nil {
		typ := info.TypeOf(loop.Value)
		ident := loop.Value.(*ast.Ident)
		if ident.Name != "_" {
			typeList = append(typeList, &ast.Field{
				Type:  ast.NewIdent(cleanType(typ.String())),
				Names: []*ast.Ident{ast.NewIdent(ident.Name)},
			})
			args = append(args, loop.Value)
		}
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
			Args: args,
		},
	}
}

func cleanType(s string) string {
	// for imported types, make sure we only include everything after the final /
	if strings.Contains(s, "/") {
		return s[strings.LastIndex(s, "/")+1:]
	}
	return s
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
