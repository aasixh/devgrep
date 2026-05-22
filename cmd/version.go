package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// Version is replaced by release builds.
	Version = "0.1.0-dev"
	// Commit is replaced by release builds.
	Commit = "none"
	// Date is replaced by release builds.
	Date = "unknown"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "devgrep %s\ncommit %s\nbuilt %s\ngo %s\n", Version, Commit, Date, runtime.Version())
		},
	}
}
