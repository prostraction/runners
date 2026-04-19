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
	Run: func(cmd *cobra.Command, args []string) {
		runner := &config.Runner{
			Name:        addName,
			URL:         addURL,
			Token:       addToken,
			Labels:      addLabels,
			CPULimit:    addCPU,
			MemoryLimit: addMemory,
		}

		// First add to config to ensure name is unique
		if err := config.AddRunner(runner); err != nil {
			log.Fatalf("Failed to add runner to config: %v", err)
		}

		dm, err := docker.NewManager()
		if err != nil {
			config.RemoveRunner(runner.Name)
			log.Fatalf("Failed to initialize docker manager: %v", err)
		}

		ctx := context.Background()

		// Ensure image is pulled
		if err := dm.PullImage(ctx); err != nil {
			fmt.Printf("Warning: failed to pull image, will try to use local: %v\n", err)
		}

		if err := dm.StartRunner(ctx, runner); err != nil {
			config.RemoveRunner(runner.Name)
			log.Fatalf("Failed to start runner container: %v", err)
		}

		// Update config with container ID
		if err := config.UpdateRunner(runner); err != nil {
			log.Printf("Warning: failed to save container ID to config: %v", err)
		}

		fmt.Printf("Successfully added and started runner '%s'!\n", runner.Name)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&addName, "name", "n", "", "Name of the runner (required)")
	addCmd.Flags().StringVarP(&addURL, "url", "u", "", "URL of the GitHub repository or organization (required)")
	addCmd.Flags().StringVarP(&addToken, "token", "t", "", "Runner registration token (required)")
	addCmd.Flags().StringVarP(&addLabels, "labels", "l", "", "Optional custom labels (comma-separated)")
	addCmd.Flags().Float64Var(&addCPU, "cpu", 0, "CPU limit in cores (e.g. 0.5 or 2)")
	addCmd.Flags().Int64Var(&addMemory, "memory", 0, "Memory limit in MB (e.g. 512 or 2048)")
	addCmd.Flags().Int64Var(&addMemory, "mem", 0, "Alias for --memory")
	addCmd.Flags().Int64Var(&addMemory, "ram", 0, "Alias for --memory")
	addCmd.MarkFlagRequired("name")
	addCmd.MarkFlagRequired("url")
	addCmd.MarkFlagRequired("token")
}
