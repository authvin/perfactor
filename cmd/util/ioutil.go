package util

import (
	"errors"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"os"
)

func CleanOrCreateTempFolder(path string) {
	// file exist check is taken from: https://stackoverflow.com/questions/12518876/how-to-check-if-a-file-exists-in-go
	if _, err := os.Stat(path); err == nil {
		// path/to/whatever exists
		err2 := os.RemoveAll(path)
		if err2 != nil {
			println("Error removing temp folder: " + err2.Error())
			return
		}
		// make the tmp folder
		err3 := os.MkdirAll(path, os.ModePerm)
		if err3 != nil {
			println("Error creating temp folder: " + err3.Error())
			return
		}
	} else if errors.Is(err, os.ErrNotExist) {
		// path/to/whatever does *not* exist
		err3 := os.MkdirAll(path, os.ModePerm)
		if err3 != nil {
			println("Error creating temp folder: " + err3.Error())
			return
		}
	}
}

func WriteModifiedAST(fset *token.FileSet, astFile *ast.File, filePath string) {
	// write the modified astFile to a new file
	// os.Create will truncate a file if it already exists
	file, err := os.Create(filePath)
	if err != nil {
		fmt.Println("Failed to create file for modified AST: " + err.Error())
		return
	}
	err = printer.Fprint(file, fset, astFile)
	if err != nil {
		fmt.Println("Failed to print modified AST: " + err.Error())
		return
	}
}
