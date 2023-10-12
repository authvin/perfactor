package cmd

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"io"
	"perfactor/cmd/util"
)

type NoData struct {
	astFile     *ast.File
	fileSet     *token.FileSet
	info        *types.Info
	pkgs        []*packages.Package
	out         io.Writer
	projectPath string
}

func (f NoData) GetWorkingDirPath() string {
	return f.projectPath
}

func (f NoData) SetWriter(out io.Writer) RefactoringMode {
	f.out = out
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

	safeLoops := util.FindSafeLoopsForRefactoring(loops, fileSet, nil, projectPath+pf.FileName, acceptMap, info, f.out)

	return f, util.GetLoopInfoArray(safeLoops)
}

func (f NoData) SetWorkingDirPath(pf ProgramSettings) {
	f.projectPath = pf.ProjectPath
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
