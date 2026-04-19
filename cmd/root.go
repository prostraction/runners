package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "runners",
	Short: "A CLI to manage GitHub runners using Docker",
	Long:  `Runners is a CLI tool that allows you to easily spin up, manage, and remove multiple GitHub runners on a single machine using Docker.`,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
