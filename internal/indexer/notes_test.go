package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devgrep/devgrep/internal/config"
	searchpkg "github.com/devgrep/devgrep/internal/search"
	"github.com/devgrep/devgrep/internal/storage"
)

func TestNoteIndexerIndexesMarkdown(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	note := filepath.Join(root, "fixes.md")
	if err := os.WriteFile(note, []byte("# CI fix\nRetry failed postgres migration after container healthcheck.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(ctx, filepath.Join(root, "devgrep.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := config.Default()
	cfg.IndexedPaths = []string{root}
	idx := &NoteIndexer{Store: store, Config: cfg}
	if err := idx.Index(ctx); err != nil {
		t.Fatal(err)
	}
	if idx.LastCount() == 0 {
		t.Fatal("no notes indexed")
	}
	docs, err := store.SearchCandidates(ctx, "postgres", []string{searchpkg.SourceNote}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) == 0 {
		t.Fatal("no note search candidates")
	}
}

type fakeIndexer struct {
	name  string
	count int
	err   error
}

func (f fakeIndexer) Name() string { return f.name }
func (f fakeIndexer) Index(context.Context) error {
	return f.err
}
func (f fakeIndexer) Search(context.Context, string) ([]Result, error) {
	return nil, nil
}
func (f fakeIndexer) LastCount() int { return f.count }

func TestRunRecordsIndexRun(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := storage.Open(ctx, filepath.Join(root, "devgrep.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := Run(ctx, store, fakeIndexer{name: "fake", count: 7}); err != nil {
		t.Fatal(err)
	}
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.LastRuns) == 0 || stats.LastRuns[0].Indexed != 7 {
		t.Fatalf("last runs = %#v", stats.LastRuns)
	}
}
