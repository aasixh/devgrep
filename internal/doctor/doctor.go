package doctor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devgrep/devgrep/internal/config"
	"github.com/devgrep/devgrep/internal/storage"
	"github.com/devgrep/devgrep/internal/utils"
)

const (
	// StatusOK means a check passed.
	StatusOK = "ok"
	// StatusWarn means a check found a non-fatal issue.
	StatusWarn = "warn"
	// StatusFail means a check failed.
	StatusFail = "fail"
)

// Check is one doctor diagnostic.
type Check struct {
	Name    string
	Status  string
	Message string
}

// Run executes local environment diagnostics.
func Run(ctx context.Context, cfg config.Config, store *storage.Store) []Check {
	checks := []Check{
		checkConfig(cfg),
		checkConfigValues(cfg),
		checkDatabase(ctx, store),
		checkDatabaseDirectory(cfg),
	}
	checks = append(checks, checkHistoryFiles()...)
	checks = append(checks, checkIndexedPaths(cfg)...)
	return checks
}

func checkConfigValues(cfg config.Config) Check {
	issues := config.Validate(cfg)
	if len(issues) == 0 {
		return Check{Name: "config values", Status: StatusOK, Message: "valid"}
	}
	return Check{Name: "config values", Status: StatusWarn, Message: strings.Join(issues, "; ")}
}

func checkConfig(cfg config.Config) Check {
	if _, err := os.Stat(cfg.ConfigPath); errors.Is(err, os.ErrNotExist) {
		return Check{Name: "config", Status: StatusOK, Message: "using built-in defaults; config file not required"}
	} else if err != nil {
		return Check{Name: "config", Status: StatusFail, Message: err.Error()}
	}
	return Check{Name: "config", Status: StatusOK, Message: utils.RelHome(cfg.ConfigPath)}
}

func checkDatabase(ctx context.Context, store *storage.Store) Check {
	if store == nil {
		return Check{Name: "database", Status: StatusFail, Message: "not open"}
	}
	if err := store.IntegrityCheck(ctx); err != nil {
		return Check{Name: "database", Status: StatusFail, Message: err.Error()}
	}
	return Check{Name: "database", Status: StatusOK, Message: utils.RelHome(store.Path())}
}

func checkDatabaseDirectory(cfg config.Config) Check {
	dir := filepath.Dir(cfg.DatabasePath)
	test := filepath.Join(dir, ".devgrep-write-test")
	if err := utils.EnsureDir(dir); err != nil {
		return Check{Name: "permissions", Status: StatusFail, Message: err.Error()}
	}
	if err := os.WriteFile(test, []byte("ok"), 0o600); err != nil {
		return Check{Name: "permissions", Status: StatusFail, Message: err.Error()}
	}
	_ = os.Remove(test)
	return Check{Name: "permissions", Status: StatusOK, Message: "database directory is writable"}
}

func checkHistoryFiles() []Check {
	home := utils.MustHome()
	sources := []struct {
		name string
		path string
	}{
		{name: "bash history", path: filepath.Join(home, ".bash_history")},
		{name: "zsh history", path: filepath.Join(home, ".zsh_history")},
	}
	checks := make([]Check, 0, len(sources))
	for _, source := range sources {
		if _, err := os.Stat(source.path); errors.Is(err, os.ErrNotExist) {
			checks = append(checks, Check{Name: source.name, Status: StatusWarn, Message: "not found"})
		} else if err != nil {
			checks = append(checks, Check{Name: source.name, Status: StatusFail, Message: err.Error()})
		} else {
			checks = append(checks, Check{Name: source.name, Status: StatusOK, Message: utils.RelHome(source.path)})
		}
	}
	return checks
}

func checkIndexedPaths(cfg config.Config) []Check {
	checks := make([]Check, 0, len(cfg.IndexedPaths))
	for _, path := range cfg.IndexedPaths {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			checks = append(checks, Check{Name: "indexed path", Status: StatusWarn, Message: utils.RelHome(path) + " not found"})
		} else if err != nil {
			checks = append(checks, Check{Name: "indexed path", Status: StatusFail, Message: err.Error()})
		} else {
			checks = append(checks, Check{Name: "indexed path", Status: StatusOK, Message: utils.RelHome(path)})
		}
	}
	return checks
}

// Print renders diagnostics for terminal output.
func Print(w io.Writer, checks []Check, styled bool) {
	statusStyle := map[string]lipgloss.Style{
		StatusOK:   lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Bold(true),
		StatusWarn: lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Bold(true),
		StatusFail: lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Bold(true),
	}
	for _, check := range checks {
		status := check.Status
		if styled {
			status = statusStyle[check.Status].Render(status)
		}
		fmt.Fprintf(w, "%-16s %-6s %s\n", check.Name, status, check.Message)
	}
}
