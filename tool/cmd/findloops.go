package cmd

import (
	"fmt"
	"github.com/google/pprof/profile"
	"github.com/spf13/cobra"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/ast/astutil"
	"os"
	gr "perfactor/graph"
	"sort"
)

var Source string
var ProfileSource = ""

// Define the Cobra command
var findloopsCmd = &cobra.Command{
	Use:     "findloops",
	Aliases: []string{"fl"},
	Short:   "Find loops in the code",
	Run:     findloops,
}

func init() {
	findloopsCmd.Flags().StringVarP(&Source, "source", "s", "", "source file to read from")
	findloopsCmd.Flags().StringVarP(&ProfileSource, "profile", "p", "", "file to read profile data from")
	rootCmd.AddCommand(findloopsCmd)
}

func findloops(cmd *cobra.Command, args []string) {
	// This will initially just go through the AST and print the lines at which it finds a for loop
	// Then, it will attempt to traverse the graph with profiling data, and through it, find the appropriate for loops

	// Parse the Golang source file to obtain the AST.
	fset := token.NewFileSet()
	println(Source)
	astFile, err := parser.ParseFile(fset, Source, nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}
	// array of AST positions for for loops
	var forLoops []*ast.ForStmt
	// array of AST positions for range loops
	var rangeLoops []*ast.RangeStmt

	// Traverse the AST looking for loops
	ast.Inspect(astFile, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ForStmt:
			//fmt.Println("Found a for loop at line", fset.Position(n.Pos()).Line)
			forLoops = append(forLoops, n)
		case *ast.RangeStmt:
			//fmt.Println("Found a range loop at line", fset.Position(n.Pos()).Line)
			rangeLoops = append(rangeLoops, n)
		}
		return true
	})
	var dataFromProfileSorting loopTimeArray = nil
	// look through the profile data and find the for loops that are the most expensive
	if ProfileSource != "" {
		// read the profile data
		rawProfile, err := os.Open("1-cpu.pprof")
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		prof, err := profile.Parse(rawProfile)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		graph := gr.GetGraphFromProfile(prof)
		// find the for loops that are the most expensive
		dataFromProfileSorting = sortLoopsUsingProfileData(graph, forLoops, rangeLoops, fset, astFile)
	}

	// Look through the for loops and range loops and find the ones that are possible to make concurrent
	// This will be done by looking at the variables that are assigned in the loop and seeing if they are declared outside of the loop
	// If they are, then the loop cannot be made concurrent
	// If they are not, then the loop can be made concurrent
	safeLoops := findSafeLoopsForRefactoring(forLoops, rangeLoops, fset)

	// filter safeLoops using the data from the profiling
	if dataFromProfileSorting != nil {
		safeLoops = filterLoopsUsingProfileData(safeLoops, dataFromProfileSorting, fset)
	}

	// get type information from the type checker
	conf := types.Config{Importer: importer.Default()}
	info := &types.Info{
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
		Types: make(map[ast.Expr]types.TypeAndValue),
	}

	_, err = conf.Check(astFile.Name.Name, fset, []*ast.File{astFile}, info)
	if err != nil {
		println(err)
		return
	}

	checker := types.Checker{
		Info: info,
	}

	for _, loop := range safeLoops {
		makeLoopConcurrent(astFile, fset, loop, checker)
	}
	// write the modified astFile to a new file
	file, err := os.Create("../temp.go")
	if err != nil {
		fmt.Println(err)
		return
	}
	err = printer.Fprint(file, fset, astFile)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func filterLoopsUsingProfileData(safeLoops []token.Pos, sorted loopTimeArray, fset *token.FileSet) []token.Pos {
	output := make([]token.Pos, 0)
	for _, lt := range sorted {
		loop, time := lt.loop, lt.time
		// check if the loop is in the list of safe loops
		if !contains(safeLoops, loop.Pos()) {
			continue
		}
		if time == 0 {
			// if the time is 0, then the loop is not worth making concurrent
			continue
		}
		println("Loop at line ", fset.Position(loop.Pos()).Line, " has a total time of ", time)
		output = append(output, loop.Pos())
	}
	return output
}

func contains(loops []token.Pos, pos token.Pos) bool {
	for _, loop := range loops {
		if loop == pos {
			return true
		}
	}
	return false
}

// type to contain both a reference to a loop and the cumulative time of the loop
// Use this in order to be able to sort the array

type loopTime struct {
	loop *ast.ForStmt
	time int64
}

type loopTimeArray []loopTime

func (l loopTimeArray) Len() int {
	return len(l)
}

func (l loopTimeArray) Less(i, j int) bool {
	// we want to sort greatest first
	return l[i].time > l[j].time
}

func (l loopTimeArray) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func sortLoopsUsingProfileData(graph *gr.Graph, forLoops []*ast.ForStmt, rangeLoops []*ast.RangeStmt, fset *token.FileSet, astFile *ast.File) loopTimeArray {
	// find the graph nodes corresponding to the for loops
	totalCumulativeTime := make(loopTimeArray, len(forLoops))
	for i, loop := range forLoops {
		totalCumulativeTime[i].loop = loop
		// get the line numbers of the loop
		startLine := fset.Position(loop.Pos()).Line
		endLine := fset.Position(loop.End()).Line
		// find the node in the graph with this line number
		nodes := graph.FindNodesByLine(startLine, endLine)
		for _, node := range nodes {
			// we have the node - now we need to get the performance data for it
			//println("Node in loop at line ", startLine, " has a total time of ", node.Cum, " and a self time of ", node.Flat)
			//println("The node has the name ", node.Info.Name)
			// add the cumulative time to the total cumulative time for this loop
			totalCumulativeTime[i].time += node.Cum
		}
	}
	sort.Sort(totalCumulativeTime)

	// now we have the total cumulative time for each loop, sorted from least to greatest. Let's return it
	return totalCumulativeTime
}

// findSafeLoopsForRefactoring finds loops that can be refactored to be concurrent
// It returns a list of loop positions pointing to for and range loops
func findSafeLoopsForRefactoring(forLoops []*ast.ForStmt, rangeLoops []*ast.RangeStmt, f *token.FileSet) []token.Pos {
	// A map to store loop variable usage information
	loopVarUsage := make(map[*ast.Ident]bool)

	// The first predicate is that the loop does not assign any values used within the loop
	// The loop should be able to write to a variable it doesn't use - right? If the writing doesn't mind the context... though maybe it wants the last index it goes through?
	// - but that's pretty poor design. Should be enough to acknowledge that this is a weakness, and that a better tool would take this into account
	// Can we check if any of our variables are assigned to in a goroutine? because we'd want to avoid any of those. But then, is it different?
	// So long as it's not a side effect of the loop itself, the program might change, but it can still be done safely. This might
	// be one of those "we can do this, but it changes behaviour" refactorings.

	// Problem: what about assigning to an array or map, where we're assigning to an index corresponding to the main loop variable?
	// Solution: Check if the assign is to an index of an array or map, and if so, check if the index is the loop variable

	// Collect all for loop variables
	for _, loop := range forLoops {
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

	// Collect all range loop variables
	// TODO: Do this when the forloop works as intended

	// list of loops that can be made concurrent
	var concurrentLoops []token.Pos

	// Now that we have the information, we can filter out the loops that can be made concurrent
	for _, loop := range forLoops {
		// The check we're doing is if the loop does not write to a variable outside the loop
		// Thus, if that doesn't trigger, we assume it's safe to refactor
		canMakeConcurrent := true
		ast.Inspect(loop, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok {
				if _, exists := loopVarUsage[ident]; exists {
					if loopVarUsage[ident] {
						// This is a good candidate for a unit test
						println("Cannot make loop at line", f.Position(loop.Pos()).Line, "concurrent because it writes to '"+ident.Name+"' declared outside the loop")
						canMakeConcurrent = false
						// no need to look into subtrees of this node
						return false
					}
				}
			}
			return true
		})
		// add to list of loops that can be made concurrent
		if canMakeConcurrent {
			concurrentLoops = append(concurrentLoops, loop.Pos())
		}
	}
	return concurrentLoops
}

// Function to insert goroutines into for loops that are already known to be safe to refactor
func makeLoopConcurrent(astFile *ast.File, fset *token.FileSet, loopPos token.Pos, checker types.Checker) {
	astutil.Apply(astFile, func(cursor *astutil.Cursor) bool {
		node := cursor.Node()

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
			// append all the statements in the for loop to the body of the goroutine
			block.List = append(block.List, forLoop.Body.List...)

			// get the type of the variable being assigned to in the init statement
			//forLoop.Init.(*ast.AssignStmt)
			l := len(forLoop.Init.(*ast.AssignStmt).Rhs) - 1

			var typeList []*ast.Field

			// for each ident in the lhs of the for loop init
			// create a field with the type from the rhs

			for i := 0; i < len(forLoop.Init.(*ast.AssignStmt).Lhs); i++ {
				ident := forLoop.Init.(*ast.AssignStmt).Lhs[i].(*ast.Ident)
				typeList = append(typeList, &ast.Field{
					Type:  ast.NewIdent(checker.TypeOf(forLoop.Init.(*ast.AssignStmt).Rhs[l]).String()),
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

			// Place the go stmt in the for loop
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

/* The previous solution, which had problems and reallydidn't work well
// This function looks for for loops
func makeLoopsConcurrent(astFile *ast.File, fset *token.FileSet, loopPos token.Pos) {
	// Traverse the AST
	ast.Inspect(astFile, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ForStmt:
			if x.Pos() != loopPos {
				return true
			}
			forStmt := n.(*ast.ForStmt)
			// Create a sync.WaitGroup variable and add the required imports
			wgIdent := &ast.Ident{Name: "wg"}
			forStmt.Decls = append(astFile.Decls, &ast.GenDecl{
				Tok: token.VAR,
				Specs: []ast.Spec{
					&ast.ValueSpec{
						Names: []*ast.Ident{wgIdent},
						Type:  &ast.SelectorExpr{X: ast.NewIdent("sync"), Sel: ast.NewIdent("WaitGroup")},
					},
				},
			})
			astFile.Imports = append(astFile.Imports, &ast.ImportSpec{Path: &ast.BasicLit{Value: "\"sync\""}})

			// Replace the for loop with a concurrent
			// Wrap the loop body in a function literal and call it as a goroutine
			loopBodyFunc := &ast.FuncLit{
				Type: &ast.FuncType{Params: &ast.FieldList{}},
				Body: &ast.BlockStmt{List: x.Body.List},
			}
			// The wait group Add call
			wgAddStmt := &ast.ExprStmt{
				X: &ast.CallExpr{
					Fun: &ast.SelectorExpr{X: wgIdent, Sel: ast.NewIdent("Add")},
					Args: []ast.Expr{
						&ast.BasicLit{Kind: token.INT, Value: "1"},
					},
				},
			}
			// the wait group Done call
			wgDoneCall := &ast.DeferStmt{
				Call: &ast.CallExpr{
					Fun: &ast.SelectorExpr{X: wgIdent, Sel: ast.NewIdent("Done")},
				},
			}
			// the Go statement, which contains the loop body
			goStmt := &ast.GoStmt{
				Call: &ast.CallExpr{
					Fun: loopBodyFunc,
				},
			}

			// Update the for loop body with the new statements
			x.Body.List = []ast.Stmt{wgAddStmt, wgDoneCall, goStmt}

			// Add a call to wg.Wait() after the loop
			waitStmt := &ast.ExprStmt{
				X: &ast.CallExpr{
					Fun: &ast.SelectorExpr{X: wgIdent, Sel: ast.NewIdent("Wait")},
				},
			}
			insertStmtAfter(astFile, n, waitStmt)
		}
		return true
	})
}*/
