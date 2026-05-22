package search

import "github.com/devgrep/devgrep/internal/storage"

const (
	// SourceHistory identifies shell history records.
	SourceHistory = "history"
	// SourceLog identifies plaintext log records.
	SourceLog = "log"
	// SourceNote identifies local markdown note fragments.
	SourceNote = "note"
)

// Result is a ranked search hit.
type Result struct {
	Document       storage.Document
	Score          int
	FuzzyScore     int
	MatchedIndexes []int
	ExactPhrase    bool
}

// Options controls a search query.
type Options struct {
	Limit       int
	SourceTypes []string
	Record      bool
}
