/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "perfactor",
	Short: "A tool to find possible refactorings in your code using performance data",
	Long:  `Now we're in the root command'`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets Flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		println("Failed to execute command: " + err.Error())
		os.Exit(1)
	}
}

func init() {
	// Here you will define your Flags and configuration settings.
	// Cobra supports persistent Flags, which, if defined here,
	// will be global for your application.

	// RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.perfactor.yaml)")

	// Cobra also supports local Flags, which will only run
	// when this action is called directly.
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
