package util

import (
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
)

func FindAssignedIdentifiers(loop *ast.ForStmt) map[*ast.Ident]bool {
	writtenTo := make(map[*ast.Ident]bool)
	ast.Inspect(loop, func(n ast.Node) bool {
		if assignStmt, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range assignStmt.Lhs {
				// Check if the assignment is to an index of an array or map
				if indexExpr, ok := assignStmt.Lhs[0].(*ast.IndexExpr); ok {
					// if the index expression is an identifier, check if it's the Loop variable
					if ident, ok := indexExpr.Index.(*ast.Ident); ok {
						// check if the identifier is the Loop variable
						if ident.Obj == loop.Init.(*ast.AssignStmt).Lhs[0].(*ast.Ident).Obj {
							//println("Found an assignment to an array using Loop variable as the index, allowing it at ", f.Position(Loop.Pos()).Line)
							continue
						}
					}
				}
				if ident, ok := lhs.(*ast.Ident); ok {
					// check if the identifier's declaration is within the Loop
					if ident.Obj.Pos() < loop.Pos() || ident.Obj.Pos() > loop.End() {
						// document a test case where the analysis is wrong but still safe
						//println("found identifier: ", ident.Name, " at line ", f.Position(ident.Pos()).Line)
						writtenTo[ident] = true
					}
				}
			}
		}
		return true
	})
	return writtenTo
}

// FindForLoopsInAST The valid function argument is for special cases, like filtering for specific line numbers
// leave as nil to not do any filtering
func FindForLoopsInAST(astFile ast.Node, fset *token.FileSet, valid func(ast.Node, *token.FileSet) bool) []Loop {
	// array of AST positions for for loops
	var forLoops []Loop

	// Traverse the AST looking for loops
	ast.Inspect(astFile, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		switch n := n.(type) {
		case *ast.ForStmt:
			if valid != nil && !valid(n, fset) {
				return true
			}
			//fmt.Println("Found a for Loop at line", fset.Position(n.Pos()).Line)
			forLoops = append(forLoops, Loop{
				For:     n,
				Pos:     n.Pos(),
				End:     n.End(),
				Body:    n.Body,
				Line:    fset.Position(n.Pos()).Line,
				EndLine: fset.Position(n.End()).Line,
			})
		case *ast.RangeStmt:
			if valid != nil && !valid(n, fset) {
				return true
			}
			//fmt.Println("Found a range Loop at line", fset.Position(n.Pos()).Line)
			forLoops = append(forLoops, Loop{
				Range:   n,
				Pos:     n.Pos(),
				End:     n.End(),
				Body:    n.Body,
				Line:    fset.Position(n.Pos()).Line,
				EndLine: fset.Position(n.End()).Line,
			})
		}
		return true
	})
	return forLoops
}

func GetASTFromFile(inputPath string, fset *token.FileSet) *ast.File {
	// parse the file
	f, err := parser.ParseFile(fset, inputPath, nil, parser.ParseComments)
	if err != nil {
		println("Failed to parse AST from file: " + err.Error())
		os.Exit(0)
	}
	return f
}

func GetPackageNameFromPath(inputPath string) string {
	fset := token.NewFileSet()
	// parse the file
	f, err := parser.ParseFile(fset, inputPath, nil, parser.PackageClauseOnly)
	if err != nil {
		fmt.Printf("Failed to get package name from file %s: %v\n", inputPath, err.Error())
		os.Exit(0)
	}
	return f.Name.Name
}

func GetTypeCheckerInfo(pkgPath string, astFiles map[string]*ast.Package, fset *token.FileSet) *types.Info {
	// get type information from the type checker
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	info := &types.Info{
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	for k, v := range astFiles {
		if k == pkgPath {
			continue
		}
		// fill in the info object using the type checker
		_, err := conf.Check(k, fset, arrayFromMap(v), info)
		if err != nil {
			println("Failed to get type checker: " + err.Error())
			os.Exit(0)
		}
	}

	return info
}

func GetTypeCheckerInfoFromFile(pkgPath string, astFiles []*ast.File, fset *token.FileSet) *types.Info {
	// get type information from the type checker
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	info := &types.Info{
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
		Types: make(map[ast.Expr]types.TypeAndValue),
	}

	// fill in the info object using the type checker
	_, err := conf.Check(pkgPath, fset, astFiles, info)
	if err != nil {
		println("Failed to get type checker: " + err.Error())
		os.Exit(0)
	}
	return info
}

func arrayFromMap(p *ast.Package) []*ast.File {
	if p == nil {
		println("Package is nil")
		return nil
	}
	var arr []*ast.File
	for _, f := range p.Files {
		arr = append(arr, f)
	}
	return arr
}

func ParseDirectories(path string, fset *token.FileSet) (map[string]*ast.Package, error) {
	// parse a directory, including sub-directories, and combine all of their ASTs into a single map
	// depth-first traversal?

	packages := make(map[string]*ast.Package, 0)
	err := traverse(packages, fset, path)
	return packages, err
}

// to import parallel; get the module string, then check all import strings if they start with the module string. If they do, we go get it.

func traverse(pkgs map[string]*ast.Package, fset *token.FileSet, path string) error {
	// traverse the directory tree, recursively calling this function on all directories
	// get the list of files from dir
	list, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	//println("traversing " + path)
	for _, file := range list {
		if file.IsDir() {
			// this is a directory, so recurse into it
			err := traverse(pkgs, fset, path+file.Name()+p)
			if err != nil {
				println("failed to traverse: " + path)
				return err
			}
		}
	}

	// run parser.ParseDir on this directory, and put the results into the pkgs map
	packages, err := parser.ParseDir(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		return nil
	}

	for k, v := range packages {
		if _, ok := pkgs[k]; ok {
			// package already exists, so throw an error
			return errors.New("package already exists: " + k)
		} else {
			// package does not exist, so add it to the map
			pkgs[k] = v
		}
	}

	return nil
}
