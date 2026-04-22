package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/runners/config"
	"github.com/runners/docker"
	"github.com/spf13/cobra"
)

var (
	logFollow bool
	logTail   string
)

var logCmd = &cobra.Command{
	Use:     "log [name]",
	Aliases: []string{"logs"},
	Short:   "Show container logs for a GitHub runner",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		name := args[0]
		runner, exists := cfg.Runners[name]
		if !exists {
			return fmt.Errorf("runner '%s' not found", name)
		}
		if runner.ContainerID == "" {
			return fmt.Errorf("runner '%s' has no container yet", name)
		}

		dm, err := docker.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize docker manager: %w", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if logFollow {
			// Cancel the stream on Ctrl-C so the reader unblocks cleanly.
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, os.Interrupt)
			go func() {
				<-sig
				cancel()
			}()
		}

		return dm.StreamLogs(ctx, runner.ContainerID, logFollow, logTail, os.Stdout, os.Stderr)
	},
}

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().BoolVarP(&logFollow, "follow", "f", false, "Stream logs continuously (Ctrl-C to stop)")
	logCmd.Flags().StringVar(&logTail, "tail", "100", "Number of lines to show from the end, or 'all'")
}
