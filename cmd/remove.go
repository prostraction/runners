package cmd

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/runners/config"
	"github.com/runners/docker"
	"github.com/spf13/cobra"
)

var (
	removeAll bool
)

var removeCmd = &cobra.Command{
	Use:     "remove [name...]",
	Aliases: []string{"rm"},
	Short:   "Remove one, multiple, or all GitHub runners",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		dm, err := docker.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize docker manager: %w", err)
		}

		ctx := context.Background()

		var namesToRemove []string
		if removeAll {
			fmt.Println("Removing all runners...")
			for name := range cfg.Runners {
				namesToRemove = append(namesToRemove, name)
			}
			sort.Strings(namesToRemove)
		} else {
			if len(args) == 0 {
				return fmt.Errorf("please specify at least one runner name or use --all")
			}
			namesToRemove = args
		}

		for _, name := range namesToRemove {
			runner, exists := cfg.Runners[name]
			if !exists {
				log.Printf("Warning: runner '%s' not found", name)
				continue
			}

			fmt.Printf("Stopping and removing runner '%s'...\n", name)
			
			if err := dm.StopRunner(ctx, runner.ContainerID); err != nil {
				log.Printf("Warning: failed to stop container for '%s': %v", name, err)
			}
			
			if err := dm.RemoveRunner(ctx, runner.ContainerID); err != nil {
				log.Printf("Warning: failed to remove container for '%s': %v", name, err)
			}

			if err := config.RemoveRunner(name); err != nil {
				log.Printf("Error: failed to remove '%s' from config: %v", name, err)
				continue
			}

			if err := config.RemoveDataDir(name); err != nil {
				log.Printf("Warning: failed to remove data directory for '%s': %v", name, err)
			}

			fmt.Printf("Successfully removed runner '%s'.\n", name)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
	removeCmd.Flags().BoolVarP(&removeAll, "all", "a", false, "Remove all runners")
}
