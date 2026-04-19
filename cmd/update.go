package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/runners/config"
	"github.com/runners/docker"
	"github.com/spf13/cobra"
)

var (
	updateCPU    float64
	updateMemory int64
)

var updateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Update resource limits for an existing runner",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		cfg, err := config.LoadConfig()
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}

		runner, exists := cfg.Runners[name]
		if !exists {
			log.Fatalf("Runner '%s' not found", name)
		}

		// Update config values if flags were provided
		updated := false
		if cmd.Flags().Changed("cpu") {
			runner.CPULimit = updateCPU
			updated = true
		}
		if cmd.Flags().Changed("memory") {
			runner.MemoryLimit = updateMemory
			updated = true
		}

		if !updated {
			fmt.Println("No changes specified. Use --cpu or --memory to update limits.")
			return
		}

		dm, err := docker.NewManager()
		if err != nil {
			log.Fatalf("Failed to initialize docker manager: %v", err)
		}

		ctx := context.Background()
		fmt.Printf("Updating limits for runner '%s'...\n", name)

		// Apply to Docker container if it exists
		if runner.ContainerID != "" {
			if err := dm.UpdateResources(ctx, runner.ContainerID, runner.CPULimit, runner.MemoryLimit); err != nil {
				log.Printf("Warning: failed to update running container resources: %v", err)
				fmt.Println("The new limits will be applied the next time the runner starts.")
			} else {
				fmt.Println("Successfully updated running container limits.")
			}
		}

		// Save to config
		if err := config.UpdateRunner(runner); err != nil {
			log.Fatalf("Failed to save updated config: %v", err)
		}

		fmt.Printf("Runner '%s' configuration updated.\n", name)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().Float64Var(&updateCPU, "cpu", 0, "New CPU limit in cores")
	updateCmd.Flags().Int64Var(&updateMemory, "memory", 0, "New Memory limit in MB")
}
