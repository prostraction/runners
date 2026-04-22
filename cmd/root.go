package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// normalizeMemoryAliases maps --mem and --ram onto --memory so all three spellings
// hit the same flag. Attach with flags.SetNormalizeFunc(normalizeMemoryAliases).
func normalizeMemoryAliases(f *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "mem", "ram":
		return pflag.NormalizedName("memory")
	}
	return pflag.NormalizedName(name)
}

var rootCmd = &cobra.Command{
	Use:   "runners",
	Short: "A CLI to manage GitHub runners using Docker",
	Long:  `Runners is a CLI tool that allows you to easily spin up, manage, and remove multiple GitHub runners on a single machine using Docker.`,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
