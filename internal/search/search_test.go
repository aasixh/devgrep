package search

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/aasixh/devgrep/internal/config"
	"github.com/aasixh/devgrep/internal/storage"
)

func TestEngineQueryRanksHistory(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "devgrep.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.UpsertDocuments(ctx, []storage.Document{
		{
			SourceType: SourceHistory,
			SourceName: "bash",
			Content:    "docker compose up -d postgres",
			Normalized: "docker compose up -d postgres",
			CWD:        "/work/auth-api",
			EventTime:  time.Now(),
			Frequency:  5,
			Hash:       "docker-postgres",
		},
		{
			SourceType: SourceHistory,
			SourceName: "bash",
			Content:    "kubectl get pods",
			Normalized: "kubectl get pods",
			EventTime:  time.Now().Add(-24 * time.Hour),
			Hash:       "kubectl-pods",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := Engine{Store: store, Config: config.Default()}
	results, err := engine.Query(ctx, "docker postgres", Options{Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no results")
	}
	if results[0].Document.Content != "docker compose up -d postgres" {
		t.Fatalf("top result = %q", results[0].Document.Content)
	}
	var out bytes.Buffer
	PrintPlain(&out, results[:1], time.Now())
	if !bytes.Contains(out.Bytes(), []byte("[history]")) {
		t.Fatalf("plain output = %s", out.String())
	}
}

func BenchmarkSearch100k(b *testing.B) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(b.TempDir(), "devgrep.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	docs := make([]storage.Document, 0, 100000)
	now := time.Now()
	for i := 0; i < 100000; i++ {
		command := fmt.Sprintf("git checkout feature-%d", i)
		if i%1000 == 0 {
			command = fmt.Sprintf("docker compose up -d postgres shard-%d", i)
		}
		docs = append(docs, storage.Document{
			SourceType: SourceHistory,
			SourceName: "bash",
			Content:    command,
			Normalized: command,
			CWD:        "/work/project",
			EventTime:  now.Add(-time.Duration(i) * time.Minute),
			Hash:       fmt.Sprintf("bench-%d", i),
		})
	}
	if _, err := store.UpsertDocuments(ctx, docs); err != nil {
		b.Fatal(err)
	}
	engine := Engine{Store: store, Config: config.Default()}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := engine.Query(ctx, "docker postgres", Options{Limit: 20}); err != nil {
			b.Fatal(err)
		}
	}
}
