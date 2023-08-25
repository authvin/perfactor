package _old

import (
	"github.com/spf13/cobra"
)

var fieldCmd = &cobra.Command{
	Use:     "field",
	Short:   "Run the field alignment analyser",
	Aliases: []string{"fi"},
	Run:     f,
}

func init() {
	//cmd.rootCmd.AddCommand(fieldCmd)
}

func f(cmd *cobra.Command, args []string) {
	//cmd.RunAnalyser(fieldalignment.Analyzer, "_examples/_streakfinder", "main.go", "", "RunProgram", "", "")
}
