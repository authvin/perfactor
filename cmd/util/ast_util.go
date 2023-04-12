package util

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
)

func FindAssignmentsInLoop(loop *ast.ForStmt, loopVarUsage map[*ast.Ident]bool) {
	ast.Inspect(loop, func(n ast.Node) bool {
		if assignStmt, ok := n.(*ast.AssignStmt); ok {
			// Check if the assignment is to an index of an array or map
			if indexExpr, ok := assignStmt.Lhs[0].(*ast.IndexExpr); ok {
				// if the index expression is an identifier, check if it's the Loop variable
				if ident, ok := indexExpr.Index.(*ast.Ident); ok {
					// check if the identifier is the Loop variable
					if ident.Obj == loop.Init.(*ast.AssignStmt).Lhs[0].(*ast.Ident).Obj {
						//println("Found an assignment to an array using Loop variable as the index, allowing it at ", f.Position(Loop.Pos()).Line)
						return true
					}
				}
			}
			for _, lhs := range assignStmt.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					// check if the identifier's declaration is within the Loop
					if ident.Obj.Pos() < loop.Pos() || ident.Obj.Pos() > loop.End() {
						// document a test case where the analysis is wrong but still safe
						//println("found identifier: ", ident.Name, " at line ", f.Position(ident.Pos()).Line)
						loopVarUsage[ident] = true
					}
				}
			}
		}
		return true
	})
}

func FindForLoopsInAST(astFile ast.Node) []*ast.ForStmt {
	// array of AST positions for for loops
	var forLoops []*ast.ForStmt

	// Traverse the AST looking for loops
	ast.Inspect(astFile, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ForStmt:
			//fmt.Println("Found a for Loop at line", fset.Position(n.Pos()).Line)
			forLoops = append(forLoops, n)
		}
		return true
	})
	return forLoops
}

func GetASTFromFile(inputPath string, fset *token.FileSet) *ast.File {
	// parse the file
	f, err := parser.ParseFile(fset, inputPath, nil, parser.ParseComments)
	if err != nil {
		println("Failed to parse AST from file: " + err.Error())
		return nil
	}
	return f
}

func GetTypeCheckerInfo(astFile *ast.File, fset *token.FileSet) *types.Info {
	// get type information from the type checker
	conf := types.Config{Importer: importer.Default()}
	info := &types.Info{
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	// fill in the info object using the type checker
	_, err := conf.Check(astFile.Name.Name, fset, []*ast.File{astFile}, info)
	if err != nil {
		println("Failed to get type checker: " + err.Error())
		os.Exit(0)
	}
	return info
}
