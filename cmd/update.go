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
	updateCPU    float64
	updateMemory int64
	updateAll    bool
)

var updateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Update resource limits for one or all runners",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}

		// Check if any changes were specified
		updated := false
		if cmd.Flags().Changed("cpu") || cmd.Flags().Changed("memory") || cmd.Flags().Changed("mem") || cmd.Flags().Changed("ram") {
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

		if updateAll {
			fmt.Println("Updating limits for all runners...")
			names := make([]string, 0, len(cfg.Runners))
			for name := range cfg.Runners {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				runner := cfg.Runners[name]
				fmt.Printf("Updating runner '%s'...\n", name)

				if cmd.Flags().Changed("cpu") {
					runner.CPULimit = updateCPU
				}
				if cmd.Flags().Changed("memory") || cmd.Flags().Changed("mem") || cmd.Flags().Changed("ram") {
					runner.MemoryLimit = updateMemory
				}

				if runner.ContainerID != "" {
					if err := dm.UpdateResources(ctx, runner.ContainerID, runner.CPULimit, runner.MemoryLimit); err != nil {
						log.Printf("Warning: failed to update runner '%s' container: %v", name, err)
					}
				}

				if err := config.UpdateRunner(runner); err != nil {
					log.Printf("Error: failed to remove '%s' from config: %v", name, err)
				}
			}
			fmt.Println("All runners updated.")
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

		fmt.Printf("Updating limits for runner '%s'...\n", name)

		if cmd.Flags().Changed("cpu") {
			runner.CPULimit = updateCPU
		}
		if cmd.Flags().Changed("memory") || cmd.Flags().Changed("mem") || cmd.Flags().Changed("ram") {
			runner.MemoryLimit = updateMemory
		}


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
	updateCmd.Flags().Int64Var(&updateMemory, "mem", 0, "Alias for --memory")
	updateCmd.Flags().Int64Var(&updateMemory, "ram", 0, "Alias for --memory")
	updateCmd.Flags().BoolVarP(&updateAll, "all", "a", false, "Update all runners")
}
