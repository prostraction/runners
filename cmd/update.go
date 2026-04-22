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
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Check if any changes were specified (mem/ram are normalized to memory).
		updated := false
		if cmd.Flags().Changed("cpu") || cmd.Flags().Changed("memory") {
			updated = true
		}

		if !updated {
			fmt.Println("No changes specified. Use --cpu or --memory to update limits.")
			return nil
		}

		dm, err := docker.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize docker manager: %w", err)
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
				if cmd.Flags().Changed("memory") {
					runner.MemoryLimit = updateMemory
				}

				if runner.ContainerID != "" {
					if err := dm.UpdateResources(ctx, runner.ContainerID, runner.CPULimit, runner.MemoryLimit); err != nil {
						log.Printf("Warning: failed to update runner '%s' container: %v", name, err)
					}
				}

				if err := config.UpdateRunner(runner); err != nil {
					log.Printf("Error: failed to update '%s' in config: %v", name, err)
				}
			}
			fmt.Println("All runners updated.")
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

		fmt.Printf("Updating limits for runner '%s'...\n", name)

		if cmd.Flags().Changed("cpu") {
			runner.CPULimit = updateCPU
		}
		if cmd.Flags().Changed("memory") {
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
			return fmt.Errorf("failed to save updated config: %w", err)
		}

		fmt.Printf("Runner '%s' configuration updated.\n", name)
		return nil
	},
}
func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().Float64Var(&updateCPU, "cpu", 0, "New CPU limit in cores")
	updateCmd.Flags().Int64Var(&updateMemory, "memory", 0, "New Memory limit in MB — aliases: --mem, --ram")
	updateCmd.Flags().BoolVarP(&updateAll, "all", "a", false, "Update all runners")
	updateCmd.Flags().SetNormalizeFunc(normalizeMemoryAliases)
}
