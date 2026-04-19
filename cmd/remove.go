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
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}

		dm, err := docker.NewManager()
		if err != nil {
			log.Fatalf("Failed to initialize docker manager: %v", err)
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
			return
		}

		if len(args) == 0 {
			log.Fatal("Please specify a runner name or use --all")
		}

		name := args[0]
		runner, exists := cfg.Runners[name]
		if !exists {
			log.Fatalf("Runner '%s' not found", name)
		}

		fmt.Printf("Stopping and removing runner '%s'...\n", name)
		
		if err := dm.StopRunner(ctx, runner.ContainerID); err != nil {
			log.Printf("Warning: failed to stop container: %v", err)
		}
		
		if err := dm.RemoveRunner(ctx, runner.ContainerID); err != nil {
			log.Printf("Warning: failed to remove container: %v", err)
		}

		if err := config.RemoveRunner(name); err != nil {
			log.Fatalf("Failed to remove runner from config: %v", err)
		}

		fmt.Printf("Successfully removed runner '%s'.\n", name)
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
	removeCmd.Flags().BoolVarP(&removeAll, "all", "a", false, "Remove all runners")
}
