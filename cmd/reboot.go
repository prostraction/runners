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
	rebootAll bool
)

var rebootCmd = &cobra.Command{
	Use:     "reboot [name]",
	Aliases: []string{"restart"},
	Short:   "Reboot (restart) one or all GitHub runners",
	Args:    cobra.MaximumNArgs(1),
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

		if rebootAll {
			fmt.Println("Rebooting all runners...")
			
			names := make([]string, 0, len(cfg.Runners))
			for name := range cfg.Runners {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				runner := cfg.Runners[name]
				fmt.Printf("Rebooting runner '%s'...\n", name)
				
				// Stop existing container
				if err := dm.StopRunner(ctx, runner.ContainerID); err != nil {
					log.Printf("Warning during stop for '%s': %v", name, err)
				}

				// Try to resume first
				if err := dm.ResumeRunner(ctx, runner.ContainerID); err == nil {
					continue
				}

				// If resume fails, it means container is missing, so we must recreate
				fmt.Printf("Container for '%s' missing. Re-creating...\n", name)
				if err := dm.StartRunner(ctx, runner); err != nil {
					log.Printf("Error: failed to restart runner '%s': %v", name, err)
					continue
				}
				if err := config.UpdateRunner(runner); err != nil {
					log.Printf("Warning: failed to save container ID for '%s' to config: %v", name, err)
				}
			}
			fmt.Println("All runners rebooted.")
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

		fmt.Printf("Rebooting runner '%s'...\n", name)
		
		// Stop if running
		if err := dm.StopRunner(ctx, runner.ContainerID); err != nil {
			log.Printf("Warning during stop: %v", err)
		}

		// Try to resume
		if err := dm.ResumeRunner(ctx, runner.ContainerID); err == nil {
			fmt.Printf("Successfully rebooted runner '%s' (resumed).\n", name)
			return
		}

		// Fallback to recreate
		fmt.Printf("Container for '%s' missing. Re-creating...\n", name)
		_ = dm.RemoveRunner(ctx, runner.ContainerID)
		if err := dm.StartRunner(ctx, runner); err != nil {
			log.Fatalf("Failed to start runner: %v", err)
		}

		if err := config.UpdateRunner(runner); err != nil {
			log.Printf("Warning: failed to save container ID to config: %v", err)
		}

		fmt.Printf("Successfully rebooted runner '%s'.\n", name)
	},
}

func init() {
	rootCmd.AddCommand(rebootCmd)
	rebootCmd.Flags().BoolVarP(&rebootAll, "all", "a", false, "Reboot all runners")
}
