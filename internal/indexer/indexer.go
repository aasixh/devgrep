package indexer

import (
	"context"
	"time"

	searchpkg "github.com/aasixh/devgrep/internal/search"
	"github.com/aasixh/devgrep/internal/storage"
)

// Result aliases the shared search result type for pluggable indexers.
type Result = searchpkg.Result

// Indexer is implemented by local source indexers.
type Indexer interface {
	Name() string
	Index(ctx context.Context) error
	Search(ctx context.Context, query string) ([]Result, error)
}

// CountingIndexer is implemented by indexers that expose their last write count.
type CountingIndexer interface {
	LastCount() int
}

// FileCountingIndexer exposes the number of source files touched.
type FileCountingIndexer interface {
	LastFiles() int
}

// Run executes one indexer and records its summary in storage.
func Run(ctx context.Context, store *storage.Store, idx Indexer) error {
	start := time.Now()
	err := idx.Index(ctx)
	count := 0
	if c, ok := idx.(CountingIndexer); ok {
		count = c.LastCount()
	}
	run := storage.IndexRun{
		SourceName: idx.Name(),
		Indexed:    count,
		Duration:   time.Since(start),
	}
	if err != nil {
		run.Error = err.Error()
	}
	_ = store.RecordIndexRun(ctx, run)
	return err
}
