package cmd

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
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

const p = string(os.PathSeparator)

func init() {
	fullCmd.Flags().StringP("benchname", "b", "RunProgram", "The name of the benchmark to run")
	fullCmd.Flags().StringP("name", "n", "", "The id/name of the program to run")
	fullCmd.Flags().StringP("project", "p", "", "The path to the project")
	fullCmd.Flags().StringP("filename", "f", "", "The path to the input file")
	fullCmd.Flags().StringP("output", "o", "_data", "The path to the output folder")
	fullCmd.Flags().StringP("testname", "t", "NONE", "The name of the test to run")
	fullCmd.Flags().StringP("flags", "", "", "Any flags to pass to the program")
	fullCmd.Flags().StringP("accept", "a", "", "Accept an identifier in a given loop")
	fullCmd.Flags().IntP("count", "c", 3, "The number of times to run the benchmark")
	fullCmd.Flags().Float32P("threshold", "d", 10.0, "The threshold for the percentage increase in runtime")
	rootCmd.AddCommand(fullCmd)
}

func full(cmd *cobra.Command, args []string) {
	//Run program, giving an input path, output path, name of benchmark, name of test, ID of the run, and any flags
	// (that's how we get here)
	projectPath, err := cmd.Flags().GetString("project")
	if err != nil {
		println("Error getting project path: " + err.Error())
		return
	}
	if len(projectPath) == 0 {
		println("Please provide a project path")
		return
	}
	// make sure the project path ends with a slash
	if !strings.HasSuffix(projectPath, p) {
		projectPath += p
	}

	// Generate a UUID if no name is provided
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		println("Error getting name: " + err.Error())
		return
	}
	if len(name) == 0 {
		u := uuid.New()
		name = u.String()
	}

	flags, benchName, testName, fileName, accept, output, count, threshold := getFlags(cmd)

	// The tmp folder - underscore is go nomenclature for an ignored folder
	// it's called tmp to indicate that contents can be deleted without warning
	tmpPath := "_tmp" + p + name + p
	tmpFilePath := tmpPath + fileName
	// clear the tmp subfolder if it exists
	util.CleanOrCreateTempFolder(tmpPath)

	// copy the project folder to the temp folder
	err = gorecurcopy.CopyDirectory(projectPath, tmpPath)
	if err != nil {
		println("Error copying project folder to temp folder: " + err.Error())
		return
	}

	//Program runs the benchmark to generate profiling data
	result := util.RunCode(flags, benchName, "NONE", name, tmpFilePath, tmpPath, true, count)
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
	acceptMap := getAcceptMap(accept)

	pkgName := util.GetPackageNameFromPath(tmpFilePath)
	fileSet := token.NewFileSet()

	// run the go mod download command to get the dependencies
	err = downloadRequiredFromModule(err, tmpPath)
	if err != nil {
		fmt.Printf("Error downloading dependencies: %s\n", err.Error())
		return
	}

	//Program reads the input file and finds all for-loops

	astFile, info, err := parseFiles(tmpPath, fileSet, pkgName, fileName)
	if err != nil {
		println("Error parsing files: " + err.Error())
		return
	}

	loops := util.FindForLoopsInAST(astFile, fileSet, nil)
	//Program analyses the given input file to find for-loops which are safe to make concurrent
	safeLoops := util.FindSafeLoopsForRefactoring(loops, fileSet, nil, projectPath+fileName, acceptMap, info)

	//Program analyses the profiling data to find which for-loops to prioritize
	sortedLoops := util.SortLoopsUsingProfileData(prof, loops, fileSet)

	thresholdNanos := int64((float32(prof.DurationNanos) / 100) * threshold)

	//Program combines the previous two to find which for-loops to prioritize, and which to ignore
	loopsToRefactor := util.FilterLoopsUsingProfileData(safeLoops, sortedLoops, thresholdNanos)
	println("Found " + strconv.Itoa(len(loopsToRefactor)) + " loops to refactor")
	// Variable to keep track of the best duration
	// for now it's a strict greater-than, but we could make it require a percentage increase
	bestDuration := prof.DurationNanos

	if len(loopsToRefactor) == 0 {
		println("No loops to refactor")
		return
	}

	hasModified := false

	//Program performs the refactoring of each loop
	for _, loopInfo := range loopsToRefactor {

		// New fileset and AST, which is needed because the type checker bugs out if we don't
		newFileSet := token.NewFileSet()
		var newAST *ast.File
		var newInfo *types.Info

		newAST, newInfo, err = parseFiles(tmpPath, newFileSet, pkgName, fileName)
		if err != nil {
			println("Error parsing files: " + err.Error())
			return
		}

		line := loopInfo.Loop.Line

		// Do the refactoring of the loopPos
		util.MakeLoopConcurrent(newAST, newFileSet, line, newInfo)

		//Program writes current state of AST to file, into a folder with a copy of the project (the tmp folder)
		util.WriteModifiedAST(newFileSet, newAST, tmpFilePath)

		//Run the tests. If these pass, then it runs the benchmark
		testResult := util.RunCode(flags, "NONE", testName, name, tmpFilePath, tmpPath, false, 1)
		if strings.Contains(testResult, "FAIL") {
			//If any tests fail, we discard the change and go back to the start of the loop
			fmt.Printf("Test failed in %s for loop at line %v\n", name, line)
			// write old version back, so we can try the next loop
			util.WriteModifiedAST(fileSet, astFile, tmpFilePath)
			continue
		}

		//If the tests pass, we run the benchmark
		benchmarkResult := util.RunCode(flags, benchName, "NONE", name, tmpFilePath, tmpPath, true, count)
		if strings.Contains(benchmarkResult, "FAIL") {
			//If any tests fail, we discard the change and go back to the start of the loop
			fmt.Printf("Benchmark failed in %s for loop at line %v\n", name, line)
			// write old version back, so we can try the next loop
			util.WriteModifiedAST(fileSet, astFile, tmpFilePath)
			continue
		}

		//If the benchmark scores better than the previous result, we keep the change.
		tempProf := util.GetProfileDataFromFile(tmpPath + "cpu.pprof")

		// get the duration from the profile for the loop on this line

		// DurationNanos is the total duration of the test
		// TimeNanos is the time when the test was run
		if tempProf.DurationNanos < bestDuration {
			fmt.Printf("Loop at line %v is now concurrent with an improvement of %s over the previous\n", line, time.Duration(bestDuration-tempProf.DurationNanos).String())
			// If the new benchmark is better, we keep the change
			bestDuration = tempProf.DurationNanos
			// update the astFile to the new copy
			astFile = newAST
			fileSet = newFileSet
			loopsToRefactor.AddLines(loopInfo.Loop)
			hasModified = true
		} else {
			fmt.Printf("Loop at line %v gave a slowdown of %s over the previous\n", line, time.Duration(tempProf.DurationNanos-bestDuration).String())
			// since we're not keeping the change, write the old ast back to file
			util.WriteModifiedAST(fileSet, astFile, tmpFilePath)
		}
	}

	if !hasModified {
		println("No loops were refactored")
		return
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

func getFlags(cmd *cobra.Command) (flags string, benchName string, testName string, fileName string, accept string, output string, count int, threshold float32) {
	flags, err := cmd.Flags().GetString("flags")
	if err != nil {
		println("Error getting flags: " + err.Error())
		return
	}
	benchName, err = cmd.Flags().GetString("benchname")
	if err != nil {
		println("Error getting benchmark name: " + err.Error())
		return
	}
	testName, err = cmd.Flags().GetString("testname")
	if err != nil {
		println("Error getting test name: " + err.Error())
		return
	}
	fileName, err = cmd.Flags().GetString("filename")
	if err != nil {
		println("Error getting filename: " + err.Error())
		return
	}
	output, err = cmd.Flags().GetString("output")
	if err != nil {
		println("Error getting output path: " + err.Error())
		return
	}
	count, err = cmd.Flags().GetInt("count")
	if err != nil {
		println("Error getting count: " + err.Error())
		return
	}
	threshold, err = cmd.Flags().GetFloat32("threshold")
	if err != nil {
		println("Error getting threshold: " + err.Error())
		return
	}
	accept, err = cmd.Flags().GetString("accept")
	if err != nil {
		println("Error getting accept: " + err.Error())
		return
	}
	return
}

func getAcceptMap(accept string) map[string]int {
	acceptMap := make(map[string]int, 0)
	// parse the accept string
	if len(accept) > 0 {
		accepts := strings.Split(accept, ",")
		for _, a := range accepts {
			str := strings.Split(a, ":")
			if len(str) != 2 {
				println("Invalid accept string")
				continue
			}
			line, err := strconv.Atoi(str[1])
			if err != nil {
				println("Invalid accept string: ", err.Error())
				continue
			}
			acceptMap[str[0]] = line
		}
	}
	return acceptMap
}

func downloadRequiredFromModule(err error, tmpPath string) error {
	data, err := os.ReadFile(tmpPath + "go.mod")
	if err != nil {
		return err
	}

	modFile, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return err
	}

	for _, require := range modFile.Require {
		//fmt.Println("Found URL:", require.Mod.Path)
		cmd := exec.Command("go", "get", require.Mod.Path)
		out, err := cmd.Output()
		if err != nil {
			println(string(out))
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr
			}
			return err
		}
	}
	return nil
}

func parseFiles(tmpPath string, fileSet *token.FileSet, pkgName string, fileName string) (*ast.File, *types.Info, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedCompiledGoFiles | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  tmpPath,
		Fset: fileSet,
	}
	pkgs, err := packages.Load(cfg, "")

	if err != nil {
		return nil, nil, err
	}

	// get the target ast file from the packages
	var astFile *ast.File
	var info *types.Info
	for _, p := range pkgs {
		if p.Name == pkgName {
			for i, f := range p.CompiledGoFiles {
				if strings.HasSuffix(f, fileName) {
					astFile = p.Syntax[i]
				}
			}
			info = p.TypesInfo
		}
	}

	if astFile == nil {
		return nil, nil, errors.New("Error getting AST from file")
	}
	return astFile, info, nil
}
