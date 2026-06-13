package cmd

import (
	"fmt"
	"sort"

	"github.com/aasixh/devgrep/internal/utils"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func newStatsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show indexed data and search statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			store, err := openStore(ctx, cfg)
			if err != nil {
				return err
			}
			defer store.Close()
			stats, err := store.Stats(ctx)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			title := "devgrep stats"
			if terminalOutput() {
				title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cfg.TUI.Accent)).Render(title)
			}
			fmt.Fprintln(out, title)
			fmt.Fprintf(out, "documents       %d\n", stats.TotalDocuments)
			fmt.Fprintf(out, "database        %s\n", utils.HumanBytes(stats.DBSizeBytes))

			keys := make([]string, 0, len(stats.BySource))
			for key := range stats.BySource {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(out, "%-15s %d\n", key, stats.BySource[key])
			}
			if len(stats.TopSearches) > 0 {
				fmt.Fprintln(out, "\nmost searched")
				for _, item := range stats.TopSearches {
					fmt.Fprintf(out, "%-5d %s\n", item.Count, item.Query)
				}
			}
			if len(stats.LastRuns) > 0 {
				fmt.Fprintln(out, "\nrecent index runs")
				for _, run := range stats.LastRuns {
					status := "ok"
					if run.Error != "" {
						status = run.Error
					}
					fmt.Fprintf(out, "%-16s %-6d %-8s %s\n", run.SourceName, run.Indexed, run.Duration, status)
				}
			}
			return nil
		},
	}
}
