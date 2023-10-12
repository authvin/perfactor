package util

import (
	"errors"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

func WriteModifiedAST(fset *token.FileSet, astFile *ast.File, filePath string, fileName string) {
	// write the modified astFile to a new file
	// make sure output folder exists

	// ensure filePath ends in /
	if !strings.HasSuffix(filePath, p) {
		filePath += p
	}
	if strings.Contains(fileName, p) {
		// if the filename contains a path, get the last part of it
		filePath += fileName[:strings.LastIndex(fileName, p)]
		fileName = fileName[strings.LastIndex(fileName, p)+1:]
		if !strings.HasSuffix(filePath, p) {
			filePath += p
		}
	}
	err := os.MkdirAll(filePath, os.ModePerm)
	if err != nil {
		fmt.Println("Failed to create folder for modified AST: " + err.Error())
		return
	}
	// os.Create will truncate a file if it already exists
	file, err := os.Create(filePath + fileName)
	if err != nil {
		fmt.Println("Failed to create file for modified AST: " + err.Error())
		return
	}
	if fset == nil || astFile == nil {
		fmt.Println("Failed to write modified AST: fset or astFile is nil")
		return
	}
	err = printer.Fprint(file, fset, astFile)
	if err != nil {
		fmt.Println("Failed to print modified AST: " + err.Error())
		return
	}
}

// GetAllGoFilesInDir returns a list of all .go files in the given directory
func GetAllGoFilesInDir(dirPath string) ([]string, error) {
	return getAllGoFilesInDir(dirPath, "")
}

func getAllGoFilesInDir(dirPath, subDirPath string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if strings.HasSuffix(path, ".go") {
			files = append(files, subDirPath+path)
		}
		if d.IsDir() && path != dirPath {
			if strings.HasPrefix(path, ".") || strings.HasPrefix(path, "_") {
				// skip hidden directories
				return nil
			}
			// recurse into the directory
			subDirFiles, err := getAllGoFilesInDir(path, subDirPath+d.Name()+p)
			if err != nil {
				return err
			}
			files = append(files, subDirFiles...)
		}
		return nil
	})
	return files, err
}
