package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/aasixh/devgrep/internal/config"
	"github.com/aasixh/devgrep/internal/storage"
	"github.com/aasixh/devgrep/internal/utils"
	"github.com/spf13/cobra"
)

var (
	cfgPath string
	dbPath  string
	plain   bool
	logger  *slog.Logger
)

// Execute builds the command tree and runs devgrep with the supplied context.
func Execute(ctx context.Context) error {
	root := NewRootCommand()
	applyDirectSearchFallback(root, os.Args[1:])
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(root.ErrOrStderr(), utils.FormatError(utils.HumanizeError(err)))
		return err
	}
	return nil
}

// NewRootCommand returns a fully wired cobra command tree.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "devgrep",
		Short:         "grep for developer workflows",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
			if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
				logger = slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: slog.LevelDebug}))
			}
		},
	}

	root.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path")
	root.PersistentFlags().StringVar(&dbPath, "db", "", "database file path")
	root.PersistentFlags().BoolVar(&plain, "plain", false, "disable interactive and styled output")
	root.PersistentFlags().Bool("verbose", false, "enable debug logging")

	root.SetVersionTemplate("devgrep {{.Version}}\n")
	root.AddCommand(newSearchCommand())
	root.AddCommand(newIndexCommand())
	root.AddCommand(newDoctorCommand())
	root.AddCommand(newStatsCommand())
	root.AddCommand(newSourcesCommand())
	root.AddCommand(newVersionCommand())

	return root
}

func applyDirectSearchFallback(root *cobra.Command, args []string) {
	if len(args) == 0 {
		return
	}
	first := firstNonFlagArg(args)
	if first == "" || hasKnownCommandArg(root, args) || isKnownCommand(root, first) {
		return
	}
	root.SetArgs(append([]string{"search"}, args...))
}

func firstNonFlagArg(args []string) string {
	for _, arg := range args {
		if arg == "--" {
			return ""
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return ""
}

func hasKnownCommandArg(root *cobra.Command, args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if isKnownCommand(root, arg) {
			return true
		}
	}
	return false
}

func isKnownCommand(root *cobra.Command, name string) bool {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return true
		}
		for _, alias := range cmd.Aliases {
			if alias == name {
				return true
			}
		}
	}
	return false
}

func loadConfig() (config.Config, error) {
	return config.Load(cfgPath)
}

func openStore(ctx context.Context, cfg config.Config) (*storage.Store, error) {
	path := dbPath
	if path == "" {
		path = cfg.DatabasePath
	}
	return storage.Open(ctx, path)
}

func terminalOutput() bool {
	return !plain && utils.IsTerminal(os.Stdout)
}
