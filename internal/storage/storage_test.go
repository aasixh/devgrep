package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreUpsertSearchAndStats(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "devgrep.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.UpsertDocuments(ctx, []Document{{
		SourceType: "history",
		SourceName: "bash",
		Content:    "docker compose up -d postgres",
		Normalized: "docker compose up -d postgres",
		EventTime:  time.Now(),
		Hash:       "test-history-docker",
	}})
	if err != nil {
		t.Fatal(err)
	}
	docs, err := store.SearchCandidates(ctx, "postgres", []string{"history"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("docs = %d, want 1", len(docs))
	}
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalDocuments != 1 {
		t.Fatalf("documents = %d, want 1", stats.TotalDocuments)
	}
	if store.Path() == "" {
		t.Fatal("path was empty")
	}
	if err := store.IntegrityCheck(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordSearch(ctx, "postgres", 1, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordIndexRun(ctx, IndexRun{SourceName: "test", Indexed: 1, Duration: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSourceState(ctx, SourceState{SourceName: "test", Path: "history", Size: 10, ModTime: time.Now(), LineOffset: 2}); err != nil {
		t.Fatal(err)
	}
	state, ok, err := store.GetSourceState(ctx, "test", "history")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || state.LineOffset != 2 {
		t.Fatalf("state = %#v ok=%v", state, ok)
	}
	if err := store.DeleteBySourcePath(ctx, "history", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveWatchedPath(ctx, "/tmp/project"); err != nil {
		t.Fatal(err)
	}
	watched, err := store.WatchedPaths(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(watched) != 1 || watched[0] != "/tmp/project" {
		t.Fatalf("watched = %#v", watched)
	}
	if err := store.RemoveWatchedPath(ctx, "/tmp/project"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.GetSourceState(ctx, "missing", "missing"); err != nil || ok {
		t.Fatalf("missing state ok=%v err=%v", ok, err)
	}
	if _, err := store.UpsertDocuments(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordSearch(ctx, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	if docs, err := store.SearchCandidates(ctx, "definitely-missing", []string{"history"}, 10); err != nil {
		t.Fatal(err)
	} else if len(docs) != 0 {
		t.Fatalf("fallback docs after delete = %#v", docs)
	}
}

func TestSourceLocationsAndDeleteByPath(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "devgrep.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.UpsertDocuments(ctx, []Document{
		{SourceType: "note", SourceName: "markdown", Content: "docs", Path: "/tmp/project/docs/a.md", EventTime: time.Now(), Hash: "source-note"},
		{SourceType: "log", SourceName: "log", Content: "INFO ok", Path: "/tmp/project/logs/app.log", EventTime: time.Now(), Hash: "source-log"},
	})
	if err != nil {
		t.Fatal(err)
	}
	locations, err := store.SourceLocations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(locations) != 2 {
		t.Fatalf("locations = %#v", locations)
	}
	if err := store.DeleteByPath(ctx, "/tmp/project/logs/app.log"); err != nil {
		t.Fatal(err)
	}
	locations, err = store.SourceLocations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(locations) != 1 {
		t.Fatalf("locations after delete = %#v", locations)
	}
}

func TestSearchCandidatesRecentFallback(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "devgrep.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.UpsertDocuments(ctx, []Document{{
		SourceType: "history",
		SourceName: "bash",
		Content:    "docker compose up",
		Normalized: "docker compose up",
		EventTime:  time.Now(),
		Hash:       "fallback-docker",
	}})
	if err != nil {
		t.Fatal(err)
	}
	docs, err := store.SearchCandidates(ctx, "dockre", []string{"history"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("fallback docs = %#v", docs)
	}
}
