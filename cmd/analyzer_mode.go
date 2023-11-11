package cmd

//import (
//	"fmt"
//	"go/ast"
//	"go/token"
//	"go/types"
//	"golang.org/x/tools/go/analysis"
//	"perfactor/cmd/util"
//	"perfactor/parallel_analyser"
//)
//
//type AnalyzerMode struct {
//	astFile *ast.File
//	fileSet *token.FileSet
//	info    *types.Info
//}
//
//func (f AnalyzerMode) GetLoopInfoArray(tmpPath string, fileSet *token.FileSet, pkgName string, ProjectPath string, pf ProgramSettings) (RefactoringMode, util.LoopInfoArray) {
//	astFile, info, err := parseFiles(tmpPath, fileSet, pkgName, pf.FileName)
//	if err != nil {
//		println("Error parsing files: " + err.Error())
//		return f, nil
//	}
//	f.astFile = astFile
//	f.fileSet = fileSet
//	f.info = info
//	acceptMap := getAcceptMap(pf.Accept)
//
//	a := parallel_analyser.Analyzer
//	resultsOf := make(map[*analysis.Analyzer]interface{})
//	if len(a.Requires) > 0 {
//		for _, req := range a.Requires {
//			fmt.Printf("Running prerequisite: %s\n", req.Name)
//			pass := &analysis.Pass{
//				Analyzer: req,
//				Fset:     fileSet,
//				Files:    []*ast.File{f.astFile},
//				Report:   func(d analysis.Diagnostic) {},
//				ResultOf: resultsOf,
//			}
//			result, err := req.Run(pass)
//			if err != nil {
//				println("Error running prerequisite: " + err.Error())
//				return f, nil
//			}
//			fmt.Printf("Adding result of prerequisite: %s\n", req.Name)
//			resultsOf[req] = result
//		}
//	}
//	diagnostics := make([]analysis.Diagnostic, 0)
//	// create the analysis pass
//	pass := &analysis.Pass{
//		Analyzer: a,
//		Fset:     fileSet,
//		Files:    []*ast.File{astFile},
//		Report:   func(d analysis.Diagnostic) { diagnostics = append(diagnostics, d) },
//		ResultOf: resultsOf,
//		// could add type info here, probably should, but need to rework our util method to handle multiple files for that
//	}
//	loopInfoArray, err := a.Run(pass)
//	if err != nil {
//		println("Error running analyzer: " + err.Error())
//		return f, nil
//	}
//	return f, loopInfoArray
//}
//
//func (f AnalyzerMode) RefactorLoop(loopInfo util.LoopInfo, tmpPath string, pkgName string, pf ProgramSettings) (RefactoringMode, bool, error) {
//
//}
//
//func (f AnalyzerMode) WriteResult(pf ProgramSettings) {
//	util.WriteModifiedAST(f.fileSet, f.astFile, pf.Output+p+pf.Id+p, pf.FileName)
//	println("Final version written to " + pf.Output + p + pf.Id + p + pf.FileName)
//}
