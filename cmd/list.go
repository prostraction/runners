package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/runners-manager/config"
	"github.com/runners-manager/docker"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured GitHub runners",
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

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tURL\tLABELS\tSTATUS")

		for _, r := range cfg.Runners {
			status := "Stopped"
			isRunning, _ := dm.IsRunning(ctx, r.ContainerID)
			if isRunning {
				status = "Running"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Name, r.URL, r.Labels, status)
		}
		w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
