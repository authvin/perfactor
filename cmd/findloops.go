package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"go/parser"
	"go/token"
	"go/types"
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
		fmt.Println("Error parsing AST from file: " + err.Error())
		return
	}
	forLoops := util.FindForLoopsInAST(astFile, fset, nil)
	prof := util.GetProfileDataFromFile(ProfileSource)
	dataFromProfileSorting := util.SortLoopsUsingProfileData(prof, forLoops, fset)

	// Look through the for loops and range loops and find the ones that are possible to make concurrent
	// This will be done by looking at the variables that are assigned in the loop and seeing if they are declared outside of the loop
	// If they are, then the loop cannot be made concurrent
	// If they are not, then the loop can be made concurrent
	safeLoops := util.FindSafeLoopsForRefactoring(forLoops, fset, nil, Source)

	// filter safeLoops using the data from the profiling
	if dataFromProfileSorting != nil {
		loops := util.FilterLoopsUsingProfileData(safeLoops, dataFromProfileSorting, fset)
		safeLoops = make([]token.Pos, len(loops))
		for i, loop := range loops {
			safeLoops[i] = loop.Loop.Pos()
		}
	}

	info := util.GetTypeCheckerInfo(astFile, fset)

	checker := types.Checker{
		Info: info,
	}

	for _, loop := range safeLoops {
		util.MakeLoopConcurrent(astFile, fset, loop, checker)
	}
	util.WriteModifiedAST(fset, astFile, "_tmp/temp.go")
}
