package logs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aasixh/devgrep/internal/config"
	"github.com/aasixh/devgrep/internal/parser"
	"github.com/aasixh/devgrep/internal/utils"
	"github.com/fsnotify/fsnotify"
)

// TailOptions controls live log tailing.
type TailOptions struct {
	Paths    []string
	Pattern  string
	Severity string
}

// Tail follows configured .log files and prints matching appended lines.
func Tail(ctx context.Context, cfg config.Config, opts TailOptions, out io.Writer) error {
	paths := opts.Paths
	if len(paths) == 0 {
		paths = cfg.IndexedPaths
	}
	var re *regexp.Regexp
	if opts.Pattern != "" {
		compiled, err := regexp.Compile(opts.Pattern)
		if err != nil {
			return err
		}
		re = compiled
	}
	files, err := discoverLogFiles(ctx, cfg, paths)
	if err != nil {
		return err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	offsets := map[string]int64{}
	watchedDirs := map[string]struct{}{}
	for _, file := range files {
		info, err := os.Stat(file)
		if err == nil {
			offsets[file] = info.Size()
			dir := filepath.Dir(file)
			if _, ok := watchedDirs[dir]; !ok {
				_ = watcher.Add(dir)
				watchedDirs[dir] = struct{}{}
			}
		}
	}
	for _, root := range paths {
		expanded, err := utils.ExpandHome(root)
		if err == nil {
			if info, err := os.Stat(expanded); err == nil && info.IsDir() {
				if _, ok := watchedDirs[expanded]; !ok {
					_ = watcher.Add(expanded)
					watchedDirs[expanded] = struct{}{}
				}
			}
		}
	}
	fmt.Fprintf(out, "tailing %d log file(s); press Ctrl-C to stop\n", len(offsets))
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-watcher.Errors:
			if err != nil {
				fmt.Fprintf(out, "watch error: %v\n", err)
			}
		case event := <-watcher.Events:
			if event.Name == "" {
				continue
			}
			if event.Has(fsnotify.Create) && utils.HasExtension(event.Name, cfg.Logs.Extensions) {
				if _, ok := offsets[event.Name]; !ok {
					offsets[event.Name] = 0
				}
			}
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}
			if !utils.HasExtension(event.Name, cfg.Logs.Extensions) {
				continue
			}
			next, err := readNewLines(event.Name, offsets[event.Name], re, strings.ToUpper(opts.Severity), out)
			if err == nil {
				offsets[event.Name] = next
			}
		}
	}
}

func discoverLogFiles(ctx context.Context, cfg config.Config, roots []string) ([]string, error) {
	var files []string
	err := utils.WalkFiles(ctx, roots, cfg.IgnoredDirectories, func(path string, entry os.DirEntry) bool {
		return utils.HasExtension(path, cfg.Logs.Extensions)
	}, func(path string, entry os.DirEntry) error {
		files = append(files, path)
		return nil
	})
	return files, err
}

func readNewLines(path string, offset int64, re *regexp.Regexp, severity string, out io.Writer) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return offset, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return offset, err
	}
	if info.Size() < offset {
		offset = 0
	}
	if _, err := file.Seek(offset, 0); err != nil {
		return offset, err
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if re != nil && !re.MatchString(line) {
			continue
		}
		if severity != "" && parser.DetectSeverity(line) != severity {
			continue
		}
		fmt.Fprintf(out, "%s: %s\n", utils.RelHome(path), line)
	}
	if err := scanner.Err(); err != nil {
		return offset, err
	}
	pos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return offset, err
	}
	return pos, nil
}
