package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/runners/config"
	"github.com/runners/docker"
	"github.com/spf13/cobra"
)

// verifyStartupTimeout is how long we wait for a newly added runner to register
// with GitHub and start its listener process. First-time registration on a slow
// host (or when the myoung34 image is still warming up) can take ~20-30s, so
// 60s gives comfortable headroom without blocking the user forever on a real
// failure.
const verifyStartupTimeout = 60 * time.Second

var (
	addName   string
	addURL    string
	addToken  string
	addLabels string
	addCPU    float64
	addMemory int64
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add and start a new GitHub runner",
	RunE: func(cmd *cobra.Command, args []string) error {
		runner := &config.Runner{
			Name:        addName,
			URL:         addURL,
			Token:       addToken,
			Labels:      addLabels,
			CPULimit:    addCPU,
			MemoryLimit: addMemory,
		}

		// Reject stale data from a previous runner of the same name. Leftover
		// credentials under the data dir would make the container come up
		// "successfully" while failing to register with GitHub.
		if config.DataDirExists(runner.Name) {
			return fmt.Errorf("stale data directory exists for '%s' at %s; remove it first (e.g. 'runners rm %s' on an older install, or delete the directory manually)", runner.Name, config.DataDir(runner.Name), runner.Name)
		}

		// First add to config to ensure name is unique
		if err := config.AddRunner(runner); err != nil {
			return fmt.Errorf("failed to add runner to config: %w", err)
		}

		dm, err := docker.NewManager()
		if err != nil {
			_ = config.RemoveRunner(runner.Name)
			return fmt.Errorf("failed to initialize docker manager: %w", err)
		}

		ctx := context.Background()

		// Ensure image is pulled
		if err := dm.PullImage(ctx); err != nil {
			fmt.Printf("Warning: failed to pull image, will try to use local: %v\n", err)
		}

		if err := dm.StartRunner(ctx, runner); err != nil {
			_ = config.RemoveRunner(runner.Name)
			_ = dm.PurgeDataDir(ctx, runner.Name)
			return fmt.Errorf("failed to start runner container: %w", err)
		}

		// Update config with container ID before verifying, so state is
		// recoverable if the user ctrl-c's during verification.
		if err := config.UpdateRunner(runner); err != nil {
			log.Printf("Warning: failed to save container ID to config: %v", err)
		}

		fmt.Printf("Waiting for runner '%s' to register with GitHub...\n", runner.Name)
		if err := dm.VerifyStartup(ctx, runner.ContainerID, verifyStartupTimeout); err != nil {
			_ = dm.StopRunner(ctx, runner.ContainerID)
			_ = dm.RemoveRunner(ctx, runner.ContainerID)
			_ = config.RemoveRunner(runner.Name)
			_ = dm.PurgeDataDir(ctx, runner.Name)
			return fmt.Errorf("runner failed to register: %w", err)
		}

		fmt.Printf("Successfully added and started runner '%s'!\n", runner.Name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&addName, "name", "n", "", "Name of the runner (required)")
	addCmd.Flags().StringVarP(&addURL, "url", "u", "", "URL of the GitHub repository or organization (required)")
	addCmd.Flags().StringVarP(&addToken, "token", "t", "", "Runner registration token (required)")
	addCmd.Flags().StringVarP(&addLabels, "labels", "l", "", "Optional custom labels (comma-separated)")
	addCmd.Flags().Float64Var(&addCPU, "cpu", 0, "CPU limit in cores (e.g. 0.5 or 2)")
	addCmd.Flags().Int64Var(&addMemory, "memory", 0, "Memory limit in MB (e.g. 512 or 2048) — aliases: --mem, --ram")
	addCmd.Flags().SetNormalizeFunc(normalizeMemoryAliases)
	if err := addCmd.MarkFlagRequired("name"); err != nil {
		panic(err)
	}
	if err := addCmd.MarkFlagRequired("url"); err != nil {
		panic(err)
	}
	if err := addCmd.MarkFlagRequired("token"); err != nil {
		panic(err)
	}
}
