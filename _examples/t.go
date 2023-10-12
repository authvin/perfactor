package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"reflect"
)

func main() {
	// function to test if we can call a type checker on a package that calls a local package
	// load the file as an AST
	fileSet := token.NewFileSet()
	astFile, err := parser.ParseFile(fileSet, "_streakfinder/main.go", nil, parser.AllErrors)
	if err != nil {
		println("Error parsing file: " + err.Error())
		return
	}
	// get the type checker
	conf := types.Config{Importer: importer.ForCompiler(fileSet, "source", nil)}
	info := &types.Info{
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	_, err = conf.Check("_streakfinder", fileSet, []*ast.File{astFile}, info)
	if err != nil {
		println("Error getting type checker: " + err.Error())
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  "_streakfinder",
	}
	pkgs, err := packages.Load(cfg, "_streakfinder")
	if err != nil {
		println("Error loading packages: " + err.Error())
		return
	}
	for _, pkg := range pkgs {
		println("Package: " + pkg.Name)
	}
}

func main_old() {
	// testing to see if we can get the type of a map from an AST and a type checker
	// load the map_example file as an AST
	fileSet := token.NewFileSet()
	astFile, err := parser.ParseFile(fileSet, "_matrixmulti/main.go", nil, parser.AllErrors)
	if err != nil {
		println("Error parsing file: " + err.Error())
		return
	}
	// get the type checker
	conf := types.Config{Importer: importer.ForCompiler(fileSet, "source", nil)}
	info := &types.Info{
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Instances:  make(map[*ast.Ident]types.Instance),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
		InitOrder:  make([]*types.Initializer, 0),
	}
	pkg := types.NewPackage("_matrixmulti", "")
	checker := types.NewChecker(&conf, fileSet, pkg, info)
	err = checker.Files([]*ast.File{astFile})
	if err != nil {
		println("Error getting type checker: " + err.Error())
		return
	}
	// traverse the AST, printing each node and printing its type
	ast.Inspect(astFile, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		s := ""
		if e, ok := node.(ast.Expr); ok {
			s = fmt.Sprintf("%s\n", info.TypeOf(e))
		} else {
			s = fmt.Sprintf("Not an expression\n")
		}
		fmt.Printf("%s \t: %s \t: %s\n", "node", reflect.TypeOf(node), s)
		return true
	})
}
