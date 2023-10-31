package cmd

import (
	"bytes"
	"fmt"
	"github.com/owenrumney/go-sarif/sarif"
	"github.com/plus3it/gorecurcopy"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"io"
	"os"
	"perfactor/cmd/util"
	"strings"
	"time"
)

type WithData struct {
	originalRuntime int64
	bestDuration    int64
	astFile         *ast.File
	loopsToRefactor util.LoopInfoArray
	fileSet         *token.FileSet
	pkgs            []*packages.Package
	out             io.Writer
	tmpPath         string
	sarifRun        *sarif.Run
}

func (f WithData) GetWorkingDirPath() string {
	return f.tmpPath
}

func (f WithData) WriteSarifFile(pf ProgramSettings) {
	// in order to write the sarif file, we use the sarifRun object and write it to a file
	report, err := sarif.New(sarif.Version210)
	if err != nil {
		println("Error creating SARIF report: " + err.Error())
		return
	}
	report.AddRun(f.sarifRun)
	buffer := bytes.NewBufferString("")
	err = report.Write(buffer)
	if err != nil {
		println("Error writing SARIF report: " + err.Error())
		return
	}
	// write the buffer to a serif file
	create, err := os.Create(pf.Id + ".sarif")
	if err != nil {
		println("Error creating SARIF file: " + err.Error())
		return
	}
	defer create.Close()
	_, err = create.Write(buffer.Bytes())
	if err != nil {
		println("Error writing SARIF file: " + err.Error())
		return
	}
	println("SARIF file written")
}
func (f WithData) SetupSarif() RefactoringMode {
	f.sarifRun = sarif.NewRun("perfactor_w", "uri_placeholder")
	return f
}

func (f WithData) SetWorkingDirPath(pf ProgramSettings) RefactoringMode {
	// The tmp folder - underscore is go nomenclature for an ignored folder
	// it's called tmp to indicate that contents can be deleted without warning
	tmpPath := "_tmp" + p + pf.Id + p
	// clear the tmp subfolder if it exists
	util.CleanOrCreateTempFolder(tmpPath)
	// copy the project folder to the temp folder
	err := gorecurcopy.CopyDirectory(pf.ProjectPath, tmpPath)
	if err != nil {
		_, _ = fmt.Fprintf(f.out, "Error copying project folder to temp folder: %v\n", err.Error())
		return f
	}
	// run the go mod download command to get the dependencies
	err = downloadRequiredFromModule(err, tmpPath, f.out)
	if err != nil {
		_, _ = fmt.Fprintf(f.out, "Error downloading dependencies: %s\n", err.Error())
		_, _ = fmt.Fprintf(f.out, "Does the destination have a go.mod file?\n")
		return f
	}
	f.tmpPath = tmpPath
	return f
}

func (f WithData) SetWriter(out io.Writer) RefactoringMode {
	f.out = out
	return f
}

func (f WithData) LoadFiles(fileSet *token.FileSet) RefactoringMode {
	f.pkgs = parseFiles(f.tmpPath, fileSet, f.out)
	return f
}

func (f WithData) GetLoopInfoArray(fileSet *token.FileSet, pkgName string, projectPath string, pf ProgramSettings) (RefactoringMode, util.LoopInfoArray) {
	astFile, info, err := getFileFromPkgs(pkgName, pf.FileName, f.pkgs)
	if err != nil {
		println("Error parsing files: " + err.Error())
		return f, nil
	}
	f.astFile = astFile
	f.fileSet = fileSet
	acceptMap := getAcceptMap(pf.Accept, f.out)

	loops := util.FindForLoopsInAST(astFile, fileSet, nil)
	//Program analyses the given input file to find for-loops which are safe to make concurrent

	//Program runs the benchmark to generate profiling data
	result := util.RunCode(pf.Flags, pf.BenchName, "NONE", pf.Id, f.tmpPath+pf.FileName, f.tmpPath, true, pf.Count)
	if strings.Contains(result, "FAIL") {
		println("Error running benchmark")
		return f, nil
	}
	if strings.Contains(result, "no test files") {
		println("Error running benchmark: no test files found")
		return f, nil
	}
	println(result)

	// Get the profiling data from file
	prof := util.GetProfileDataFromFile(f.tmpPath + "cpu.pprof")
	if prof == nil {
		println("Error getting profiling data")
		return f, nil
	}

	f.bestDuration = prof.DurationNanos
	f.originalRuntime = prof.DurationNanos

	safeLoops := util.FindSafeLoopsForRefactoring(loops, fileSet, nil, projectPath+pf.FileName, acceptMap, info, f.out)

	//Program analyses the profiling data to find which for-loops to prioritize
	sortedLoops := util.SortLoopsUsingProfileData(prof, loops, fileSet)

	thresholdNanos := int64((float32(prof.DurationNanos) / 100) * pf.Threshold)
	f.loopsToRefactor = util.FilterLoopsUsingProfileData(safeLoops, sortedLoops, thresholdNanos)
	//Program combines the previous two to find which for-loops to prioritize, and which to ignore
	return f, f.loopsToRefactor
}

func (f WithData) RefactorLoop(loopInfo util.LoopInfo, pkgName string, pf ProgramSettings) (RefactoringMode, bool, error) {
	newFileSet := token.NewFileSet()
	var newAST *ast.File
	var newInfo *types.Info

	tmpFilePath := f.tmpPath + pf.FileName
	f.pkgs = parseFiles(f.tmpPath, newFileSet, f.out)

	newAST, newInfo, err := getFileFromPkgs(pkgName, pf.FileName, f.pkgs)
	if err != nil {
		println("Error parsing files: " + err.Error())
		return f, false, err
	}

	line := loopInfo.Loop.Line

	// Do the refactoring of the loopPos
	util.MakeLoopConcurrent(newAST, newFileSet, line, newInfo)

	// ------ run benchmarks etc

	//Program writes current state of AST to file, into a folder with a copy of the project (the tmp folder)
	util.WriteModifiedAST(newFileSet, newAST, f.tmpPath, pf.FileName)

	//Run the tests. If these pass, then it runs the benchmark
	testResult := util.RunCode(pf.Flags, "NONE", pf.TestName, pf.Id, tmpFilePath, f.tmpPath, false, 1)
	if strings.Contains(testResult, "FAIL") {
		//If any tests fail, we discard the change and go back to the start of the loop
		fmt.Printf("Test failed in %s for loop at line %v\n", pf.Id, line)
		// write old version back, so we can try the next loop
		util.WriteModifiedAST(f.fileSet, f.astFile, f.tmpPath, pf.FileName)
		return f, false, nil
	}

	//If the tests pass, we run the benchmark
	benchmarkResult := util.RunCode(pf.Flags, pf.BenchName, "NONE", pf.Id, tmpFilePath, f.tmpPath, true, pf.Count)
	if strings.Contains(benchmarkResult, "FAIL") {
		//If any tests fail, we discard the change and go back to the start of the loop
		fmt.Printf("Benchmark failed in %s for loop at line %v\n", pf.Id, line)
		// write old version back, so we can try the next loop
		util.WriteModifiedAST(f.fileSet, f.astFile, f.tmpPath, pf.FileName)
		return f, false, nil
	}

	//If the benchmark scores better than the previous result, we keep the change.
	tempProf := util.GetProfileDataFromFile(f.tmpPath + "cpu.pprof")

	// get the duration from the profile for the loop on this line

	// ---- finish up this iteration

	// DurationNanos is the total duration of the test
	// TimeNanos is the time when the test was run
	if tempProf.DurationNanos < f.bestDuration {
		fmt.Printf("Loop at line %v is now concurrent with an improvement of %s over the previous\n", line, time.Duration(f.bestDuration-tempProf.DurationNanos).String())
		// If the new benchmark is better, we keep the change
		f.bestDuration = tempProf.DurationNanos
		// update the astFile to the new copy
		f.astFile = newAST
		f.fileSet = newFileSet
		f.loopsToRefactor.AddLines(loopInfo.Loop)
		return f, true, nil
	} else {
		fmt.Printf("Loop at line %v gave a slowdown of %s over the previous\n", line, time.Duration(tempProf.DurationNanos-f.bestDuration).String())
		// since we're not keeping the change, write the old ast back to file
		util.WriteModifiedAST(f.fileSet, f.astFile, f.tmpPath, pf.FileName)
		return f, false, nil
	}
}

func (f WithData) WriteResult(pf ProgramSettings) {
	util.WriteModifiedAST(f.fileSet, f.astFile, pf.Output+p+pf.Id+p, pf.FileName)
	println("Final version written to " + pf.Output + p + pf.Id + p + pf.FileName)
	fmt.Printf("Original runtime: %s\n", time.Duration(f.originalRuntime))
	fmt.Printf("New runtime: %s\n", time.Duration(f.bestDuration))
}
