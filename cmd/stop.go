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

		fmt.Printf("Stopping runner '%s'...\n", name)
		if err := dm.StopRunner(ctx, runner.ContainerID); err != nil {
			log.Fatalf("Failed to stop runner: %v", err)
		}

		fmt.Printf("Successfully stopped runner '%s'.\n", name)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().BoolVarP(&stopAll, "all", "a", false, "Stop all runners")
}
