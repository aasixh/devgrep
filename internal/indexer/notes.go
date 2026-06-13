package indexer

import (
	"context"
	"io/fs"
	"os"
	"time"

	"github.com/aasixh/devgrep/internal/config"
	"github.com/aasixh/devgrep/internal/parser"
	searchpkg "github.com/aasixh/devgrep/internal/search"
	"github.com/aasixh/devgrep/internal/storage"
	"github.com/aasixh/devgrep/internal/utils"
)

// NoteIndexer indexes local markdown notes.
type NoteIndexer struct {
	Store      *storage.Store
	Config     config.Config
	ExtraPaths []string
	lastCount  int
	lastFiles  int
}

// Name returns the indexer name.
func (i *NoteIndexer) Name() string {
	return "markdown-notes"
}

// LastCount returns the number of indexed note fragments from the last run.
func (i *NoteIndexer) LastCount() int {
	return i.lastCount
}

// LastFiles returns the number of markdown files touched by the last run.
func (i *NoteIndexer) LastFiles() int {
	return i.lastFiles
}

// Index indexes markdown note fragments.
func (i *NoteIndexer) Index(ctx context.Context) error {
	i.lastCount = 0
	i.lastFiles = 0
	paths := i.ExtraPaths
	if len(paths) == 0 {
		paths = i.Config.IndexedPaths
	}
	return utils.WalkFilesWithOptions(ctx, paths, utils.WalkOptions{
		IgnoredDirs:      i.Config.IgnoredDirectories,
		MaxFiles:         i.Config.Indexing.MaxFiles,
		MaxFileSizeBytes: int64(i.Config.Indexing.MaxFileSizeMB) * 1024 * 1024,
	}, func(path string, entry fs.DirEntry) bool {
		return utils.HasExtension(path, []string{".md", ".markdown"})
	}, func(path string, entry fs.DirEntry) error {
		count, err := i.indexFile(ctx, path)
		if err != nil {
			return err
		}
		i.lastFiles++
		i.lastCount += count
		return nil
	})
}

// Search searches note results through the shared engine.
func (i *NoteIndexer) Search(ctx context.Context, query string) ([]searchpkg.Result, error) {
	engine := searchpkg.Engine{Store: i.Store, Config: i.Config}
	return engine.Query(ctx, query, searchpkg.Options{SourceTypes: []string{searchpkg.SourceNote}, Limit: 50})
}

func (i *NoteIndexer) indexFile(ctx context.Context, path string) (int, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	state, ok, err := i.Store.GetSourceState(ctx, "note", path)
	if err != nil {
		return 0, err
	}
	if ok && state.Size == info.Size() && state.ModTime.Unix() == info.ModTime().Unix() {
		return 0, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	fragments, err := parser.ParseMarkdownFragments(file)
	if err != nil {
		return 0, err
	}
	if err := i.Store.DeleteBySourcePath(ctx, searchpkg.SourceNote, path); err != nil {
		return 0, err
	}
	docs := make([]storage.Document, 0, len(fragments))
	for _, fragment := range fragments {
		docs = append(docs, storage.Document{
			SourceType: searchpkg.SourceNote,
			SourceName: "markdown",
			Content:    fragment.Content,
			Normalized: utils.NormalizeSearchText(fragment.Content + " " + path),
			Path:       path,
			Line:       fragment.Line,
			EventTime:  info.ModTime(),
			Hash:       utils.HashString(searchpkg.SourceNote, path, fragment.Content),
			Metadata: map[string]string{
				"kind": "markdown",
			},
		})
	}
	written, err := i.Store.UpsertDocuments(ctx, docs)
	if err != nil {
		return written, err
	}
	now := time.Now()
	if err := i.Store.SaveSourceState(ctx, storage.SourceState{
		SourceName:    "note",
		Path:          path,
		Size:          info.Size(),
		ModTime:       info.ModTime(),
		LineOffset:    len(fragments),
		Metadata:      map[string]string{"indexed_at": now.Format(time.RFC3339)},
		LastIndexedAt: now,
	}); err != nil {
		return written, err
	}
	return written, nil
}
