package utils

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// WalkFiles visits matching files under roots while respecting ignored directory names.
func WalkFiles(ctx context.Context, roots []string, ignoredDirs []string, match func(string, fs.DirEntry) bool, visit func(string, fs.DirEntry) error) error {
	return WalkFilesWithOptions(ctx, roots, WalkOptions{IgnoredDirs: ignoredDirs}, match, visit)
}

// WalkFilesWithOptions visits matching files with centralized ignore handling.
func WalkFilesWithOptions(ctx context.Context, roots []string, opts WalkOptions, match func(string, fs.DirEntry) bool, visit func(string, fs.DirEntry) error) error {
	visited := 0
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return err
		}
		if root == "" {
			continue
		}
		expanded, err := ExpandHome(root)
		if err != nil {
			return err
		}
		if _, err := os.Stat(expanded); os.IsNotExist(err) {
			continue
		}
		err = filepath.WalkDir(expanded, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.IsDir() {
				if ShouldIgnoreDir(entry.Name(), opts.IgnoredDirs) && path != expanded {
					if opts.OnIgnored != nil {
						opts.OnIgnored(IgnoreDecision{Path: path, Reason: entry.Name()})
					}
					return filepath.SkipDir
				}
				return nil
			}
			if reason, ignored := ShouldIgnoreFile(path, entry, opts.MaxFileSizeBytes); ignored {
				if opts.OnIgnored != nil {
					opts.OnIgnored(IgnoreDecision{Path: path, Reason: reason})
				}
				return nil
			}
			if match(path, entry) {
				if opts.MaxFiles > 0 && visited >= opts.MaxFiles {
					if opts.OnIgnored != nil {
						opts.OnIgnored(IgnoreDecision{Path: path, Reason: "max file limit"})
					}
					return nil
				}
				visited++
				return visit(path, entry)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// HasExtension reports whether path has one of the supplied case-insensitive extensions.
func HasExtension(path string, exts []string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, candidate := range exts {
		if strings.EqualFold(ext, candidate) {
			return true
		}
	}
	return false
}
