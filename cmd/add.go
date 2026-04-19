package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/runners-manager/config"
	"github.com/runners-manager/docker"
	"github.com/spf13/cobra"
)

var (
	addName   string
	addURL    string
	addToken  string
	addLabels string
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add and start a new GitHub runner",
	Run: func(cmd *cobra.Command, args []string) {
		runner := &config.Runner{
			Name:   addName,
			URL:    addURL,
			Token:  addToken,
			Labels: addLabels,
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
	addCmd.MarkFlagRequired("name")
	addCmd.MarkFlagRequired("url")
	addCmd.MarkFlagRequired("token")
}
