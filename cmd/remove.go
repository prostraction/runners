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
	Use:   "remove [name]",
	Short: "Remove one or all GitHub runners",
	Args:  cobra.MaximumNArgs(1),
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

		if removeAll {
			fmt.Println("Removing all runners...")
			names := make([]string, 0, len(cfg.Runners))
			for name := range cfg.Runners {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				runner := cfg.Runners[name]
				fmt.Printf("Stopping and removing runner '%s'...\n", name)
				if err := dm.StopRunner(ctx, runner.ContainerID); err != nil {
					log.Printf("Warning: failed to stop runner '%s': %v", name, err)
				}
				if err := dm.RemoveRunner(ctx, runner.ContainerID); err != nil {
					log.Printf("Warning: failed to remove container for '%s': %v", name, err)
				}
				if err := config.RemoveRunner(name); err != nil {
					log.Printf("Error: failed to remove '%s' from config: %v", name, err)
				}
			}
			fmt.Println("All runners removed.")
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("please specify a runner name or use --all")
		}

		name := args[0]
		runner, exists := cfg.Runners[name]
		if !exists {
			return fmt.Errorf("runner '%s' not found", name)
		}

		fmt.Printf("Stopping and removing runner '%s'...\n", name)
		
		if err := dm.StopRunner(ctx, runner.ContainerID); err != nil {
			log.Printf("Warning: failed to stop container: %v", err)
		}
		
		if err := dm.RemoveRunner(ctx, runner.ContainerID); err != nil {
			log.Printf("Warning: failed to remove container: %v", err)
		}

		if err := config.RemoveRunner(name); err != nil {
			return fmt.Errorf("failed to remove runner from config: %w", err)
		}

		fmt.Printf("Successfully removed runner '%s'.\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
	removeCmd.Flags().BoolVarP(&removeAll, "all", "a", false, "Remove all runners")
}
