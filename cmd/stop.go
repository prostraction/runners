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
	Use:   "stop [name]",
	Short: "Stop one or all running GitHub runners",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		dm, err := docker.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize docker manager: %w", err)
		}

		ctx := context.Background()

		if stopAll {
			fmt.Println("Stopping all runners...")
			
			names := make([]string, 0, len(cfg.Runners))
			for name := range cfg.Runners {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				runner := cfg.Runners[name]
				fmt.Printf("Stopping runner '%s'...\n", name)
				if err := dm.StopRunner(ctx, runner.ContainerID); err != nil {
					log.Printf("Warning: failed to stop runner '%s': %v", name, err)
				}
			}
			fmt.Println("All runners stopped.")
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

		fmt.Printf("Stopping runner '%s'...\n", name)
		if err := dm.StopRunner(ctx, runner.ContainerID); err != nil {
			return fmt.Errorf("failed to stop runner: %w", err)
		}

		fmt.Printf("Successfully stopped runner '%s'.\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().BoolVarP(&stopAll, "all", "a", false, "Stop all runners")
}
