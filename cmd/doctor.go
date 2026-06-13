package cmd

import (
	"fmt"

	"github.com/aasixh/devgrep/internal/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local devgrep health",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig()
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "%-16s %-6s %s\n", "config", "fail", "invalid YAML or unreadable config: "+err.Error())
				return nil
			}
			store, err := openStore(ctx, cfg)
			if err != nil {
				return err
			}
			defer store.Close()
			checks := doctor.Run(ctx, cfg, store)
			doctor.Print(cmd.OutOrStdout(), checks, terminalOutput())
			return nil
		},
	}
}
