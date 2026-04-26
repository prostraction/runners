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
	stopAll bool
)

var stopCmd = &cobra.Command{
	Use:   "stop [name...]",
	Short: "Stop one or more (or all) running GitHub runners",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if !stopAll && len(args) == 0 {
			return fmt.Errorf("please specify one or more runner names or use --all")
		}

		dm, err := docker.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize docker manager: %w", err)
		}

		ctx := context.Background()

		names := resolveTargets(cfg, args, stopAll)

		for _, name := range names {
			runner, exists := cfg.Runners[name]
			if !exists {
				log.Printf("Runner '%s' not found, skipping", name)
				continue
			}
			fmt.Printf("Stopping runner '%s'...\n", name)
			if err := dm.StopRunner(ctx, runner.ContainerID); err != nil {
				log.Printf("Warning: failed to stop runner '%s': %v", name, err)
			}
		}

		return nil
	},
}

// resolveTargets returns the list of runner names to act on. When all is true
// every configured runner is returned in sorted order; otherwise the names
// the user passed on the command line are returned verbatim.
func resolveTargets(cfg *config.Config, args []string, all bool) []string {
	if !all {
		return args
	}
	names := make([]string, 0, len(cfg.Runners))
	for name := range cfg.Runners {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().BoolVarP(&stopAll, "all", "a", false, "Stop all runners")
}
