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
	startAll bool
)

var startCmd = &cobra.Command{
	Use:   "start [name...]",
	Short: "Start one or more (or all) stopped GitHub runners",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if !startAll && len(args) == 0 {
			return fmt.Errorf("please specify one or more runner names or use --all")
		}

		dm, err := docker.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize docker manager: %w", err)
		}

		ctx := context.Background()

		names := resolveTargets(cfg, args, startAll)

		for _, name := range names {
			runner, exists := cfg.Runners[name]
			if !exists {
				log.Printf("Runner '%s' not found, skipping", name)
				continue
			}

			info, _ := dm.GetRunnerInfo(ctx, runner.ContainerID)
			if info != nil && info.IsRunning {
				fmt.Printf("Runner '%s' is already running.\n", name)
				continue
			}

			fmt.Printf("Starting runner '%s'...\n", name)

			if runner.ContainerID != "" && info != nil && info.ExitCode == 0 {
				if err := dm.ResumeRunner(ctx, runner.ContainerID); err == nil {
					if err := dm.EnsureRestartPolicy(ctx, runner.ContainerID); err != nil {
						log.Printf("Warning: failed to set restart policy for '%s': %v", name, err)
					}
					fmt.Printf("Runner '%s' resumed.\n", name)
					continue
				}
			}

			fmt.Printf("Re-creating runner '%s'...\n", name)
			_ = dm.RemoveRunner(ctx, runner.ContainerID)

			if err := dm.StartRunner(ctx, runner); err != nil {
				log.Printf("Error: failed to start runner '%s': %v", name, err)
				continue
			}
			if err := config.UpdateRunner(runner); err != nil {
				log.Printf("Warning: failed to save container ID for '%s' to config: %v", name, err)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolVarP(&startAll, "all", "a", false, "Start all runners")
}
