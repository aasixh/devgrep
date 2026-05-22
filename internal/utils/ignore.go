package utils

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// IgnoreDecision explains why a path was skipped.
type IgnoreDecision struct {
	Path   string
	Reason string
}

// WalkOptions controls safe project file discovery.
type WalkOptions struct {
	IgnoredDirs      []string
	MaxFiles         int
	MaxFileSizeBytes int64
	OnIgnored        func(IgnoreDecision)
}

var defaultIgnoredFileExtensions = map[string]string{
	".7z":    "archive",
	".a":     "binary",
	".avi":   "media",
	".bin":   "binary",
	".bmp":   "media",
	".class": "binary",
	".dmg":   "archive",
	".dll":   "binary",
	".exe":   "binary",
	".gif":   "media",
	".gz":    "archive",
	".ico":   "media",
	".iso":   "archive",
	".jar":   "archive",
	".jpeg":  "media",
	".jpg":   "media",
	".mov":   "media",
	".mp3":   "media",
	".mp4":   "media",
	".o":     "binary",
	".pdf":   "media",
	".png":   "media",
	".so":    "binary",
	".tar":   "archive",
	".tgz":   "archive",
	".wasm":  "binary",
	".webm":  "media",
	".webp":  "media",
	".zip":   "archive",
}

// DefaultIgnoredDirectories returns the built-in directory skip list.
func DefaultIgnoredDirectories() []string {
	return []string{
		".git",
		".hg",
		".svn",
		"node_modules",
		"vendor",
		".venv",
		"__pycache__",
		"target",
		"dist",
		"build",
		".next",
		".cache",
	}
}

// ShouldIgnoreDir reports whether a directory name should be skipped.
func ShouldIgnoreDir(name string, ignoredDirs []string) bool {
	for _, dir := range ignoredDirs {
		if strings.EqualFold(name, dir) {
			return true
		}
	}
	return false
}

// ShouldIgnoreFile reports whether a file should be skipped before parsing.
func ShouldIgnoreFile(path string, entry fs.DirEntry, maxFileSizeBytes int64) (string, bool) {
	ext := strings.ToLower(filepath.Ext(path))
	if reason, ok := defaultIgnoredFileExtensions[ext]; ok {
		return reason, true
	}
	if maxFileSizeBytes > 0 {
		info, err := entry.Info()
		if err == nil && info.Size() > maxFileSizeBytes {
			return "huge file", true
		}
	}
	return "", false
}
