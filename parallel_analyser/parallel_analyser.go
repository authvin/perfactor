package parallel_analyser

// inspired by "Using go/analysis to write a custom linter" - F Arslan

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/analysis/singlechecker"
	"os"
	"perfactor/cmd/util"
	"reflect"

	"github.com/owenrumney/go-sarif/sarif"
	"golang.org/x/tools/go/analysis"
)

func main() {
	singlechecker.Main(Analyzer)
}

var Analyzer = &analysis.Analyzer{
	Name:       "concheck",
	Doc:        "reports for loops that can be made concurrent",
	Run:        run,
	ResultType: reflect.TypeOf(&sarif.Run{}),
}

type RunResult struct {
	Run   sarif.Run
	Loops util.LoopInfoArray
}

// It doesn't seem possible to smuggle verbose, info or the acceptmap into the analyser, which is a shame
// This means that we can't use the same code for the analyser and the normal code, most likely
// Alternatively, we need to get information here that is normally fetched higher up in the chain?
// the verbose and the accept map are hard to get, and we get the info from packages...
// yeah, those are all going to be very hard to fake... So unless we can smuggle it out, I'm not sure quite how to do this...
// The only way I can see is smuggling it in through

func run(pass *analysis.Pass) (interface{}, error) {
	// each file is an AST -> inspect each AST. Or do we use a visitor?
	sarifRun := sarif.NewRun("name_placeholder", "uri_placeholder")
	loops := make(util.LoopInfoArray, 0)
	for _, file := range pass.Files {
		info := util.GetTypeCheckerInfoFromFile(file.Name.Name, []*ast.File{file}, pass.Fset)
		// First, we need to be in a for loop
		ast.Walk(&ConcurrentLoopVisitor{
			// need to add a checker here!
			info:         *info,
			f:            pass.Fset,
			run:          *sarifRun,
			fileLocation: sarif.NewPhysicalLocation().WithArtifactLocation(sarif.NewArtifactLocation().WithUri(pass.Fset.Position(file.Pos()).Filename)),
			pass:         pass,
			loops:        &loops,
			verbose:      true,
		}, file)
	}
	return sarifRun, nil
}

// Does it fit better to include the pass data in this visitor, or to extend the diagnostic?

type ConcurrentLoopVisitor struct {
	f            *token.FileSet
	run          sarif.Run
	fileLocation *sarif.PhysicalLocation
	pass         *analysis.Pass
	info         types.Info
	loops        *util.LoopInfoArray
	verbose      bool
	acceptMap    map[string]int
}

func (w ConcurrentLoopVisitor) Visit(n ast.Node) ast.Visitor {
	if forStmt, ok := n.(*ast.ForStmt); ok {
		loop := util.Loop{
			For:     forStmt,
			Pos:     forStmt.Pos(),
			End:     forStmt.End(),
			Body:    forStmt.Body,
			Line:    w.f.Position(forStmt.Pos()).Line,
			EndLine: w.f.Position(forStmt.End()).Line,
		}
		if util.LoopCanBeConcurrent(loop, w.f, &w.run, w.fileLocation, w.acceptMap, &w.info, os.Stdout) {
			// Get the statements that will replace the for loop
			newStmts := util.GetConcurrentLoop(forStmt, w.f, &w.info)
			var buf bytes.Buffer
			for _, stmt := range newStmts {
				// write each statement to the buffer
				if err := format.Node(&buf, token.NewFileSet(), stmt); err != nil {
					return nil
				}
				// newline between the statements
				buf.WriteByte('\n')
			}
			w.pass.Report(analysis.Diagnostic{
				Pos:     n.Pos(),
				End:     n.Pos() + token.Pos(len("for")),
				Message: "This for loop can be made concurrent",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Make loop concurrent",
					TextEdits: []analysis.TextEdit{{
						Pos:     n.Pos(),
						End:     n.End(),
						NewText: buf.Bytes(),
					}, {
						Pos:     token.NoPos,
						End:     token.NoPos,
						NewText: []byte("\n\"sync\""),
					}},
				}},
			})
		}
	}
	return w
}
