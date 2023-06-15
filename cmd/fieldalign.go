package cmd

import (
	"github.com/spf13/cobra"
	"golang.org/x/tools/go/analysis/passes/fieldalignment"
)

var fieldCmd = &cobra.Command{
	Use:     "field",
	Short:   "Run the field alignment analyser",
	Aliases: []string{"fi"},
	Run:     f,
}

func init() {
	rootCmd.AddCommand(fieldCmd)
}

func f(cmd *cobra.Command, args []string) {
	RunAnalyser(fieldalignment.Analyzer, "_examples/_streakfinder", "main.go", "", "RunProgram", "", "")
}
