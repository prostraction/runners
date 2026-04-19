package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/runners-manager/config"
	"github.com/runners-manager/docker"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Remove a GitHub runner",
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

		dm, err := docker.NewManager()
		if err != nil {
			log.Fatalf("Failed to initialize docker manager: %v", err)
		}

		ctx := context.Background()
		fmt.Printf("Stopping and removing runner '%s'...\n", name)
		
		if err := dm.StopRunner(ctx, runner.ContainerID); err != nil {
			log.Printf("Warning: failed to stop container: %v", err)
		}
		
		if err := dm.RemoveRunner(ctx, runner.ContainerID); err != nil {
			log.Printf("Warning: failed to remove container: %v", err)
		}

		if err := config.RemoveRunner(name); err != nil {
			log.Fatalf("Failed to remove runner from config: %v", err)
		}

		fmt.Printf("Successfully removed runner '%s'.\n", name)
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
