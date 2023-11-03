package tests

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func ProcessGoFile(filename string) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	predictions := make(map[int]Prediction)

	var startPos, endPos token.Pos

	for _, commentGroup := range node.Comments {
		for _, comment := range commentGroup.List {
			position := fset.Position(comment.Pos())
			//fmt.Println(comment.Text, position.Line) // Debugging line
			switch comment.Text {
			case "// Allowed":
				predictions[position.Line] = Prediction{Line: position.Line, ShouldPass: true}
			case "// Not allowed":
				predictions[position.Line] = Prediction{Line: position.Line, ShouldPass: false}
			}
		}
	}

	ast.Inspect(node, func(n ast.Node) bool {
		// Check for existing map variable
		decl, ok := n.(*ast.GenDecl)
		if ok && decl.Tok == token.VAR {
			for _, spec := range decl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if ok {
					for _, ident := range valueSpec.Names {
						baseName := filepath.Base(filename)
						nameNoExt := baseName[:len(baseName)-len(filepath.Ext(baseName))]
						mapName := fmt.Sprintf("%s%s", strings.ToUpper(nameNoExt[:1]), nameNoExt[1:])
						if ident.Name == mapName+"Predictions" {
							startPos = decl.Pos()
							endPos = decl.End()
						}
					}
				}
			}
		}

		return true
	})

	// Extract the base name of the file without extension
	baseName := filepath.Base(filename)
	nameNoExt := baseName[:len(baseName)-len(filepath.Ext(baseName))]
	// Capitalize the first letter of the filename
	mapName := fmt.Sprintf("%s%s", strings.ToUpper(nameNoExt[:1]), nameNoExt[1:])

	var predictionStr = fmt.Sprintf("var %sPredictions = map[int]Prediction{\n", mapName)

	for line, prediction := range predictions {
		predictionStr += fmt.Sprintf("\t%d:  {%d, %v},\n", line, prediction.Line, prediction.ShouldPass)
	}
	predictionStr += "}\n"

	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// Remove existing map if found
	if startPos != 0 && endPos != 0 {
		startOffset := fset.Position(startPos).Offset
		endOffset := fset.Position(endPos).Offset
		data = append(data[:startOffset], data[endOffset:]...)
	}

	// Remove trailing newlines from original data
	for len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}

	err = os.WriteFile(filename, append(append(data, '\n', '\n'), predictionStr...), 0644)
	if err != nil {
		return err
	}

	return nil
}
