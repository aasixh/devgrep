package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devgrep/devgrep/internal/config"
	"github.com/devgrep/devgrep/internal/storage"
)

func TestRootVersionCommand(t *testing.T) {
	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("devgrep")) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestWatchIndexCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := config.Default()
	cfg.IndexedPaths = []string{t.TempDir()}
	var out bytes.Buffer
	if err := watchIndex(ctx, cfg, nil, nil, nil, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("watching")) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestIndexSearchStatsDoctorCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), []byte("#1700000000\ndocker compose up -d postgres\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, "config.yaml")
	db := filepath.Join(home, "devgrep.db")
	run := func(args ...string) string {
		t.Helper()
		var out bytes.Buffer
		root := NewRootCommand()
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(append([]string{"--plain", "--config", cfg, "--db", db}, args...))
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if out := run("index", "--source", "history"); !bytes.Contains([]byte(out), []byte("shell-history")) {
		t.Fatalf("index output = %s", out)
	}
	if out := run("search", "docker", "postgres"); !bytes.Contains([]byte(out), []byte("docker compose")) {
		t.Fatalf("search output = %s", out)
	}
	if out := run("stats"); !bytes.Contains([]byte(out), []byte("documents")) {
		t.Fatalf("stats output = %s", out)
	}
	if out := run("doctor"); !bytes.Contains([]byte(out), []byte("database")) {
		t.Fatalf("doctor output = %s", out)
	}
	if out := run("sources"); !bytes.Contains([]byte(out), []byte("[history]")) {
		t.Fatalf("sources output = %s", out)
	}
	if out := run("sources", "--tree"); !bytes.Contains([]byte(out), []byte(".bash_history")) && !bytes.Contains([]byte(out), []byte("No indexed")) {
		t.Fatalf("sources tree output = %s", out)
	}
}

func TestDirectSearchFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := NewRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	args := []string{"--plain", "--db", filepath.Join(home, "devgrep.db"), "postgres", "timeout"}
	applyDirectSearchFallback(root, args)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	known := NewRootCommand()
	applyDirectSearchFallback(known, []string{"stats"})
	known.SetArgs([]string{"version"})
	if err := known.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestDangerousIndexingPrevention(t *testing.T) {
	if err := confirmSafeIndexPaths([]string{"/"}, true, false, bytes.NewBuffer(nil), &bytes.Buffer{}); err == nil {
		t.Fatal("expected root path to be rejected without confirmation")
	}
	if err := confirmSafeIndexPaths([]string{"/"}, true, true, bytes.NewBuffer(nil), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

func TestDryRunIndex(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# note\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.log"), []byte("INFO ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "skip.log"), []byte("INFO skip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Indexing.AutoWatch = false
	var out bytes.Buffer
	if err := runDryRun(context.Background(), cfg, []string{"logs", "notes"}, []string{root}, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Dry run")) || !bytes.Contains(out.Bytes(), []byte("node_modules")) {
		t.Fatalf("dry run output = %s", out.String())
	}
}

func TestPrintSources(t *testing.T) {
	locations := []storage.SourceLocation{
		{Type: "history", Path: "/home/alice/.bash_history"},
		{Type: "note", Path: "/home/alice/projects/devgrep/docs/config.md"},
		{Type: "log", Path: "/home/alice/projects/api/logs/app.log"},
	}
	var out bytes.Buffer
	printSources(&out, locations)
	if !bytes.Contains(out.Bytes(), []byte("[history]")) || !bytes.Contains(out.Bytes(), []byte("[notes]")) || !bytes.Contains(out.Bytes(), []byte("[logs]")) {
		t.Fatalf("sources output = %s", out.String())
	}
	out.Reset()
	printSourcesTree(&out, locations)
	if !bytes.Contains(out.Bytes(), []byte("projects")) {
		t.Fatalf("tree output = %s", out.String())
	}
}
