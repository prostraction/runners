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
	startAll bool
)

var startCmd = &cobra.Command{
	Use:   "start [name]",
	Short: "Start one or all stopped GitHub runners",
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

		if startAll {
			fmt.Println("Starting all runners...")
			
			names := make([]string, 0, len(cfg.Runners))
			for name := range cfg.Runners {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				runner := cfg.Runners[name]
				
				isRunning, _ := dm.IsRunning(ctx, runner.ContainerID)
				if isRunning {
					fmt.Printf("Runner '%s' is already running.\n", name)
					continue
				}

				fmt.Printf("Starting runner '%s'...\n", name)
				
				// Try to resume existing container first to preserve registration
				if runner.ContainerID != "" {
					if err := dm.ResumeRunner(ctx, runner.ContainerID); err == nil {
						fmt.Printf("Runner '%s' resumed.\n", name)
						continue
					}
				}

				// Fallback to creating a new container (requires valid token)
				fmt.Printf("Container not found or failed to resume. Re-creating runner '%s'...\n", name)
				_ = dm.RemoveRunner(ctx, runner.ContainerID)

				if err := dm.StartRunner(ctx, runner); err != nil {
					log.Printf("Error: failed to start runner '%s': %v", name, err)
					continue
				}
				if err := config.UpdateRunner(runner); err != nil {
					log.Printf("Warning: failed to save container ID for '%s' to config: %v", name, err)
				}
			}
			fmt.Println("Done.")
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

		isRunning, _ := dm.IsRunning(ctx, runner.ContainerID)
		if isRunning {
			fmt.Printf("Runner '%s' is already running.\n", name)
			return
		}

		fmt.Printf("Starting runner '%s'...\n", name)
		
		if runner.ContainerID != "" {
			if err := dm.ResumeRunner(ctx, runner.ContainerID); err == nil {
				fmt.Printf("Runner '%s' resumed.\n", name)
				return
			}
		}

		fmt.Printf("Container not found or failed to resume. Re-creating runner '%s'...\n", name)
		_ = dm.RemoveRunner(ctx, runner.ContainerID)

		if err := dm.StartRunner(ctx, runner); err != nil {
			log.Fatalf("Failed to start runner: %v", err)
		}

		if err := config.UpdateRunner(runner); err != nil {
			log.Printf("Warning: failed to save container ID to config: %v", err)
		}

		fmt.Printf("Successfully started runner '%s'.\n", name)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolVarP(&startAll, "all", "a", false, "Start all runners")
}
