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
			dataExists := config.DataDirExists(name)

			// Orphan path: the config entry is gone but leftovers (data dir or a
			// container named gh-runner-<name>) may still be on disk from a
			// previous partial failure. Clean them up rather than silently skipping.
			if !exists {
				if !dataExists {
					// Also try to remove a container that may have been left by a
					// previous failed add. ContainerRemove by name is a no-op if
					// none exists.
					_ = dm.RemoveContainerByName(ctx, name)
					log.Printf("Warning: runner '%s' not found", name)
					continue
				}
				fmt.Printf("Cleaning up orphaned '%s' (no config entry, data left on disk)...\n", name)
				_ = dm.RemoveContainerByName(ctx, name)
				if err := dm.PurgeDataDir(ctx, name); err != nil {
					log.Printf("Error: failed to remove data directory for orphan '%s': %v", name, err)
					continue
				}
				fmt.Printf("Removed orphan data for '%s'.\n", name)
				continue
			}

			fmt.Printf("Stopping and removing runner '%s'...\n", name)

			if err := dm.StopRunner(ctx, runner.ContainerID); err != nil {
				log.Printf("Warning: failed to stop container for '%s': %v", name, err)
			}

			if err := dm.RemoveRunner(ctx, runner.ContainerID); err != nil {
				log.Printf("Warning: failed to remove container for '%s': %v", name, err)
			}

			// Also remove by name, in case ContainerID is stale and a fresh
			// container with the same name exists (e.g. recreated out-of-band).
			_ = dm.RemoveContainerByName(ctx, name)

			if err := config.RemoveRunner(name); err != nil {
				log.Printf("Error: failed to remove '%s' from config: %v", name, err)
				continue
			}

			if err := dm.PurgeDataDir(ctx, name); err != nil {
				// Config is already gone; warn but don't claim full success so
				// the user knows disk state may still need cleanup.
				log.Printf("Warning: failed to remove data directory for '%s': %v", name, err)
				fmt.Printf("Removed runner '%s' from config, but data directory cleanup failed (see warning above).\n", name)
				continue
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
