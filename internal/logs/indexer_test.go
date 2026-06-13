package logs

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/aasixh/devgrep/internal/config"
	searchpkg "github.com/aasixh/devgrep/internal/search"
	"github.com/aasixh/devgrep/internal/storage"
)

func TestLogIndexerIndexesSeverity(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := osWrite(filepath.Join(root, "app.log"), "INFO started\nERROR database failed\n"); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(ctx, filepath.Join(root, "devgrep.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := config.Default()
	cfg.IndexedPaths = []string{root}
	indexer := &LogIndexer{Store: store, Config: cfg}
	if err := indexer.Index(ctx); err != nil {
		t.Fatal(err)
	}
	if indexer.LastCount() != 2 {
		t.Fatalf("count = %d", indexer.LastCount())
	}
	docs, err := store.SearchCandidates(ctx, "database", []string{searchpkg.SourceLog}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) == 0 || docs[0].Severity != "ERROR" {
		t.Fatalf("docs = %#v", docs)
	}
}

func TestTailCanceledAndInvalidRegex(t *testing.T) {
	root := t.TempDir()
	if err := osWrite(filepath.Join(root, "app.log"), "INFO started\n"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.IndexedPaths = []string{root}
	if err := Tail(context.Background(), cfg, TailOptions{Pattern: "["}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected invalid regex error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	var out bytes.Buffer
	if err := Tail(ctx, cfg, TailOptions{}, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("tailing")) {
		t.Fatalf("tail output = %q", out.String())
	}
}

func TestDiscoverAndReadNewLines(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "app.log")
	if err := osWrite(path, "INFO started\nERROR database failed\nWARN ignored\n"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.IndexedPaths = []string{root}
	files, err := discoverLogFiles(context.Background(), cfg, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("files = %#v", files)
	}
	var out bytes.Buffer
	offset, err := readNewLines(path, 0, regexp.MustCompile("database"), "ERROR", &out)
	if err != nil {
		t.Fatal(err)
	}
	if offset == 0 || !bytes.Contains(out.Bytes(), []byte("database failed")) {
		t.Fatalf("offset=%d output=%q", offset, out.String())
	}
	out.Reset()
	if _, err := readNewLines(path, offset, nil, "", &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected output = %q", out.String())
	}
}

func osWrite(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
