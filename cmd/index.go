package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aasixh/devgrep/internal/config"
	"github.com/aasixh/devgrep/internal/history"
	idxpkg "github.com/aasixh/devgrep/internal/indexer"
	"github.com/aasixh/devgrep/internal/logs"
	"github.com/aasixh/devgrep/internal/storage"
	"github.com/aasixh/devgrep/internal/utils"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

func newIndexCommand() *cobra.Command {
	var sources []string
	var paths []string
	var watch bool
	var noWatch bool
	var dryRun bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "index [path...]",
		Short: "Index shell history, logs, and markdown notes",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if err := config.EnsureDefaultFile(cfg); err != nil {
				return err
			}
			indexPaths, explicitPaths, err := resolveIndexPaths(cfg, args, paths)
			if err != nil {
				return err
			}
			if err := confirmSafeIndexPaths(indexPaths, explicitPaths, yes, cmd.InOrStdin(), cmd.OutOrStdout()); err != nil {
				return err
			}
			if dryRun {
				return runDryRun(ctx, cfg, sources, indexPaths, cmd.OutOrStdout())
			}
			store, err := openStore(ctx, cfg)
			if err != nil {
				return err
			}
			defer store.Close()

			indexers, err := buildIndexers(store, cfg, sources, indexPaths)
			if err != nil {
				return err
			}
			summary, err := runIndexers(ctx, store, indexers, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			printIndexSummary(cmd.OutOrStdout(), summary, cfg.IgnoredDirectories)
			if explicitPaths {
				for _, path := range indexPaths {
					if info, err := os.Stat(path); err == nil && info.IsDir() {
						_ = store.SaveWatchedPath(ctx, path)
					}
				}
			}
			if watch || (explicitPaths && cfg.Indexing.AutoWatch && !noWatch) {
				return watchIndex(ctx, cfg, store, indexers, indexPaths, cmd.OutOrStdout())
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&sources, "source", []string{"all"}, "sources to index: all, history, logs, notes")
	cmd.Flags().StringArrayVar(&paths, "path", nil, "additional path to index for logs and notes")
	cmd.Flags().BoolVar(&watch, "watch", false, "watch configured paths and re-index on changes")
	cmd.Flags().BoolVar(&noWatch, "no-watch", false, "disable automatic watch after indexing explicit paths")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be indexed without writing to the database")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm dangerous indexing paths non-interactively")
	return cmd
}

type indexRunSummary struct {
	Name    string
	Count   int
	Files   int
	Elapsed time.Duration
}

func buildIndexers(store *storage.Store, cfg config.Config, sources []string, paths []string) ([]idxpkg.Indexer, error) {
	sourceSet, err := normalizeSources(sources)
	if err != nil {
		return nil, err
	}
	var indexers []idxpkg.Indexer
	if sourceSet["history"] {
		indexers = append(indexers, &history.ShellHistoryIndexer{Store: store, Config: cfg})
	}
	if sourceSet["logs"] {
		indexers = append(indexers, &logs.LogIndexer{Store: store, Config: cfg, ExtraPaths: paths})
	}
	if sourceSet["notes"] {
		indexers = append(indexers, &idxpkg.NoteIndexer{Store: store, Config: cfg, ExtraPaths: paths})
	}
	return indexers, nil
}

func normalizeSources(sources []string) (map[string]bool, error) {
	if len(sources) == 0 {
		sources = []string{"all"}
	}
	set := map[string]bool{}
	for _, source := range sources {
		for _, part := range strings.Split(source, ",") {
			part = strings.ToLower(strings.TrimSpace(part))
			switch part {
			case "", "all":
				set["history"] = true
				set["logs"] = true
				set["notes"] = true
			case "history", "hist":
				set["history"] = true
			case "logs", "log":
				set["logs"] = true
			case "notes", "note", "markdown":
				set["notes"] = true
			default:
				return nil, fmt.Errorf("unknown source %q", part)
			}
		}
	}
	return set, nil
}

func runIndexers(ctx context.Context, store *storage.Store, indexers []idxpkg.Indexer, out io.Writer) ([]indexRunSummary, error) {
	var summaries []indexRunSummary
	for _, idx := range indexers {
		start := time.Now()
		if err := idxpkg.Run(ctx, store, idx); err != nil {
			return summaries, err
		}
		count := 0
		if c, ok := idx.(idxpkg.CountingIndexer); ok {
			count = c.LastCount()
		}
		files := 0
		if c, ok := idx.(idxpkg.FileCountingIndexer); ok {
			files = c.LastFiles()
		}
		elapsed := time.Since(start).Round(time.Millisecond)
		summaries = append(summaries, indexRunSummary{Name: idx.Name(), Count: count, Files: files, Elapsed: elapsed})
		fmt.Fprintf(out, "%-16s indexed %-6d %s\n", idx.Name(), count, elapsed)
	}
	return summaries, nil
}

func watchIndex(ctx context.Context, cfg config.Config, store *storage.Store, indexers []idxpkg.Indexer, paths []string, out io.Writer) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	watchPaths := paths
	if len(watchPaths) == 0 {
		if store != nil {
			if restored, err := store.WatchedPaths(ctx); err == nil && len(restored) > 0 {
				watchPaths = restored
			}
		}
		if len(watchPaths) == 0 {
			watchPaths = cfg.IndexedPaths
		}
	}
	home := utils.MustHome()
	watchPaths = append(watchPaths, filepath.Join(home, ".bash_history"), filepath.Join(home, ".zsh_history"))
	added := map[string]struct{}{}
	for _, path := range watchPaths {
		expanded, err := utils.ExpandHome(path)
		if err != nil {
			continue
		}
		info, err := os.Stat(expanded)
		if err != nil {
			if store != nil {
				_ = store.RemoveWatchedPath(ctx, expanded)
			}
			continue
		}
		target := expanded
		if !info.IsDir() {
			target = filepath.Dir(expanded)
		}
		if _, ok := added[target]; ok {
			continue
		}
		if err := watcher.Add(target); err == nil {
			added[target] = struct{}{}
		}
	}
	fmt.Fprintf(out, "watching %d path(s); press Ctrl-C to stop\n", len(added))
	var timer *time.Timer
	for {
		var timerC <-chan time.Time
		if timer != nil {
			timerC = timer.C
		}
		select {
		case <-ctx.Done():
			return nil
		case err := <-watcher.Errors:
			if err != nil {
				fmt.Fprintf(out, "watch error: %v\n", err)
			}
		case event := <-watcher.Events:
			if event.Name == "" || (!event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) && !event.Has(fsnotify.Rename)) {
				if event.Has(fsnotify.Remove) && store != nil {
					_ = store.DeleteByPath(ctx, event.Name)
				}
				continue
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(500 * time.Millisecond)
		case <-timerC:
			fmt.Fprintln(out, "change detected; re-indexing")
			if _, err := runIndexers(ctx, store, indexers, out); err != nil {
				fmt.Fprintf(out, "index error: %v\n", err)
			}
			timer = nil
		}
	}
}

func resolveIndexPaths(cfg config.Config, args []string, flagPaths []string) ([]string, bool, error) {
	raw := append([]string{}, args...)
	raw = append(raw, flagPaths...)
	explicit := len(raw) > 0
	if !explicit {
		raw = cfg.IndexedPaths
	}
	if len(raw) == 0 {
		return nil, explicit, fmt.Errorf("no paths configured for indexing")
	}
	seen := map[string]struct{}{}
	var paths []string
	for _, path := range raw {
		expanded, err := utils.ExpandHome(path)
		if err != nil {
			return nil, explicit, err
		}
		abs, err := filepath.Abs(expanded)
		if err != nil {
			return nil, explicit, err
		}
		clean := filepath.Clean(abs)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		paths = append(paths, clean)
	}
	return paths, explicit, nil
}

func confirmSafeIndexPaths(paths []string, explicit bool, yes bool, in io.Reader, out io.Writer) error {
	if !explicit {
		return nil
	}
	for _, path := range paths {
		reason := dangerousIndexReason(path)
		if reason == "" {
			continue
		}
		if yes {
			fmt.Fprintf(out, "warning: indexing %s (%s)\n", utils.RelHome(path), reason)
			continue
		}
		if !utils.IsTerminal(os.Stdin) {
			return fmt.Errorf("refusing to index %s: %s. Re-run with --yes to confirm", utils.RelHome(path), reason)
		}
		fmt.Fprintf(out, "Indexing %s may scan a huge number of files (%s).\nContinue? [y/N] ", utils.RelHome(path), reason)
		answer, _ := bufio.NewReader(in).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			return fmt.Errorf("index canceled")
		}
	}
	return nil
}

func dangerousIndexReason(path string) string {
	clean := filepath.Clean(path)
	if clean == string(filepath.Separator) {
		return "filesystem root"
	}
	home := utils.MustHome()
	if clean == filepath.Clean(home) {
		return "home directory"
	}
	return ""
}

func printIndexSummary(out io.Writer, summaries []indexRunSummary, ignoredDirs []string) {
	fmt.Fprintln(out, "\nIndexed:")
	if len(summaries) == 0 {
		fmt.Fprintln(out, "- nothing")
		return
	}
	for _, summary := range summaries {
		switch summary.Name {
		case "shell-history":
			fmt.Fprintf(out, "- %d history entries\n", summary.Count)
		case "markdown-notes":
			fmt.Fprintf(out, "- %d markdown notes", summary.Count)
			if summary.Files > 0 {
				fmt.Fprintf(out, " across %d files", summary.Files)
			}
			fmt.Fprintln(out)
		case "logs":
			fmt.Fprintf(out, "- %d log lines", summary.Count)
			if summary.Files > 0 {
				fmt.Fprintf(out, " across %d log files", summary.Files)
			}
			fmt.Fprintln(out)
		default:
			fmt.Fprintf(out, "- %d from %s\n", summary.Count, summary.Name)
		}
	}
	fmt.Fprintln(out, "\nIgnored:")
	if len(ignoredDirs) == 0 {
		fmt.Fprintln(out, "- none")
		return
	}
	for _, dir := range ignoredDirs {
		fmt.Fprintf(out, "- %s\n", dir)
	}
}

type dryRunSummary struct {
	HistoryFiles int
	HistoryLines int
	LogFiles     int
	NoteFiles    int
	Ignored      []utils.IgnoreDecision
	WouldIndex   []string
}

func runDryRun(ctx context.Context, cfg config.Config, sources []string, paths []string, out io.Writer) error {
	sourceSet, err := normalizeSources(sources)
	if err != nil {
		return err
	}
	summary := dryRunSummary{}
	if sourceSet["history"] {
		home := utils.MustHome()
		for _, path := range []string{filepath.Join(home, ".bash_history"), filepath.Join(home, ".zsh_history")} {
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				continue
			}
			lines, _ := countLines(path)
			summary.HistoryFiles++
			summary.HistoryLines += lines
			summary.WouldIndex = append(summary.WouldIndex, path)
		}
	}
	walkOpts := utils.WalkOptions{
		IgnoredDirs:      cfg.IgnoredDirectories,
		MaxFiles:         cfg.Indexing.MaxFiles,
		MaxFileSizeBytes: int64(cfg.Indexing.MaxFileSizeMB) * 1024 * 1024,
		OnIgnored: func(decision utils.IgnoreDecision) {
			summary.Ignored = append(summary.Ignored, decision)
		},
	}
	seen := map[string]struct{}{}
	if sourceSet["logs"] {
		err := utils.WalkFilesWithOptions(ctx, paths, walkOpts, func(path string, entry os.DirEntry) bool {
			return utils.HasExtension(path, cfg.Logs.Extensions)
		}, func(path string, entry os.DirEntry) error {
			if _, ok := seen[path]; !ok {
				seen[path] = struct{}{}
				summary.LogFiles++
				summary.WouldIndex = append(summary.WouldIndex, path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if sourceSet["notes"] {
		err := utils.WalkFilesWithOptions(ctx, paths, walkOpts, func(path string, entry os.DirEntry) bool {
			return utils.HasExtension(path, []string{".md", ".markdown"})
		}, func(path string, entry os.DirEntry) error {
			if _, ok := seen[path]; !ok {
				seen[path] = struct{}{}
				summary.NoteFiles++
				summary.WouldIndex = append(summary.WouldIndex, path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	printDryRunSummary(out, summary)
	return nil
}

func printDryRunSummary(out io.Writer, summary dryRunSummary) {
	fmt.Fprintln(out, "Dry run: no database writes will be made.")
	fmt.Fprintln(out, "\nWould index:")
	fmt.Fprintf(out, "- %d history entries from %d history files\n", summary.HistoryLines, summary.HistoryFiles)
	fmt.Fprintf(out, "- %d markdown files\n", summary.NoteFiles)
	fmt.Fprintf(out, "- %d log files\n", summary.LogFiles)
	if len(summary.WouldIndex) > 0 {
		fmt.Fprintln(out, "\nFiles:")
		sort.Strings(summary.WouldIndex)
		for _, path := range summary.WouldIndex {
			fmt.Fprintf(out, "- %s\n", utils.RelHome(path))
		}
	}
	fmt.Fprintln(out, "\nIgnored:")
	if len(summary.Ignored) == 0 {
		fmt.Fprintln(out, "- none")
		return
	}
	seen := map[string]struct{}{}
	for _, ignored := range summary.Ignored {
		key := ignored.Path + "\x00" + ignored.Reason
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		fmt.Fprintf(out, "- %s (%s)\n", utils.RelHome(ignored.Path), ignored.Reason)
	}
}

func countLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}
