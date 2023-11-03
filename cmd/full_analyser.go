package cmd

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"perfactor/cmd/util"
	"perfactor/concurrentcheck"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/plus3it/gorecurcopy"
	"github.com/spf13/cobra"
	"golang.org/x/tools/go/analysis"
)

var fullACmd = &cobra.Command{
	Use:     "fullA",
	Short:   "Run the full program",
	Aliases: []string{"fa"},
	Run:     fullA,
}

var analyzer string

func init() {
	fullACmd.Flags().StringP("benchname", "b", "RunProgram", "The Id of the benchmark to run")
	fullACmd.Flags().StringP("Id", "n", "", "The Id/Id of the program to run")
	fullACmd.Flags().StringP("project", "p", "", "The path to the project")
	fullACmd.Flags().StringP("filename", "f", "", "The path to the input file")
	fullACmd.Flags().StringP("Output", "o", "_data", "The path to the Output folder")
	fullACmd.Flags().StringP("testname", "t", "NONE", "The Id of the test to run")
	fullACmd.Flags().StringP("Flags", "", "", "Any Flags to pass to the program")
	fullACmd.Flags().StringVarP(&analyzer, "analyzer", "a", "loop_concurrent", "The analyzer to run")
	fullACmd.Flags().StringP("Accept", "e", "", "Accept an identifier in a given loop")
	fullACmd.Flags().IntP("Count", "c", 3, "The number of times to run the benchmark")
	fullACmd.Flags().Float32P("Threshold", "d", 0.1, "The Threshold for the percentage increase in runtime")
	RootCmd.AddCommand(fullACmd)
}

func fullA(cmd *cobra.Command, args []string) {
	//Run program, giving an input path, Output path, Id of benchmark, Id of test, ID of the run, and any Flags
	// (that's how we get here)
	ProjectPath, err := cmd.Flags().GetString("project")
	if err != nil {
		println("Error getting project path: " + err.Error())
		return
	}
	if len(ProjectPath) == 0 {
		println("Please provide a project path")
		return
	}
	// make sure the project path ends with a slash
	if !strings.HasSuffix(ProjectPath, p) {
		ProjectPath += p
	}

	// Generate a UUID if no Id is provided
	Id, err := cmd.Flags().GetString("Id")
	if err != nil {
		println("Error getting Id: " + err.Error())
		return
	}
	if len(Id) == 0 {
		u := uuid.New()
		Id = u.String()
	}

	a := concurrentcheck.Analyzer
	pf, err := programSettings(cmd)
	if err != nil {
		fmt.Printf("Error getting Flags: %s\n", err.Error())
		return
	}
	RunAnalyser(a, ProjectPath, Id, pf)
}

func RunAnalyser(a *analysis.Analyzer, ProjectPath, Id string, pf ProgramSettings) {
	// check if the project path ends with a slash
	if pf.ProjectPath[len(pf.ProjectPath)-1] != os.PathSeparator {
		// if not, add one
		pf.ProjectPath += string(os.PathSeparator)
	}

	// Generate a UUID if no Id is provided
	// mktemp
	if len(Id) == 0 {
		u := uuid.New()
		Id = u.String()
	}

	fileSet := token.NewFileSet()
	astFile := util.GetASTFromFile(ProjectPath+pf.FileName, fileSet)
	if astFile == nil {
		println("Error getting AST from file")
		return
	}

	// see if the analyzer has prerequisites
	// if so, run them
	resultsOf := make(map[*analysis.Analyzer]interface{})
	if len(a.Requires) > 0 {
		for _, req := range a.Requires {
			fmt.Printf("Running prerequisite: %s\n", req.Name)
			pass := &analysis.Pass{
				Analyzer: req,
				Fset:     fileSet,
				Files:    []*ast.File{astFile},
				Report:   func(d analysis.Diagnostic) {},
				ResultOf: resultsOf,
			}
			result, err := req.Run(pass)
			if err != nil {
				println("Error running prerequisite: " + err.Error())
				return
			}
			fmt.Printf("Adding result of prerequisite: %s\n", req.Name)
			resultsOf[req] = result
		}
	}

	// The tmp folder - underscore is go nomenclature for an ignored folder
	// it's called tmp to indicate that contents can be deleted without warning
	tmpPath := "_tmp" + p + Id + p
	// clear the tmp subfolder if it exists
	util.CleanOrCreateTempFolder(tmpPath)

	// copy the project folder to the temp folder
	err := gorecurcopy.CopyDirectory(ProjectPath, tmpPath)
	if err != nil {
		println("Error copying project folder to temp folder: " + err.Error())
		return
	}
	fmt.Printf("Running initial benchmark: %s\n", pf.BenchName)
	//Program runs the benchmark to generate profiling data
	result := util.RunCode(pf.Flags, pf.BenchName, "NONE", Id, tmpPath+pf.FileName, tmpPath, true, pf.Count)
	if result == "FAILED" {
		println("Error running benchmark")
		return
	}

	// Get the profiling data from file
	prof := util.GetProfileDataFromFile(tmpPath + "cpu.pprof")

	diagnostics := make([]analysis.Diagnostic, 0)
	// create the analysis pass
	pass := &analysis.Pass{
		Analyzer: a,
		Fset:     fileSet,
		Files:    []*ast.File{astFile},
		Report:   func(d analysis.Diagnostic) { diagnostics = append(diagnostics, d) },
		ResultOf: resultsOf,
		// could add type info here, probably should, but need to rework our util method to handle multiple files for that
	}

	// Run the analyzer
	fmt.Printf("Running analyzer: %s\n", a.Name)
	_, err = a.Run(pass)
	if err != nil {
		println("Error running analyzer: " + err.Error())
		return
	}
	fmt.Printf("Analyzer finished: %s\n", a.Name)

	// Variable to keep track of the best duration
	// for now it's a strict greater-than, but we could make it require a percentage increase
	originalDuration := prof.DurationNanos

	// if we work based off of the positions, that changes from run to run. So we need to benchmark each change on its own, then combine them all at the end

	// map to track if fixes pass the tests, and if they give improvement
	resultMap := make(map[*analysis.Diagnostic]runResult)

	//Program performs the refactoring of each loop
	for _, diag := range diagnostics {
		suggestedFixes := diag.SuggestedFixes
		if len(suggestedFixes) == 0 {
			continue
		}

		// list of text edits
		changes := make(fixPositionList, 0)

		changes.fillListFromDiag(&diag)

		// sort fixes and check for overlaps

		sort.Sort(changes)
		changes.removeOverlaps()
		oldFile, err := os.ReadFile(ProjectPath + pf.FileName)
		if err != nil {
			println("Error reading file: " + err.Error())
			return
		}
		curIndex := 0

		// apply import changes
		var importpos int
		ast.Inspect(astFile, func(n ast.Node) bool {
			if p, ok := n.(*ast.ImportSpec); ok {
				importpos = fileSet.Position(p.End()).Offset
				return false
			}
			return true
		})

		// buffer to hold the new file
		var buf bytes.Buffer
		buf = writeChangesToBuffer(changes, fileSet, importpos, curIndex, buf, oldFile)

		// write the buffer to file
		err = os.WriteFile(tmpPath+p+pf.FileName, buf.Bytes(), 0644)
		if err != nil {
			println("Error writing file: " + err.Error())
			return
		}

		res := runResult{
			passed:   false,
			improved: false,
		}

		if runTestAndBenchmark(tmpPath, pf.Flags, pf.TestName, pf.BenchName, Id, pf.FileName, pf.Count) {
			fmt.Printf("Fix passed: %s\n", diag.Message)
			res.passed = true
		} else {
			fmt.Printf("Fix failed: %s\n", diag.Message)
			continue
		}

		//If the benchmark scores better than the previous result, we keep the change.
		tempProf := util.GetProfileDataFromFile(tmpPath + "cpu.pprof")
		// DurationNanos is the total duration of the test
		// TimeNanos is the time when the test was run
		if tempProf.DurationNanos < originalDuration {
			// If the new benchmark is better, we keep the change
			res.improved = true
		}
		resultMap[&diag] = res
	}
	if len(resultMap) == 0 {
		println("No fixes found")
		return
	}
	// if no fixes passed, we're done
	anyPassed := false
	for _, result := range resultMap {
		if result.passed {
			anyPassed = true
			break
		}
	}
	if !anyPassed {
		println("No fixes passed")
		return
	}

	// create Output folder
	err = os.MkdirAll(pf.Output+p+Id+p, os.ModePerm)
	if err != nil {
		println("Error creating Output folder: " + err.Error())
		return
	}
	fixesToApply := make(fixPositionList, 0)
	// now we apply all the successful changes at once, then test and benchmark, and write a report
	for diag, result := range resultMap {
		if result.passed && result.improved {
			fixesToApply.fillListFromDiag(diag)
		}
	}
	// sort fixes and check for overlaps
	// TODO: These need to report what they do
	sort.Sort(fixesToApply)
	fixesToApply.removeOverlaps()

	oldFile, err := os.ReadFile(ProjectPath + pf.FileName)
	if err != nil {
		println("Error reading file: " + err.Error())
		return
	}
	curIndex := 0

	// buffer to hold the new file
	var buf bytes.Buffer

	var importpos int
	ast.Inspect(astFile, func(n ast.Node) bool {
		if p, ok := n.(*ast.ImportSpec); ok {
			importpos = fileSet.Position(p.End()).Offset
			return false
		}
		return true
	})

	buf = writeChangesToBuffer(fixesToApply, fileSet, importpos, curIndex, buf, oldFile)

	// write the buffer to file
	err = os.WriteFile(tmpPath+pf.FileName, buf.Bytes(), 0644)

	if err != nil {
		println("Error writing file: " + err.Error())
		return
	}

	res := runResult{
		passed:   false,
		improved: false,
	}

	if runTestAndBenchmark(tmpPath, pf.Flags, pf.TestName, pf.BenchName, Id, pf.FileName, pf.Count) {
		res.passed = true
	} else {
		return
	}

	// both test and benchmark are successful
	res.passed = true

	//If the benchmark scores better than the previous result, we keep the change.
	tempProf := util.GetProfileDataFromFile(tmpPath + "cpu.pprof")
	// DurationNanos is the total duration of the test
	// TimeNanos is the time when the test was run
	if tempProf.DurationNanos < originalDuration {
		// If the new benchmark is better, we keep the change
		res.improved = true
		fmt.Println("Benchmark improved")
	} else {
		fmt.Println("Benchmark did not improve")
	}
}

func runTestAndBenchmark(tmpPath, Flags, TestName, BenchName, Id, FileName string, Count int) bool {
	testResult := util.RunCode(Flags, "NONE", TestName, Id, tmpPath+FileName, tmpPath, false, Count)
	if strings.Contains(testResult, "FAIL") {
		fmt.Println("Test failed")
		return false
	}

	benchmarkResult := util.RunCode(Flags, BenchName, "NONE", Id, tmpPath+FileName, tmpPath, true, Count)
	if strings.Contains(benchmarkResult, "FAIL") {
		fmt.Println("Benchmark failed")
		return false
	}
	return true
}

func writeChangesToBuffer(changes fixPositionList, fileSet *token.FileSet, importpos int, curIndex int, buf bytes.Buffer, oldFile []byte) bytes.Buffer {
	for _, textEdit := range changes {
		start := fileSet.Position(textEdit.start).Offset
		if textEdit.start == token.NoPos {

			start = importpos
		}
		end := fileSet.Position(textEdit.end).Offset
		if textEdit.end == token.NoPos {
			end = start
		}
		if curIndex < start {
			buf.Write(oldFile[curIndex:start])
		}

		buf.Write(textEdit.textEdit)
		curIndex = end
	}

	if curIndex < len(oldFile) {
		buf.Write(oldFile[curIndex:])
	}
	return buf
}

func (f fixPositionList) fillListFromDiag(diag *analysis.Diagnostic) {
	RefactoringMode := 0
	for _, fix := range diag.SuggestedFixes {
		for _, change := range fix.TextEdits {
			f = append(f, fixPosition{
				start:        change.Pos,
				end:          change.End,
				suggestedFix: &fix,
				textEdit:     change.NewText,
				Id:           RefactoringMode,
			})
			RefactoringMode++
		}
	}
}

type runResult struct {
	passed   bool
	improved bool
}

type fixPosition struct {
	start        token.Pos
	end          token.Pos
	suggestedFix *analysis.SuggestedFix
	textEdit     []byte
	Id           int
}

func (f fixPosition) startsBefore(o fixPosition) bool {
	return f.start < o.start
}

func (f fixPosition) overlaps(o fixPosition) bool {
	// pagewide edits do not overlap - handled separately
	if f.start == token.NoPos || o.start == token.NoPos {
		return false
	}
	// if start is between start and end, or end is between start and end, then they overlap
	if f.start >= o.start && f.start <= o.end {
		return true
	}
	if f.end >= o.start && f.end <= o.end {
		return true
	}
	return false
}

type fixPositionList []fixPosition

func (l fixPositionList) Len() int {
	return len(l)
}

func (l fixPositionList) Less(RefactoringMode, j int) bool {
	// we want to sort greatest first
	return l[RefactoringMode].startsBefore(l[j])
}

func (l fixPositionList) Swap(RefactoringMode, j int) {
	l[RefactoringMode], l[j] = l[j], l[RefactoringMode]
}

func (l fixPositionList) removeOverlaps() {
	Id := l.removeOverlapHelper(0)
	for Id != -1 {
		Id = l.removeOverlapHelper(Id)
	}
}

func (l fixPositionList) removeOverlapHelper(Id int) int {
	index := 0
	for RefactoringMode := 0; RefactoringMode < len(l); RefactoringMode++ {
		if Id == 0 {
			break
		}
		if l[RefactoringMode].Id == Id {
			index = RefactoringMode
		}
	}
	// continue with where we left off previously with the index
	for RefactoringMode := index; RefactoringMode < len(l); RefactoringMode++ {
		for j := RefactoringMode + 1; j < len(l); j++ {
			if l[RefactoringMode].overlaps(l[j]) {
				Id := l[RefactoringMode].Id
				l.removeSuggestedFix(l[RefactoringMode].suggestedFix)
				return Id
			}
		}
	}
	return -1
}

func (l fixPositionList) removeSuggestedFix(s *analysis.SuggestedFix) {
	for RefactoringMode := 0; RefactoringMode < len(l); RefactoringMode++ {
		if l[RefactoringMode].suggestedFix == s {
			l = append(l[:RefactoringMode], l[RefactoringMode+1:]...)
			RefactoringMode--
		}
	}
}
