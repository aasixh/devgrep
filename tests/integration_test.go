package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/aasixh/devgrep/cmd"
)

func TestIndexHistoryCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	historyPath := filepath.Join(home, ".bash_history")
	if err := os.WriteFile(historyPath, []byte("#1700000000\ndocker compose up -d postgres\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	root := cmd.NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--config", filepath.Join(home, ".config", "devgrep", "config.yaml"),
		"--db", filepath.Join(home, ".local", "share", "devgrep", "devgrep.db"),
		"index",
		"--source", "history",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("shell-history")) {
		t.Fatalf("output did not mention shell-history: %s", out.String())
	}
}
