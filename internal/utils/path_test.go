package utils

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestPathHelpers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	expanded, err := ExpandHome("~/project")
	if err != nil {
		t.Fatal(err)
	}
	if expanded != filepath.Join(home, "project") {
		t.Fatalf("expanded = %q", expanded)
	}
	if got := RelHome(filepath.Join(home, "project")); got != "~/project" {
		t.Fatalf("RelHome = %q", got)
	}
	if got := Truncate("abcdef", 4); got != "a..." {
		t.Fatalf("Truncate = %q", got)
	}
	if HashString("a", "b") == HashString("ab") {
		t.Fatal("hash separator was ineffective")
	}
	if got := NormalizeSearchText(" Docker\tPOSTGRES\n"); got != "docker postgres" {
		t.Fatalf("NormalizeSearchText = %q", got)
	}
	if got := SearchTokens("docker postgres"); !reflect.DeepEqual(got, []string{"docker", "postgres"}) {
		t.Fatalf("SearchTokens = %#v", got)
	}
	if got := FormatError(errors.New("boom")); got != "devgrep: boom" {
		t.Fatalf("FormatError = %q", got)
	}
	if IsTerminal(nil) {
		t.Fatal("nil terminal reported true")
	}
	if got := HumanDuration(time.Now().Add(-2*time.Hour), time.Now()); got != "2h ago" {
		t.Fatalf("HumanDuration = %q", got)
	}
	if got := HumanBytes(2048); got != "2.0 KiB" {
		t.Fatalf("HumanBytes = %q", got)
	}
	if err := EnsureDir(filepath.Join(home, "nested")); err != nil {
		t.Fatal(err)
	}
	if err := CopyText(""); err == nil {
		t.Fatal("expected copy error")
	}
	if len(clipboardCandidates()) == 0 {
		t.Fatal("no clipboard candidates")
	}
	if len(DefaultIgnoredDirectories()) == 0 {
		t.Fatal("default ignores empty")
	}
	if got := HumanizeError(errors.New("sqlite: no such table: documents")).Error(); got == "" || got == "sqlite: no such table: documents" {
		t.Fatalf("HumanizeError did not convert: %q", got)
	}
	if got := HumanizeError(errors.New("permission denied")).Error(); got == "permission denied" {
		t.Fatalf("HumanizeError permission unchanged")
	}
}

func TestWalkFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.log"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "skip.log"), []byte("skip"), 0o644); err != nil {
		t.Fatal(err)
	}
	var visited []string
	err := WalkFiles(t.Context(), []string{root}, []string{"node_modules"}, func(path string, _ os.DirEntry) bool {
		return HasExtension(path, []string{".log"})
	}, func(path string, _ os.DirEntry) error {
		visited = append(visited, filepath.Base(path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(visited) != 1 || visited[0] != "app.log" {
		t.Fatalf("visited = %#v", visited)
	}
}

func TestWalkFilesWithIgnorePolicy(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "image.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "huge.log"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "one.log"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "two.log"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	var ignored []IgnoreDecision
	var visited []string
	err := WalkFilesWithOptions(t.Context(), []string{root}, WalkOptions{
		MaxFiles:         1,
		MaxFileSizeBytes: 4,
		OnIgnored: func(decision IgnoreDecision) {
			ignored = append(ignored, decision)
		},
	}, func(path string, _ os.DirEntry) bool {
		return HasExtension(path, []string{".log", ".png"})
	}, func(path string, _ os.DirEntry) error {
		visited = append(visited, filepath.Base(path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(visited) != 1 {
		t.Fatalf("visited = %#v", visited)
	}
	if len(ignored) < 2 {
		t.Fatalf("ignored = %#v", ignored)
	}
}
