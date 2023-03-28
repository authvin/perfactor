package util

import "go/ast"

func FindAssignmentsInLoop(loop *ast.ForStmt, loopVarUsage map[*ast.Ident]bool) {
	ast.Inspect(loop, func(n ast.Node) bool {
		if assignStmt, ok := n.(*ast.AssignStmt); ok {
			// Check if the assignment is to an index of an array or map
			if indexExpr, ok := assignStmt.Lhs[0].(*ast.IndexExpr); ok {
				// if the index expression is an identifier, check if it's the loop variable
				if ident, ok := indexExpr.Index.(*ast.Ident); ok {
					// check if the identifier is the loop variable
					if ident.Obj == loop.Init.(*ast.AssignStmt).Lhs[0].(*ast.Ident).Obj {
						//println("Found an assignment to an array using loop variable as the index, allowing it at ", f.Position(loop.Pos()).Line)
						return true
					}
				}
			}
			for _, lhs := range assignStmt.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					// check if the identifier's declaration is within the loop
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
