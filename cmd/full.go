package cmd

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
	"io"
	"os"
	"os/exec"
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

const p = string(os.PathSeparator)

func init() {
	fullCmd.Flags().StringP("benchname", "b", "RunProgram", "The Id of the benchmark to run")
	fullCmd.Flags().StringP("Id", "n", "", "The Id/Id of the program to run")
	fullCmd.Flags().StringP("project", "p", "", "The path to the project")
	fullCmd.Flags().StringP("filename", "f", "", "The path to the input file")
	fullCmd.Flags().StringP("Output", "o", "_data", "The path to the Output folder")
	fullCmd.Flags().StringP("testname", "t", "NONE", "The Id of the test to run")
	fullCmd.Flags().StringP("Flags", "", "", "Any Flags to pass to the program")
	fullCmd.Flags().StringP("Accept", "a", "", "Accept an identifier in a given loop")
	fullCmd.Flags().IntP("Count", "c", 3, "The number of times to run the benchmark")
	fullCmd.Flags().Float32P("Threshold", "d", 10.0, "The Threshold for the percentage increase in runtime")
	fullCmd.Flags().BoolP("Mode", "m", false, "Benchmark the program when refactoring")
	fullCmd.Flags().BoolP("Sarif", "s", false, "Output the results in SARIF format")
	RootCmd.AddCommand(fullCmd)
}

func full(cmd *cobra.Command, args []string) {
	//Run program, giving an input path, Output path, Id of benchmark, Id of test, ID of the run, and any Flags
	// (that's how we get here)

	pf, err := programSettings(cmd)
	if err != nil {
		fmt.Printf("Error getting Flags: %s\n", err.Error())
		return
	}
	Full(pf, os.Stdout)
}

func Full(pf ProgramSettings, out io.Writer) {
	var mode RefactoringMode
	if pf.Mode {
		mode = WithData{}
	} else {
		mode = NoData{}
	}
	mode = mode.SetWriter(out)
	mode = mode.SetWorkingDirPath(pf)
	fileSet := token.NewFileSet()
	mode = mode.LoadFiles(fileSet)
	mode = mode.SetupSarif()
	for _, fileName := range pf.FileNames {
		_, _ = fmt.Fprintf(out, "Running on file: %s\n", fileName)
		pf.FileName = fileName
		tmpFilePath := mode.GetWorkingDirPath() + pf.FileName
		pkgName := util.GetPackageNameFromPath(tmpFilePath)

		//------- Program reads the input file and finds all for-loops
		var loopsToRefactor util.LoopInfoArray
		mode, loopsToRefactor = mode.GetLoopInfoArray(fileSet, pkgName, pf.ProjectPath, pf)
		if loopsToRefactor == nil {
			_, _ = fmt.Fprintf(out, "Error getting loops to refactor for file %s+n", fileName)
			continue
		}

		// Variable to keep track of the best duration
		// for now it's a strict greater-than, but we could make it require a percentage increase

		if len(loopsToRefactor) == 0 {
			_, _ = fmt.Fprintf(out, "No loops to refactor\n")
			continue
		} else {
			_, _ = fmt.Fprintf(out, "Found %v loops to refactor\n", len(loopsToRefactor))
		}

		hasModified := false

		//Program performs the refactoring of each loop
		for _, loopInfo := range loopsToRefactor {
			// -------- Program refactors the for-loops to be concurrent
			// New fileset and AST, which is needed because the type checker bugs out if we don't
			if mode == nil {
				_, _ = fmt.Fprintf(out, "Error: Mode not set\n")
				return
			}

			var result bool
			var err error
			mode, result, err = mode.RefactorLoop(loopInfo, pkgName, pf)
			if err != nil {
				_, _ = fmt.Fprintf(out, "Error refactoring loop: %s\n", err.Error())
				return
			}
			if result {
				hasModified = true
			}
		}
		if pf.Sarif {
			_, _ = fmt.Fprintf(out, "Writing SARIF file\n")
			mode.WriteSarifFile(pf)
		}

		if !hasModified {
			_, _ = fmt.Fprintf(out, "No loops were refactored\n")
			continue
		}

		// create Output folder
		err := os.MkdirAll(pf.Output+p+pf.Id+p, os.ModePerm)
		if err != nil {
			_, _ = fmt.Fprintf(out, "Error creating Output folder: %s\n"+err.Error())
			return
		}
		// write the finished program to Output
		mode.WriteResult(pf)
	}
}

func programSettings(cmd *cobra.Command) (ProgramSettings, error) {
	pf := ProgramSettings{}

	var err error
	pf.ProjectPath, err = cmd.Flags().GetString("project")
	if err != nil {
		return pf, err
	}
	if len(pf.ProjectPath) == 0 {
		fmt.Println("Please provide a project path")
		return pf, errors.New("no project path provided")
	}
	// make sure the project path ends with a slash
	if !strings.HasSuffix(pf.ProjectPath, p) {
		pf.ProjectPath += p
	}

	// Generate a UUID if no Id is provided
	pf.Id, err = cmd.Flags().GetString("Id")
	if err != nil {
		fmt.Println("Error getting Id: " + err.Error())
		return pf, err
	}
	if len(pf.Id) == 0 {
		u := uuid.New()
		pf.Id = u.String()
	}
	pf.Flags, err = cmd.Flags().GetString("Flags")
	if err != nil {
		return pf, err
	}
	pf.BenchName, err = cmd.Flags().GetString("benchname")
	if err != nil {
		return pf, err
	}
	pf.TestName, err = cmd.Flags().GetString("testname")
	if err != nil {
		return pf, err
	}
	pf.FileName, err = cmd.Flags().GetString("filename")
	if err != nil {
		return pf, err
	}
	pf.Output, err = cmd.Flags().GetString("Output")
	if err != nil {
		return pf, err
	}
	pf.Count, err = cmd.Flags().GetInt("Count")
	if err != nil {
		return pf, err
	}
	pf.Threshold, err = cmd.Flags().GetFloat32("Threshold")
	if err != nil {
		return pf, err
	}
	pf.Accept, err = cmd.Flags().GetString("Accept")
	if err != nil {
		return pf, err
	}
	pf.Mode, err = cmd.Flags().GetBool("Mode")
	if err != nil {
		return pf, err
	}
	pf.Sarif, err = cmd.Flags().GetBool("Sarif")
	if err != nil {
		return pf, err
	}
	if pf.FileName == "all" {
		pf.FileNames, err = util.GetAllGoFilesInDir(pf.ProjectPath)
		if err != nil {
			return pf, err
		}
	} else {
		pf.FileNames = []string{pf.FileName}
	}
	return pf, nil
}

func getAcceptMap(accept string, out io.Writer) map[string]int {
	acceptMap := make(map[string]int, 0)
	// parse the Accept string
	if len(accept) > 0 {
		accepts := strings.Split(accept, ",")
		for _, a := range accepts {
			str := strings.Split(a, ":")
			if len(str) != 2 {
				_, _ = fmt.Fprintf(out, "Invalid Accept string")
				continue
			}
			line, err := strconv.Atoi(str[1])
			if err != nil {
				_, _ = fmt.Fprintf(out, "Invalid Accept string: ", err.Error())
				continue
			}
			acceptMap[str[0]] = line
		}
	}
	return acceptMap
}

func downloadRequiredFromModule(err error, tmpPath string, out io.Writer) error {
	data, err := os.ReadFile(tmpPath + "go.mod")
	if err != nil {
		return err
	}

	modFile, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return err
	}

	for _, require := range modFile.Require {
		//fmt._,_ = fmt.Fprintf(out,"Found URL:", require.Mod.Path)
		cmd := exec.Command("go", "get", require.Mod.Path)
		output, err := cmd.Output()
		if err != nil {
			_, _ = fmt.Fprintf(out, string(output))
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr
			}
			return err
		}
	}
	return nil
}

func parseFiles(tmpPath string, fileSet *token.FileSet, out io.Writer) []*packages.Package {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedCompiledGoFiles | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  tmpPath,
		Fset: fileSet,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		_, _ = fmt.Fprintf(out, "Error loading packages: "+err.Error())
		return nil
	}
	return pkgs
}

func getFileFromPkgs(pkgName string, fileName string, pkgs []*packages.Package) (*ast.File, *types.Info, error) {
	// get the target ast file from the packages
	if len(pkgs) == 0 {
		return nil, nil, errors.New("Error: Packages did not exist")
	}

	for strings.HasPrefix(fileName, "../") {
		fileName = fileName[3:]
	}

	for _, p := range pkgs {
		if p.Name == pkgName {
			for i, f := range p.CompiledGoFiles {
				if strings.HasSuffix(f, fileName) {
					return p.Syntax[i], p.TypesInfo, nil
				}
			}

		}
	}
	return nil, nil, errors.New("Error getting AST from file")
}

// struct to contain the Flags for the program
type ProgramSettings struct {
	ProjectPath string
	BenchName   string
	TestName    string
	FileName    string
	Id          string
	Output      string
	Flags       string
	Count       int
	Threshold   float32
	Accept      string
	Mode        bool
	FileNames   []string
	Sarif       bool
}

type RefactoringMode interface {
	LoadFiles(fileSet *token.FileSet) RefactoringMode
	GetLoopInfoArray(fileSet *token.FileSet, pkgName string, projectPath string, pf ProgramSettings) (RefactoringMode, util.LoopInfoArray)
	RefactorLoop(loopInfo util.LoopInfo, pkgName string, pf ProgramSettings) (RefactoringMode, bool, error)
	WriteResult(pf ProgramSettings)
	GetWorkingDirPath() string
	SetWriter(out io.Writer) RefactoringMode
	SetWorkingDirPath(pf ProgramSettings) RefactoringMode
	WriteSarifFile(pf ProgramSettings)
	SetupSarif() RefactoringMode
}
