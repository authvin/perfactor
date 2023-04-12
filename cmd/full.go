package cmd

import (
	"github.com/google/uuid"
	"github.com/plus3it/gorecurcopy"
	"github.com/spf13/cobra"
	"go/token"
	"go/types"
	"os"
	"perfactor/cmd/util"
	"strconv"
	"strings"
)

var fullCmd = &cobra.Command{
	Use:     "full",
	Short:   "Run the full program",
	Aliases: []string{"f"},
	Run:     full,
}

var benchName string
var name string
var projectPath string
var fileName string
var output string
var testName string
var flags string

const p = string(os.PathSeparator)

func init() {
	fullCmd.Flags().StringVarP(&benchName, "benchname", "b", "RunProgram", "The name of the benchmark to run")
	fullCmd.Flags().StringVarP(&name, "name", "n", "", "The id/name of the program to run")
	fullCmd.Flags().StringVarP(&projectPath, "project", "p", "", "The path to the project")
	fullCmd.Flags().StringVarP(&fileName, "filename", "f", "", "The path to the input file")
	fullCmd.Flags().StringVarP(&output, "output", "o", "_data", "The path to the output folder")
	fullCmd.Flags().StringVarP(&testName, "testname", "t", "NONE", "The name of the test to run")
	fullCmd.Flags().StringVarP(&flags, "flags", "", "", "Any flags to pass to the program")
	rootCmd.AddCommand(fullCmd)
}

func full(cmd *cobra.Command, args []string) {
	//Run program, giving an input path, output path, name of benchmark, name of test, ID of the run, and any flags
	// (that's how we get here)
	if len(projectPath) == 0 {
		println("Please provide an input path")
		return
	}
	if len(name) == 0 {
		u := uuid.New()
		name = u.String()
	}

	tmpPath := "_tmp" + p + name + p
	// clear the tmp subfolder if it exists
	util.CleanOrCreateTempFolder(tmpPath)
	// copy the project folder to the temp folder
	err := gorecurcopy.CopyDirectory(projectPath, tmpPath)
	if err != nil {
		println("Error copying project folder to temp folder: " + err.Error())
		return
	}

	//Program runs the benchmark to generate profiling data
	result := util.RunCode(flags, benchName, "NONE", name, tmpPath+fileName, tmpPath, true)
	if result == "FAILED" {
		println("Error running benchmark")
		return
	}

	// Get the profiling data from file
	prof := util.GetProfileDataFromFile(tmpPath + "cpu.pprof")

	fset := token.NewFileSet()
	astFile := util.GetASTFromFile(projectPath+fileName, fset)
	//originalAstFile := astcopy.File(astFile) // In case we need the original file at some point?
	if astFile == nil {
		println("Error getting AST from file")
		return
	}
	forLoops := util.FindForLoopsInAST(astFile)

	//Program analyses the given input file to find for-loops which are safe to make concurrent
	safeLoops := util.FindSafeLoopsForRefactoring(forLoops, fset)

	sortedLoops := util.SortLoopsUsingProfileData(prof, forLoops, fset)

	//Program analyses this profiling data to find which for-loops to prioritize, and which to ignore
	loopsToRefactor := util.FilterLoopsUsingProfileData(safeLoops, sortedLoops, fset)

	bestDuration := prof.DurationNanos

	//Program performs the refactoring of one loop
	for _, lt := range loopsToRefactor {
		newFileSet := token.NewFileSet()
		loopPos := lt.Loop.Pos()
		// make a deep copy of the ast file, so that we can discard it if the tests fail or it doesn't provide a benefit
		newAST := util.GetASTFromFile(tmpPath+fileName, newFileSet)
		// Get the type checker
		// We need to get a new type checker for each copy of the AST, because otherwise it doesn't know the types
		info := util.GetTypeCheckerInfo(newAST, newFileSet)
		checker := types.Checker{
			Info: info,
		}
		// Do the refactoring of the loopPos
		util.MakeLoopConcurrent(newAST, newFileSet, loopPos, checker)

		//Program writes current state to file, into a folder with a copy of the project
		util.WriteModifiedAST(newFileSet, newAST, tmpPath+fileName)

		//Run the tests. If these pass, then it runs the benchmark
		testResult := util.RunCode(flags, "NONE", testName, name, tmpPath+fileName, tmpPath, false)
		if strings.Contains(testResult, "FAIL") {
			//If any tests fail, we discard the change and go back to step 5
			println("Test failed in " + name + " for loop at line " + string(rune(newFileSet.Position(loopPos).Line)))
			continue
		}
		//If the tests pass, we run the benchmark
		benchmarkResult := util.RunCode(flags, benchName, "NONE", name, tmpPath+fileName, tmpPath, true)
		if strings.Contains(benchmarkResult, "FAIL") {
			//If any tests fail, we discard the change and go back to step 5
			println("Benchmark failed in " + name + " for loop at line " + string(rune(newFileSet.Position(loopPos).Line)))
			continue
		}
		//If the benchmark scores better than the previous result, we keep the change.
		tempProf := util.GetProfileDataFromFile(tmpPath + "cpu.pprof")
		// DurationNanos is the total duration of the test
		// TimeNanos is the time when the test was run
		if tempProf.DurationNanos < bestDuration {
			// If the new benchmark is better, we keep the change
			bestDuration = tempProf.DurationNanos
			// update the astFile to the new copy
			astFile = newAST
			fset = newFileSet
		} else {
			// since we're not keeping the change, write the old ast back to file
			util.WriteModifiedAST(fset, astFile, tmpPath+fileName)
		}
		//Go back to step 5 until there are no more loops left that we want to refactor
	}
	// create output folder
	err = os.MkdirAll(output+name+p, os.ModePerm)
	if err != nil {
		println("Error creating output folder: " + err.Error())
		return
	}
	// write the finished program to output
	util.WriteModifiedAST(fset, astFile, output+name+p+fileName)
	println("Final version written to " + output + name + p + fileName)
	println("Original runtime: " + strconv.FormatInt(prof.DurationNanos, 10))
	println("New runtime: " + strconv.FormatInt(bestDuration, 10))
}
