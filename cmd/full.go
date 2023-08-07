package cmd

import (
	"fmt"
	"go/token"
	"go/types"
	"golang.org/x/mod/modfile"
	"log"
	"os"
	"os/exec"
	"perfactor/cmd/util"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/plus3it/gorecurcopy"
	"github.com/spf13/cobra"
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
var accept string

const p = string(os.PathSeparator)

func init() {
	fullCmd.Flags().StringVarP(&benchName, "benchname", "b", "RunProgram", "The name of the benchmark to run")
	fullCmd.Flags().StringVarP(&name, "name", "n", "", "The id/name of the program to run")
	fullCmd.Flags().StringVarP(&projectPath, "project", "p", "", "The path to the project")
	fullCmd.Flags().StringVarP(&fileName, "filename", "f", "", "The path to the input file")
	fullCmd.Flags().StringVarP(&output, "output", "o", "_data", "The path to the output folder")
	fullCmd.Flags().StringVarP(&testName, "testname", "t", "NONE", "The name of the test to run")
	fullCmd.Flags().StringVarP(&flags, "flags", "", "", "Any flags to pass to the program")
	fullCmd.Flags().StringVarP(&accept, "accept", "a", "", "Accept an identifier in a given loop")
	rootCmd.AddCommand(fullCmd)
}

func full(cmd *cobra.Command, args []string) {
	//Run program, giving an input path, output path, name of benchmark, name of test, ID of the run, and any flags
	// (that's how we get here)
	if len(projectPath) == 0 {
		println("Please provide an input path")
		return
	}
	// Generate a UUID if no name is provided
	// mktemp
	if len(name) == 0 {
		u := uuid.New()
		name = u.String()
	}

	// The tmp folder - underscore is go nomenclature for an ignored folder
	// it's called tmp to indicate that contents can be deleted without warning
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
	if prof == nil {
		println("Error getting profiling data")
		return
	}

	//Program reads the input file and finds all for-loops
	fileSet := token.NewFileSet()
	astFile := util.GetASTFromFile(projectPath+fileName, fileSet)
	if astFile == nil {
		println("Error getting AST from file")
		return
	}
	loops := util.FindForLoopsInAST(astFile, fileSet, nil)

	// parse the accept string
	accepts := strings.Split(accept, ",")
	acceptMap := make(map[string]int, 0)
	for _, a := range accepts {
		str := strings.Split(a, ":")
		if len(str) != 2 {
			println("Invalid accept string")
			continue
		}
		line, err := strconv.Atoi(str[1])
		if err != nil {
			println("Invalid accept string")
			continue
		}
		acceptMap[str[0]] = line
	}

	//Program analyses the given input file to find for-loops which are safe to make concurrent
	safeLoops := util.FindSafeLoopsForRefactoring(loops, fileSet, nil, projPath+fileName, acceptMap)

	//Program analyses the profiling data to find which for-loops to prioritize
	sortedLoops := util.SortLoopsUsingProfileData(prof, loops, fileSet)

	//Program combines the previous two to find which for-loops to prioritize, and which to ignore
	loopsToRefactor := util.FilterLoopsUsingProfileData(safeLoops, sortedLoops, fileSet)

	// Variable to keep track of the best duration
	// for now it's a strict greater-than, but we could make it require a percentage increase
	bestDuration := prof.DurationNanos

	if len(loopsToRefactor) == 0 {
		println("No loops to refactor")
		return
	}

	// run the go mod download command to get the dependencies
	data, err := os.ReadFile(tmpPath + "go.mod")
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	modFile, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		log.Fatalf("Failed to parse file: %v", err)
	}

	for _, require := range modFile.Require {
		//fmt.Println("Found URL:", require.Mod.Path)
		err := exec.Command("go", "mod", "download", require.Mod.Path).Run()
		if err != nil {
			println("Error downloading dependencies")
			return
		}
	}

	//Program performs the refactoring of each loop
	for _, lt := range loopsToRefactor {

		// New fileset and AST, which is needed because the type checker bugs out if we don't
		newFileSet := token.NewFileSet()
		// Get the AST from the file in the tmp folder. This will either be the original file, or the most recent accepted change
		newAST := util.GetASTFromFile(tmpPath+fileName, newFileSet)

		// Get the type checker
		// We need to get a new type checker for each copy of the AST, because otherwise it doesn't know the types
		info := util.GetTypeCheckerInfo(newAST, newFileSet)
		checker := types.Checker{
			Info: info,
		}

		loopPos := lt.Loop.Pos

		// Do the refactoring of the loopPos
		util.MakeLoopConcurrent(newAST, newFileSet, loopPos, checker)

		//Program writes current state of AST to file, into a folder with a copy of the project (the tmp folder)
		util.WriteModifiedAST(newFileSet, newAST, tmpPath+fileName)

		//Run the tests. If these pass, then it runs the benchmark
		testResult := util.RunCode(flags, "NONE", testName, name, tmpPath+fileName, tmpPath, false)
		if strings.Contains(testResult, "FAIL") {
			//If any tests fail, we discard the change and go back to the start of the loop
			println("Test failed in " + name + " for loop at line " + string(rune(newFileSet.Position(loopPos).Line)))
			// write old version back, so we can try the next loop
			//util.WriteModifiedAST(fileSet, astFile, tmpPath+fileName)
			continue
		}

		//If the tests pass, we run the benchmark
		benchmarkResult := util.RunCode(flags, benchName, "NONE", name, tmpPath+fileName, tmpPath, true)
		if strings.Contains(benchmarkResult, "FAIL") {
			//If any tests fail, we discard the change and go back to the start of the loop
			println("Benchmark failed in " + name + " for loop at line " + string(rune(newFileSet.Position(loopPos).Line)))
			// write old version back, so we can try the next loop
			util.WriteModifiedAST(fileSet, astFile, tmpPath+fileName)
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
			fileSet = newFileSet
		} else {
			// since we're not keeping the change, write the old ast back to file
			util.WriteModifiedAST(fileSet, astFile, tmpPath+fileName)
		}
	}
	// create output folder
	err = os.MkdirAll(output+p+name+p, os.ModePerm)
	if err != nil {
		println("Error creating output folder: " + err.Error())
		return
	}
	// write the finished program to output
	util.WriteModifiedAST(fileSet, astFile, output+p+name+p+fileName)
	println("Final version written to " + output + p + name + p + fileName)
	fmt.Printf("Original runtime: %s\n", time.Duration(prof.DurationNanos))
	fmt.Printf("New runtime: %s\n", time.Duration(bestDuration))
}
