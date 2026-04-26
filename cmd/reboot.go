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
	rebootAll bool
)

var rebootCmd = &cobra.Command{
	Use:     "reboot [name...]",
	Aliases: []string{"restart"},
	Short:   "Reboot (restart) one or more (or all) GitHub runners",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if !rebootAll && len(args) == 0 {
			return fmt.Errorf("please specify one or more runner names or use --all")
		}

		dm, err := docker.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize docker manager: %w", err)
		}

		ctx := context.Background()

		names := resolveTargets(cfg, args, rebootAll)

		for _, name := range names {
			runner, exists := cfg.Runners[name]
			if !exists {
				log.Printf("Runner '%s' not found, skipping", name)
				continue
			}
			fmt.Printf("Rebooting runner '%s'...\n", name)

			infoBefore, _ := dm.GetRunnerInfo(ctx, runner.ContainerID)
			healthy := infoBefore != nil && (infoBefore.IsRunning || infoBefore.ExitCode == 0)

			if err := dm.StopRunner(ctx, runner.ContainerID); err != nil {
				log.Printf("Warning during stop for '%s': %v", name, err)
			}

			if healthy {
				if err := dm.ResumeRunner(ctx, runner.ContainerID); err == nil {
					if err := dm.EnsureRestartPolicy(ctx, runner.ContainerID); err != nil {
						log.Printf("Warning: failed to set restart policy for '%s': %v", name, err)
					}
					continue
				}
			}

			fmt.Printf("Re-creating runner '%s'...\n", name)
			_ = dm.RemoveRunner(ctx, runner.ContainerID)
			if err := dm.StartRunner(ctx, runner); err != nil {
				log.Printf("Error: failed to restart runner '%s': %v", name, err)
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
	rootCmd.AddCommand(rebootCmd)
	rebootCmd.Flags().BoolVarP(&rebootAll, "all", "a", false, "Reboot all runners")
}
