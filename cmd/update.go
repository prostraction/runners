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
	updateAll    bool
)

var updateCmd = &cobra.Command{
	Use:   "update [name...]",
	Short: "Update resource limits for one or more (or all) runners",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if !cmd.Flags().Changed("cpu") && !cmd.Flags().Changed("memory") {
			fmt.Println("No changes specified. Use --cpu or --memory to update limits.")
			return nil
		}

		if !updateAll && len(args) == 0 {
			return fmt.Errorf("please specify one or more runner names or use --all")
		}

		dm, err := docker.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize docker manager: %w", err)
		}

		ctx := context.Background()

		names := resolveTargets(cfg, args, updateAll)

		for _, name := range names {
			runner, exists := cfg.Runners[name]
			if !exists {
				log.Printf("Runner '%s' not found, skipping", name)
				continue
			}
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
