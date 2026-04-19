package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/runners/config"
	"github.com/runners/docker"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured GitHub runners",
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

		// Get and sort names
		names := make([]string, 0, len(cfg.Runners))
		for name := range cfg.Runners {
			names = append(names, name)
		}
		sort.Strings(names)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tURL\tLABELS\tSTATUS\tUPTIME\tERRORS\tCPU\tRAM")

		for _, name := range names {
			r := cfg.Runners[name]
			info, err := dm.GetRunnerInfo(ctx, r.ContainerID)
			if err != nil {
				fmt.Fprintf(w, "%s\t%s\t%s\tError\t-\t%d\t-\t-\n", r.Name, r.URL, r.Labels, r.ErrorCount)
				continue
			}

			status := "Stopped"
			if info.IsRunning {
				status = fmt.Sprintf("Running (%s)", info.InternalStatus)
			} else if info.ExitCode != 0 {
				status = fmt.Sprintf("Exited (%d)", info.ExitCode)
			}

			cpuLimit := "-"
			if r.CPULimit > 0 {
				cpuLimit = fmt.Sprintf("%.1f", r.CPULimit)
			}
			memLimit := "-"
			if r.MemoryLimit > 0 {
				memLimit = fmt.Sprintf("%dMB", r.MemoryLimit)
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n", r.Name, r.URL, r.Labels, status, info.Uptime, r.ErrorCount, cpuLimit, memLimit)
		}
		if err := w.Flush(); err != nil {
			log.Printf("Warning: failed to flush output: %v", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
