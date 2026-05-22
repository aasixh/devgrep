package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsWithoutFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg, err := Load(filepath.Join(home, "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabasePath == "" {
		t.Fatal("database path was empty")
	}
	if len(cfg.IndexedPaths) == 0 {
		t.Fatal("indexed paths were empty")
	}
}

func TestEnsureDefaultFileAndLoad(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := Default()
	cfg.ConfigPath = filepath.Join(home, ".config", "devgrep", "config.yaml")
	if err := EnsureDefaultFile(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg.ConfigPath); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TUI.Accent == "" {
		t.Fatal("theme accent was empty")
	}
}

func TestValidateConfigIssues(t *testing.T) {
	cfg := Default()
	cfg.DatabasePath = ""
	cfg.IndexedPaths = []string{filepath.Join(t.TempDir(), "missing")}
	cfg.History.Limit = -1
	cfg.Logs.MaxFileSizeMB = -1
	cfg.Logs.Extensions = nil
	cfg.Indexing.MaxFiles = -1
	cfg.Indexing.MaxFileSizeMB = -1
	cfg.Ranking.Fuzzy = -1
	issues := Validate(cfg)
	if len(issues) < 6 {
		t.Fatalf("issues = %#v", issues)
	}
}
