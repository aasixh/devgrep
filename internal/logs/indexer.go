package logs

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/devgrep/devgrep/internal/config"
	"github.com/devgrep/devgrep/internal/parser"
	searchpkg "github.com/devgrep/devgrep/internal/search"
	"github.com/devgrep/devgrep/internal/storage"
	"github.com/devgrep/devgrep/internal/utils"
)

// LogIndexer indexes plaintext .log files.
type LogIndexer struct {
	Store      *storage.Store
	Config     config.Config
	ExtraPaths []string
	lastCount  int
	lastFiles  int
}

// Name returns the indexer name.
func (i *LogIndexer) Name() string {
	return "logs"
}

// LastCount returns the number of indexed log lines from the last run.
func (i *LogIndexer) LastCount() int {
	return i.lastCount
}

// LastFiles returns the number of log files touched by the last run.
func (i *LogIndexer) LastFiles() int {
	return i.lastFiles
}

// Index incrementally indexes configured .log files.
func (i *LogIndexer) Index(ctx context.Context) error {
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
		return utils.HasExtension(path, i.Config.Logs.Extensions)
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

// Search searches log results through the shared engine.
func (i *LogIndexer) Search(ctx context.Context, query string) ([]searchpkg.Result, error) {
	engine := searchpkg.Engine{Store: i.Store, Config: i.Config}
	return engine.Query(ctx, query, searchpkg.Options{SourceTypes: []string{searchpkg.SourceLog}, Limit: 50})
}

func (i *LogIndexer) indexFile(ctx context.Context, path string) (int, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if max := int64(i.Config.Logs.MaxFileSizeMB) * 1024 * 1024; max > 0 && info.Size() > max {
		return 0, nil
	}
	state, ok, err := i.Store.GetSourceState(ctx, "log", path)
	if err != nil {
		return 0, err
	}
	if ok && state.Size == info.Size() && state.ModTime.Unix() == info.ModTime().Unix() {
		return 0, nil
	}
	offset := int64(0)
	lineOffset := 0
	if ok && state.Size > 0 && info.Size() >= state.Size {
		offset = state.Size
		lineOffset = state.LineOffset
	}
	if offset == 0 {
		if err := i.Store.DeleteBySourcePath(ctx, searchpkg.SourceLog, path); err != nil {
			return 0, err
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	if offset > 0 {
		if _, err := file.Seek(offset, 0); err != nil {
			return 0, err
		}
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNo := lineOffset
	now := time.Now()
	var docs []storage.Document
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if line == "" {
			continue
		}
		severity := parser.DetectSeverity(line)
		metadata := map[string]string{}
		if severity == parser.SeverityError {
			metadata["group"] = utils.HashString(parser.ErrorGroupKey(line))
		}
		docs = append(docs, storage.Document{
			SourceType: searchpkg.SourceLog,
			SourceName: "log",
			Content:    line,
			Normalized: utils.NormalizeSearchText(line + " " + path + " " + severity),
			Path:       path,
			Line:       lineNo,
			Severity:   severity,
			EventTime:  info.ModTime(),
			Hash:       utils.HashString(searchpkg.SourceLog, path, fmt.Sprint(lineNo), line),
			Metadata:   metadata,
		})
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	written, err := i.Store.UpsertDocuments(ctx, docs)
	if err != nil {
		return written, err
	}
	if err := i.Store.SaveSourceState(ctx, storage.SourceState{
		SourceName:    "log",
		Path:          path,
		Size:          info.Size(),
		ModTime:       info.ModTime(),
		LineOffset:    lineNo,
		Metadata:      map[string]string{"indexed_at": now.Format(time.RFC3339)},
		LastIndexedAt: now,
	}); err != nil {
		return written, err
	}
	return written, nil
}
