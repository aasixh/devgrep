package history

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/devgrep/devgrep/internal/config"
	searchpkg "github.com/devgrep/devgrep/internal/search"
	"github.com/devgrep/devgrep/internal/storage"
	"github.com/devgrep/devgrep/internal/utils"
)

// ShellHistoryIndexer indexes bash and zsh history files.
type ShellHistoryIndexer struct {
	Store     *storage.Store
	Config    config.Config
	Home      string
	lastCount int
}

// Name returns the indexer name.
func (i *ShellHistoryIndexer) Name() string {
	return "shell-history"
}

// LastCount returns the number of records written by the last run.
func (i *ShellHistoryIndexer) LastCount() int {
	return i.lastCount
}

// Index indexes bash and zsh histories incrementally.
func (i *ShellHistoryIndexer) Index(ctx context.Context) error {
	i.lastCount = 0
	home := i.Home
	if home == "" {
		home = utils.MustHome()
	}
	replayRequired, err := i.Store.RepairCorruptHistoryCWD(ctx, home)
	if err != nil {
		return err
	}
	sources := []struct {
		shell string
		path  string
	}{
		{shell: "bash", path: filepath.Join(home, ".bash_history")},
		{shell: "zsh", path: filepath.Join(home, ".zsh_history")},
	}
	for _, source := range sources {
		count, err := i.indexFile(ctx, source.shell, source.path, home, replayRequired)
		if err != nil {
			return err
		}
		i.lastCount += count
	}
	return nil
}

// Search searches history results through the shared engine.
func (i *ShellHistoryIndexer) Search(ctx context.Context, query string) ([]searchpkg.Result, error) {
	engine := searchpkg.Engine{Store: i.Store, Config: i.Config}
	return engine.Query(ctx, query, searchpkg.Options{SourceTypes: []string{searchpkg.SourceHistory}, Limit: 50})
}

func (i *ShellHistoryIndexer) indexFile(ctx context.Context, shell, path, home string, forceReplay bool) (int, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("%s history: %w", shell, err)
	}

	state, ok, err := i.Store.GetSourceState(ctx, shell, path)
	if err != nil {
		return 0, err
	}
	if !forceReplay && ok && state.Size == info.Size() && state.ModTime.Unix() == info.ModTime().Unix() {
		return 0, nil
	}

	offset := int64(0)
	lineOffset := 0
	startCWD := home
	if !forceReplay && ok && state.Size > 0 && info.Size() >= state.Size {
		offset = state.Size
		lineOffset = state.LineOffset
		if cwd := state.Metadata["cwd"]; cwd != "" {
			startCWD = ResumeCWD(cwd, home)
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	reader := io.Reader(file)
	if offset > 0 {
		if _, err := file.Seek(offset, 0); err != nil {
			return 0, err
		}
		br := bufio.NewReader(file)
		if _, err := br.ReadString('\n'); err != nil && err != io.EOF {
			return 0, err
		}
		reader = br
	}

	var parsed ParseResult
	switch shell {
	case "bash":
		parsed, err = ParseBash(reader, home, startCWD)
	case "zsh":
		parsed, err = ParseZsh(reader, home, startCWD)
	default:
		err = fmt.Errorf("unsupported shell: %s", shell)
	}
	if err != nil {
		return 0, err
	}
	if offset == 0 && i.Config.History.Limit > 0 && len(parsed.Records) > i.Config.History.Limit {
		parsed.Records = parsed.Records[len(parsed.Records)-i.Config.History.Limit:]
	}

	now := time.Now()
	endCWD := ResumeCWD(parsed.EndCWD, home)
	docs := make([]storage.Document, 0, len(parsed.Records))
	for _, record := range parsed.Records {
		if record.Timestamp.IsZero() {
			record.Timestamp = now
		}
		cwd := SanitizeRecordCWD(record.CWD, home)
		if cwd == "" && record.CWD != "" {
			continue
		}
		normalized := utils.NormalizeSearchText(record.Command + " " + cwd)
		docs = append(docs, storage.Document{
			SourceType: searchpkg.SourceHistory,
			SourceName: shell,
			Content:    record.Command,
			Normalized: normalized,
			CWD:        cwd,
			Path:       path,
			EventTime:  record.Timestamp,
			Hash:       utils.HashString(searchpkg.SourceHistory, shell, utils.NormalizeSearchText(record.Command), cwd),
			Metadata: map[string]string{
				"shell": shell,
			},
		})
	}
	written, err := i.Store.UpsertDocuments(ctx, docs)
	if err != nil {
		return written, err
	}
	if err := i.Store.SaveSourceState(ctx, storage.SourceState{
		SourceName: shell,
		Path:       path,
		Size:       info.Size(),
		ModTime:    info.ModTime(),
		LineOffset: lineOffset + parsed.Lines,
		Metadata: map[string]string{
			"cwd": endCWD,
		},
		LastIndexedAt: now,
	}); err != nil {
		return written, err
	}
	return written, nil
}
