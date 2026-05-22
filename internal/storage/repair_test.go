package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRepairCorruptHistoryCWD(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	dbPath := filepath.Join(home, "devgrep.db")
	store, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	corrupt := "/tmp/~/work/url-shortner"
	historyPath := filepath.Join(home, ".bash_history")
	if err := os.WriteFile(historyPath, []byte("#1700000000\ncd project\nmake test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = store.UpsertDocuments(ctx, []Document{{
		SourceType: "history",
		SourceName: "bash",
		Content:    "git status",
		Normalized: "git status " + corrupt,
		CWD:        corrupt,
		Path:       historyPath,
		EventTime:  time.Now(),
		Hash:       "corrupt-row",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertDocuments(ctx, []Document{{
		SourceType: "history",
		SourceName: "bash",
		Content:    "make test",
		Normalized: "make test",
		CWD:        filepath.Join(home, "project"),
		Path:       historyPath,
		EventTime:  time.Now(),
		Hash:       "valid-row",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSourceState(ctx, SourceState{
		SourceName:    "bash",
		Path:          historyPath,
		Size:          42,
		ModTime:       time.Now(),
		LineOffset:    99,
		Metadata:      map[string]string{"cwd": corrupt},
		LastIndexedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	replay, err := store.RepairCorruptHistoryCWD(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	if !replay {
		t.Fatal("expected replay required")
	}

	count, err := store.DocumentCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("documents after repair = %d, want 0", count)
	}
	if _, ok, err := store.GetSourceState(ctx, "bash", historyPath); err != nil || ok {
		t.Fatalf("source state should be reset, ok=%v err=%v", ok, err)
	}
}

func TestRepairDetectsKernelPathCWD(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	store, err := Open(ctx, filepath.Join(home, "devgrep.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	corrupt := "/sys/kernel/boot_params/params/legacy/ebda/ebda2/software_nodes/node0"
	_, err = store.UpsertDocuments(ctx, []Document{{
		SourceType: "history",
		SourceName: "zsh",
		Content:    "grep pattern",
		Normalized: "grep pattern " + corrupt,
		CWD:        corrupt,
		Path:       filepath.Join(home, ".zsh_history"),
		EventTime:  time.Now(),
		Hash:       "kernel-path-row",
	}})
	if err != nil {
		t.Fatal(err)
	}

	replay, err := store.RepairCorruptHistoryCWD(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	if !replay {
		t.Fatal("expected replay for kernel path cwd")
	}
	count, err := store.DocumentCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("documents after repair = %d, want 0", count)
	}
}
