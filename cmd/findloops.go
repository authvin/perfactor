package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"perfactor/cmd/util"
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

	// Traverse the AST looking for loops
	ast.Inspect(astFile, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ForStmt:
			//fmt.Println("Found a for loop at line", fset.Position(n.Pos()).Line)
			forLoops = append(forLoops, n)
		}
		return true
	})

	dataFromProfileSorting := util.GetDataFromProfile(forLoops, fset)

	// Look through the for loops and range loops and find the ones that are possible to make concurrent
	// This will be done by looking at the variables that are assigned in the loop and seeing if they are declared outside of the loop
	// If they are, then the loop cannot be made concurrent
	// If they are not, then the loop can be made concurrent
	safeLoops := util.FindSafeLoopsForRefactoring(forLoops, fset)

	// filter safeLoops using the data from the profiling
	if dataFromProfileSorting != nil {
		safeLoops = util.FilterLoopsUsingProfileData(safeLoops, dataFromProfileSorting, fset)
	}

	info := getTypeCheckerInfo(astFile, fset)

	checker := types.Checker{
		Info: info,
	}

	for _, loop := range safeLoops {
		util.MakeLoopConcurrent(astFile, fset, loop, checker)
	}
	writeModifiedAST(fset, astFile)
}

func writeModifiedAST(fset *token.FileSet, astFile *ast.File) {
	// write the modified astFile to a new file
	file, err := os.Create("_tmp/temp.go")
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

func getTypeCheckerInfo(astFile *ast.File, fset *token.FileSet) *types.Info {
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
		println(err)
		os.Exit(0)
	}
	return info
}

// type to contain both a reference to a loop and the cumulative time of the loop
// Use this in order to be able to sort the array
