package cmd

import (
	"bytes"
	"fmt"
	"github.com/owenrumney/go-sarif/sarif"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"io"
	"os"
	"perfactor/cmd/util"
)

type NoData struct {
	astFile     *ast.File
	fileSet     *token.FileSet
	info        *types.Info
	pkgs        []*packages.Package
	out         io.Writer
	projectPath string
	sarifRun    *sarif.Run
}

func (f NoData) GetWorkingDirPath() string {
	return f.projectPath
}

func (f NoData) SetWriter(out io.Writer) RefactoringMode {
	f.out = out
	return f
}

func (f NoData) WriteSarifFile(pf ProgramSettings) {
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

func (f NoData) SetupSarif() RefactoringMode {
	f.sarifRun = sarif.NewRun("perfactor_wo", "uri_placeholder")
	return f
}

func (f NoData) GetLoopInfoArray(fileSet *token.FileSet, pkgName string, projectPath string, pf ProgramSettings) (RefactoringMode, util.LoopInfoArray) {
	astFile, info, err := getFileFromPkgs(pkgName, pf.FileName, f.pkgs)
	if err != nil {
		fmt.Fprintf(f.out, "Error parsing files: "+err.Error())
		return f, nil
	}
	f.astFile = astFile
	f.fileSet = fileSet
	f.info = info
	acceptMap := getAcceptMap(pf.Accept, f.out)

	loops := util.FindForLoopsInAST(astFile, fileSet, nil)

	safeLoops := util.FindSafeLoopsForRefactoring(loops, fileSet, f.sarifRun, projectPath+pf.FileName, acceptMap, info, f.out)

	return f, util.GetLoopInfoArray(safeLoops)
}

func (f NoData) SetWorkingDirPath(pf ProgramSettings) RefactoringMode {
	f.projectPath = pf.ProjectPath
	return f
}

func (f NoData) LoadFiles(fileSet *token.FileSet) RefactoringMode {
	f.pkgs = parseFiles(f.projectPath, fileSet, f.out)
	return f
}

func (f NoData) RefactorLoop(loopInfo util.LoopInfo, pkgName string, pf ProgramSettings) (RefactoringMode, bool, error) {
	line := loopInfo.Loop.Line

	// Do the refactoring of the loopPos
	util.MakeLoopConcurrent(f.astFile, f.fileSet, line, f.info)
	fmt.Fprintf(f.out, "Refactored: %v ;\n", line)
	return f, true, nil
}

func (f NoData) WriteResult(pf ProgramSettings) {
	util.WriteModifiedAST(f.fileSet, f.astFile, pf.Output+p+pf.Id+p, pf.FileName)
	fmt.Fprintf(f.out, "Final version written to %s\n", pf.Output+p+pf.Id+p+pf.FileName)
}
