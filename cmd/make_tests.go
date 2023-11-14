package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"perfactor/tests"
)

var testMakeCmd = &cobra.Command{
	Use:     "maketest",
	Short:   "Make the tests for the full program (requires modifying run_tests.go to add more tests)",
	Aliases: []string{"m"},
	Run:     run_make_tests,
}

func init() {
	testMakeCmd.Flags().StringP("filename", "f", "", "The path to the input file")
	RootCmd.AddCommand(testMakeCmd)
}

func run_make_tests(cmd *cobra.Command, args []string) {
	fileName := cmd.Flag("filename").Value.String()
	if fileName == "" {
		// without filename, we instead just do it for all files ending with .go
		// except make_pred_map.go and prediction.go
		dirEntry, err := os.ReadDir("tests")
		if err != nil {
			fmt.Printf("Error reading directory: %s\n", err.Error())
			return
		}
		for _, entry := range dirEntry {
			if entry.IsDir() {
				continue
			}
			if entry.Name() == "make_pred_map.go" || entry.Name() == "prediction.go" {
				continue
			}
			if entry.Name()[len(entry.Name())-3:] == ".go" {
				err := tests.ProcessGoFile("tests/" + entry.Name())
				if err != nil {
					fmt.Printf("Error processing file: %s\n", err.Error())
					return
				}
			}
			println("Predictions map made for " + entry.Name())
		}
		return
	}
	err := tests.ProcessGoFile(fileName)
	if err != nil {
		fmt.Printf("Error processing file: %s\n", err.Error())
		return
	}
}
