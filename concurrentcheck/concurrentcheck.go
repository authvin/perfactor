package concurrentcheck

// inspired by "Using go/analysis to write a custom linter" - F Arslan

import (
	"github.com/owenrumney/go-sarif/sarif"
	"go/ast"
	"go/token"
	"golang.org/x/tools/go/analysis"
	"reflect"
)

var Analyzer = &analysis.Analyzer{
	Name:       "concheck",
	Doc:        "reports for loops that can be made concurrent",
	Run:        run,
	ResultType: reflect.TypeOf(&sarif.Run{}),
}

func run(pass *analysis.Pass) (interface{}, error) {
	// each file is an AST -> inspect each AST. Or do we use a visitor?
	sarifRun := sarif.NewRun("name_placeholder", "uri_placeholder")
	for _, file := range pass.Files {
		// First, we need to be in a for loop
		ast.Walk(&ConcurrentLoopVisitor{
			f:            pass.Fset,
			run:          *sarifRun,
			fileLocation: sarif.NewPhysicalLocation().WithArtifactLocation(sarif.NewArtifactLocation().WithUri(pass.Fset.Position(file.Pos()).Filename)),
			pass:         pass,
		}, file)
	}
	return sarifRun, nil
}

// Does it fit better to include the pass data in in this visitor, or to extend the diagnostic?

type ConcurrentLoopVisitor struct {
	f            *token.FileSet
	run          sarif.Run
	fileLocation *sarif.PhysicalLocation
	pass         *analysis.Pass
}

func (w ConcurrentLoopVisitor) Visit(n ast.Node) ast.Visitor {
	if forStmt, ok := n.(*ast.ForStmt); ok {
		assignedTo := findAssignmentsInLoop(forStmt)
		if w.canBeConcurrent(forStmt, assignedTo) {
			w.pass.Reportf(forStmt.Pos(), "This for loop can be made concurrent")
		}
	}
	return w
}

// Function to see if a for loop is valid for concurrentization
func (w ConcurrentLoopVisitor) canBeConcurrent(loop *ast.ForStmt, assignedTo map[*ast.Ident]bool) bool {
	safe := true
	ast.Inspect(loop, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if assignedTo[ident] {
				w.run.AddResult("PERFACTOR_RULE_001").
					WithLocation(sarif.NewLocationWithPhysicalLocation(w.fileLocation.
						WithRegion(sarif.NewRegion().
							WithStartLine(w.f.Position(loop.Pos()).Line).
							WithStartColumn(w.f.Position(loop.Pos()).Column)))).
					WithMessage(sarif.NewMessage().WithText("Cannot make Loop concurrent because it writes to '" + ident.Name + "' declared outside the Loop"))
				safe = false
				return false
			}
		}
		return true
	})
	return safe
}

// Function to find identifiers that are written to in a for loop
func findAssignmentsInLoop(loop *ast.ForStmt) map[*ast.Ident]bool {
	assignedTo := make(map[*ast.Ident]bool)
	ast.Inspect(loop, func(n ast.Node) bool {
		if assignStmt, ok := n.(*ast.AssignStmt); ok {
			// Check if the assignment is to an index of an array or map
			if indexExpr, ok := assignStmt.Lhs[0].(*ast.IndexExpr); ok {
				// if the index expression is an identifier, check if it's the Loop variable
				if ident, ok := indexExpr.Index.(*ast.Ident); ok {
					// check if the identifier is the Loop variable
					if ident.Obj == loop.Init.(*ast.AssignStmt).Lhs[0].(*ast.Ident).Obj {
						// This is allowed because there's a direct correlation between the index and the Loop variable
						// This means that while it *does* write to an external variable, it does so in a way where
						//		each iteration of the loop writes to a different index of the array
						// This can fail if the loop variable is not unique to each loop iteration
						// *** document a test case where the analysis is wrong but still safe
						return true
					}
				}
			}
			for _, lhs := range assignStmt.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					// check if the identifier's declaration is within the Loop
					if ident.Obj.Pos() < loop.Pos() || ident.Obj.Pos() > loop.End() {
						assignedTo[ident] = true
						// what happens if it's written to inside a nested loop?
						// It's not possible for the variable to be declared in a nested loop - wrong scope
					}
				}
			}
		}
		return true
	})
	return assignedTo
}
