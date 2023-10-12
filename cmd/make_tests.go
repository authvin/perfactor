package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"perfactor/tests"
)

var testMakeCmd = &cobra.Command{
	Use:     "maketest",
	Short:   "Run the tests for the full program",
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
		fmt.Printf("Error: filename is required\n")
		return
	}
	err := tests.ProcessGoFile(fileName)
	if err != nil {
		fmt.Printf("Error processing file: %s\n", err.Error())
		return
	}
}
