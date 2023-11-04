package cmd

import (
	"bytes"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"perfactor/tests"
	"strconv"
	"strings"
)

var testCmd = &cobra.Command{
	Use:     "test",
	Short:   "Run the tests for the full program",
	Aliases: []string{"t"},
	Run:     run_tests,
}

func init() {
	RootCmd.AddCommand(testCmd)
}

func run_tests(cmd *cobra.Command, args []string) {
	RunTests()
}

type Result struct {
	Correct    int
	Incorrect  int
	NotFound   int
	Unexpected int
}

func RunTests() {
	dirEntry, err := os.ReadDir("tests")
	if err != nil {
		fmt.Printf("Error reading directory: %s\n", err.Error())
		return
	}
	for _, entry := range dirEntry {
		if entry.IsDir() {
			continue
		}
		if entry.Name() == "make_pred_map.go" || entry.Name() == "prediction.go" {
			continue
		}
		if entry.Name()[len(entry.Name())-3:] == ".go" {
			predictions := getPredictions(entry.Name())
			result := VerifyFile(entry.Name(), predictions)
			fmt.Printf("Correct: %d\n", result.Correct)
			fmt.Printf("Incorrect: %d\n", result.Incorrect)
			fmt.Printf("Not found: %d\n", result.NotFound)
			fmt.Printf("Unexpected: %d\n", result.Unexpected)
		}
	}
}

func getPredictions(s string) map[int]tests.Prediction {
	switch s {
	case "array.go":
		return tests.ArrayPredictions
	case "branch.go":
		return tests.BranchPredictions
	case "loopvar.go":
		return tests.LoopvarPredictions
	case "return.go":
		return tests.ReturnPredictions
	case "methodcall.go":
		return tests.MethodcallPredictions
	case "assign.go":
		return tests.AssignPredictions
	default:
		println("No predictions found for " + s)
		os.Exit(0)
	}
	return nil
}

// BufferAndStdoutWriter implements io.Writer
type BufferAndStdoutWriter struct {
	Buffer *bytes.Buffer
}

// NewBufferAndStdoutWriter constructor
func NewBufferAndStdoutWriter() *BufferAndStdoutWriter {
	return &BufferAndStdoutWriter{
		Buffer: new(bytes.Buffer),
	}
}

// Write writes to both the buffer and stdout
func (bw *BufferAndStdoutWriter) Write(p []byte) (n int, err error) {
	// Write to internal buffer
	n, err = bw.Buffer.Write(p)
	if err != nil {
		return n, err
	}

	// Write to stdout
	n, err = os.Stdout.Write(p)
	if err != nil {
		return n, err
	}

	return n, nil
}

func VerifyFile(fileName string, predictions map[int]tests.Prediction) Result {
	var res Result
	println("Running tests for " + fileName)
	// Initialize a map to keep track of the lines you've checked
	checkedLines := make(map[int]bool)
	pf := ProgramSettings{
		ProjectPath: "./",                // Need to run with root as project path
		FileName:    "tests/" + fileName, // File path will need to be prefixed with "tests/"
		Mode:        false,               // Because we're working on ourself, we can't run benchmarks - infinite recursion
		Id:          "test",
		FileNames:   []string{"tests/" + fileName},
		Output:      "_data",
	}
	//buffer := NewBufferAndStdoutWriter()
	buffer := new(bytes.Buffer)
	// Run the tool on the test file, storing the output in the buffer
	Full(pf, buffer)
	//fmt.Printf("Output: %s\n", buffer.Buffer.String())
	//output := bytes.Split(buffer.Buffer.Bytes(), []byte("\n"))
	output := bytes.Split(buffer.Bytes(), []byte("\n"))
	var fileLines []string
	for _, line := range output {
		fileLines = append(fileLines, string(line))
	}
	// Loop through each line of the file
	for _, lineContent := range fileLines {
		lineNum, pass := parseLine(lineContent)
		if lineNum == -1 {
			//fmt.Printf("Error parsing line, did not start with 'Rejected:' or 'Refactored:': %s\n", lineContent)
			continue
		}
		if lineNum == -2 {
			fmt.Printf("Error parsing line, did not contain ';': %s\n", lineContent)
			continue
		}
		if lineNum == -3 {
			fmt.Printf("Error parsing line, did not contain ':': %s\n", lineContent)
			continue
		}
		if lineNum == -4 {
			fmt.Printf("Error parsing line, line number was not a number: %s\n", lineContent)
			continue
		}

		if prediction, exists := predictions[lineNum]; exists {
			checkedLines[lineNum] = true
			if prediction.ShouldPass == pass {
				res.Correct++
			} else {
				res.Incorrect++
				fmt.Printf("Incorrect result: %s\n", lineContent)
			}
		} else {
			fmt.Printf("Unexpected result: %s\n", lineContent)
			res.Unexpected++
		}
	}

	// Identify "Not Found" predictions
	for lineNum := range predictions {
		if !checkedLines[lineNum] {
			fmt.Printf("Not found in results: %d\n", lineNum)
			res.NotFound++
		}
	}

	return res
}

func parseLine(content string) (int, bool) {
	if !strings.HasPrefix(content, "Rejected:") && !strings.HasPrefix(content, "Refactored:") {
		return -1, false
	}
	t := strings.Split(content, ";")
	if len(t) < 2 {
		return -2, false
	}
	s := strings.Split(t[0], ":")
	if len(s) != 2 {
		return -3, false
	}
	lineNum, err := strconv.Atoi(strings.TrimSpace(s[1]))
	if err != nil {
		return -4, false
	}
	return lineNum, strings.Contains(content, "Refactored:")
}
