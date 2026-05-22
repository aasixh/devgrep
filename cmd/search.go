package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/devgrep/devgrep/internal/logs"
	searchpkg "github.com/devgrep/devgrep/internal/search"
	"github.com/devgrep/devgrep/internal/tui"
	"github.com/spf13/cobra"
)

func newSearchCommand() *cobra.Command {
	var limit int
	var sources []string
	var interactive bool
	var tail bool
	var regex string
	var severity string
	var paths []string

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search developer workflows",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			query := strings.TrimSpace(strings.Join(args, " "))
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if tail {
				pattern := regex
				if pattern == "" {
					pattern = query
				}
				return logs.Tail(ctx, cfg, logs.TailOptions{Paths: paths, Pattern: pattern, Severity: severity}, cmd.OutOrStdout())
			}

			store, err := openStore(ctx, cfg)
			if err != nil {
				return err
			}
			defer store.Close()

			sourceTypes, err := searchSources(sources)
			if err != nil {
				return err
			}
			engine := searchpkg.Engine{Store: store, Config: cfg}
			opts := searchpkg.Options{Limit: limit, SourceTypes: sourceTypes, Record: true}
			if interactive || terminalOutput() {
				return tui.Run(ctx, engine, cfg, query, opts)
			}
			results, err := engine.Query(ctx, query, opts)
			if err != nil {
				return err
			}
			searchpkg.PrintPlain(cmd.OutOrStdout(), results, time.Now())
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "maximum results")
	cmd.Flags().StringSliceVar(&sources, "source", []string{"all"}, "sources to search: all, history, logs, notes")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "force interactive TUI")
	cmd.Flags().BoolVar(&tail, "tail", false, "tail matching log lines")
	cmd.Flags().StringVar(&regex, "regex", "", "regular expression for log tail filtering")
	cmd.Flags().StringVar(&severity, "severity", "", "log severity filter: DEBUG, INFO, WARN, ERROR")
	cmd.Flags().StringArrayVar(&paths, "path", nil, "path to search or tail")
	return cmd
}

func searchSources(sources []string) ([]string, error) {
	sourceSet, err := normalizeSources(sources)
	if err != nil {
		return nil, err
	}
	var sourceTypes []string
	if sourceSet["history"] {
		sourceTypes = append(sourceTypes, searchpkg.SourceHistory)
	}
	if sourceSet["logs"] {
		sourceTypes = append(sourceTypes, searchpkg.SourceLog)
	}
	if sourceSet["notes"] {
		sourceTypes = append(sourceTypes, searchpkg.SourceNote)
	}
	if len(sourceTypes) == 0 {
		return nil, fmt.Errorf("no search sources selected")
	}
	return sourceTypes, nil
}
