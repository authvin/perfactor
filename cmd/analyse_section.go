package cmd

import (
	"bytes"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"perfactor/cmd/util"
	"runtime"
	"strings"

	"github.com/owenrumney/go-sarif/sarif"
	"github.com/spf13/cobra"
)

var analyseCmd = &cobra.Command{
	Use:     "analyse",
	Short:   "Do static analysis for a portion of the program",
	Aliases: []string{"a"},
	Run:     analyse,
}

var projPath string
var fName string
var startPos int
var endPos int

func init() {
	//
	analyseCmd.Flags().StringVarP(&projPath, "project", "p", "", "The path to the project")
	analyseCmd.Flags().StringVarP(&fName, "filename", "f", "", "The path to the input file")
	analyseCmd.Flags().IntVarP(&startPos, "startline", "s", -1, "The starting position to look through")
	analyseCmd.Flags().IntVarP(&endPos, "endline", "e", -1, "The ending position to look through")
	rootCmd.AddCommand(analyseCmd)
}

func analyse(cmd *cobra.Command, args []string) {
	// input: start pos, end pos
	// output: written, in SARIF format

	// the SARIF report that we want to fill
	run := sarif.NewRun("name_placeholder", "uri_placeholder")

	baseDir, err := os.Getwd()
	if err != nil {
		println("Error getting working directory: " + err.Error())
		return
	}

	fpath := convertPathForWindowsIfNeeded(baseDir + projPath + fName)

	// get the AST from the file in the project
	fset := token.NewFileSet()
	astFile := util.GetASTFromFile(projPath+fName, fset)
	forloops := util.FindForLoopsInAST(astFile, fset, func(node ast.Node, fset *token.FileSet) bool {
		pos := fset.Position(node.Pos())
		if (startPos == -1 || pos.Line >= startPos) && (endPos == -1 || pos.Line <= endPos) {
			return true
		}
		return false
	})
	util.FindSafeLoopsForRefactoring(forloops, fset, run, fpath)
	report, err := sarif.New(sarif.Version210)
	if err != nil {
		println("Error creating SARIF report: " + err.Error())
		return
	}
	report.AddRun(run)
	buffer := bytes.NewBufferString("")
	err = report.Write(buffer)
	if err != nil {
		println("Error writing SARIF report: " + err.Error())
		return
	}
	// write the buffer to a serif file
	create, err := os.Create("test.sarif")
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
	println("SARIF file written to test.sarif")
	//println(buffer.String())
}

func convertPathForWindowsIfNeeded(path string) string {
	if runtime.GOOS != "linux" {
		return path
	}

	// Check if the path is under the WSL /mnt directory
	if strings.HasPrefix(path, "/mnt/") {
		// Remove the /mnt prefix and split the remaining path
		remainingPath := strings.TrimPrefix(path, "/mnt/")
		parts := strings.SplitN(remainingPath, "/", 2)

		// Reconstruct the path as a Windows path
		if len(parts) >= 2 {
			windowsDrive := strings.ToUpper(parts[0]) + ":"
			windowsPath := filepath.Join(windowsDrive, parts[1])
			return windowsPath
		}
	}

	return path
}
