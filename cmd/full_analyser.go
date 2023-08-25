package cmd

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"perfactor/cmd/util"
	"perfactor/concurrentcheck"
	"regexp"
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
	fullACmd.Flags().StringP("benchname", "b", "RunProgram", "The name of the benchmark to run")
	fullACmd.Flags().StringP("name", "n", "", "The id/name of the program to run")
	fullACmd.Flags().StringP("project", "p", "", "The path to the project")
	fullACmd.Flags().StringP("filename", "f", "", "The path to the input file")
	fullACmd.Flags().StringP("output", "o", "_data", "The path to the output folder")
	fullACmd.Flags().StringP("testname", "t", "NONE", "The name of the test to run")
	fullACmd.Flags().StringP("flags", "", "", "Any flags to pass to the program")
	fullACmd.Flags().StringVarP(&analyzer, "analyzer", "a", "loop_concurrent", "The analyzer to run")
	fullACmd.Flags().StringP("accept", "e", "", "Accept an identifier in a given loop")
	fullACmd.Flags().IntP("count", "c", 3, "The number of times to run the benchmark")
	fullACmd.Flags().Float32P("threshold", "d", 0.1, "The threshold for the percentage increase in runtime")
	rootCmd.AddCommand(fullACmd)
}

func fullA(cmd *cobra.Command, args []string) {
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

	var a *analysis.Analyzer
	// run the appropriate analyzer
	switch analyzer {
	case "loop_concurrent":
		a = concurrentcheck.Analyzer
	default:
		fmt.Printf("Unknown analyzer: %s\n", analyzer)
		return
	}
	flags, benchName, testName, fileName, accept, output, count, threshold := getFlags(cmd)
	RunAnalyser(a, projectPath, fileName, name, benchName, testName, flags, count, output, accept, threshold)
}

func RunAnalyser(a *analysis.Analyzer, projectPath, fileName, name, benchName, testName, flags string, count int, output string, accept string, threshold float32) {
	// check if the project path ends with a slash
	if projectPath[len(projectPath)-1] != os.PathSeparator {
		// if not, add one
		projectPath += string(os.PathSeparator)
	}

	// Generate a UUID if no name is provided
	// mktemp
	if len(name) == 0 {
		u := uuid.New()
		name = u.String()
	}

	fileSet := token.NewFileSet()
	astFile := util.GetASTFromFile(projectPath+fileName, fileSet)
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
			result, error := req.Run(pass)
			if error != nil {
				println("Error running prerequisite: " + error.Error())
				return
			}
			fmt.Printf("Adding result of prerequisite: %s\n", req.Name)
			resultsOf[req] = result
		}
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
	fmt.Printf("Running initial benchmark: %s\n", benchName)
	//Program runs the benchmark to generate profiling data
	result := util.RunCode(flags, benchName, "NONE", name, tmpPath+fileName, tmpPath, true, count)
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
		oldFile, err := os.ReadFile(projectPath + fileName)
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
		err = os.WriteFile(tmpPath+p+fileName, buf.Bytes(), 0644)
		if err != nil {
			println("Error writing file: " + err.Error())
			return
		}

		res := runResult{
			passed:   false,
			improved: false,
		}

		if runTestAndBenchmark(tmpPath, flags, testName, benchName, name, fileName, count) {
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

	// create output folder
	err = os.MkdirAll(output+p+name+p, os.ModePerm)
	if err != nil {
		println("Error creating output folder: " + err.Error())
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

	oldFile, err := os.ReadFile(projectPath + fileName)
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
	err = os.WriteFile(tmpPath+fileName, buf.Bytes(), 0644)

	if err != nil {
		println("Error writing file: " + err.Error())
		return
	}

	res := runResult{
		passed:   false,
		improved: false,
	}

	if runTestAndBenchmark(tmpPath, flags, testName, benchName, name, fileName, count) {
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

func runTestAndBenchmark(tmpPath, flags, testName, benchName, name, fileName string, count int) bool {
	testResult := util.RunCode(flags, "NONE", testName, name, tmpPath+fileName, tmpPath, false, count)
	if strings.Contains(testResult, "FAIL") {
		fmt.Println("Test failed")
		return false
	}

	benchmarkResult := util.RunCode(flags, benchName, "NONE", name, tmpPath+fileName, tmpPath, true, count)
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

func extractQuotedContent(data []byte) []byte {
	re := regexp.MustCompile(`"(.*?)"`)
	matches := re.FindAll(data, -1)
	return bytes.Join(matches, []byte("\n"))
}

func (f fixPositionList) fillListFromDiag(diag *analysis.Diagnostic) {
	i := 0
	for _, fix := range diag.SuggestedFixes {
		for _, change := range fix.TextEdits {
			f = append(f, fixPosition{
				start:        change.Pos,
				end:          change.End,
				suggestedFix: &fix,
				textEdit:     change.NewText,
				id:           i,
			})
			i++
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
	id           int
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

func (l fixPositionList) Less(i, j int) bool {
	// we want to sort greatest first
	return l[i].startsBefore(l[j])
}

func (l fixPositionList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l fixPositionList) removeOverlaps() {
	id := l.removeOverlapHelper(0)
	for id != -1 {
		id = l.removeOverlapHelper(id)
	}
}

func (l fixPositionList) removeOverlapHelper(id int) int {
	index := 0
	for i := 0; i < len(l); i++ {
		if id == 0 {
			break
		}
		if l[i].id == id {
			index = i
		}
	}
	// continue with where we left off previously with the index
	for i := index; i < len(l); i++ {
		for j := i + 1; j < len(l); j++ {
			if l[i].overlaps(l[j]) {
				id := l[i].id
				l.removeSuggestedFix(l[i].suggestedFix)
				return id
			}
		}
	}
	return -1
}

func (l fixPositionList) removeSuggestedFix(s *analysis.SuggestedFix) {
	for i := 0; i < len(l); i++ {
		if l[i].suggestedFix == s {
			l = append(l[:i], l[i+1:]...)
			i--
		}
	}
}
