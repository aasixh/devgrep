package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
)

// ExpandHome expands a leading ~ in a path.
func ExpandHome(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return ExpandPathWithHome(path, home)
}

// ExpandPathWithHome expands ~, ~/, and $HOME using the provided home directory.
func ExpandPathWithHome(path, home string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if home == "" {
		home = MustHome()
	}
	switch {
	case path == "~":
		return home, nil
	case strings.HasPrefix(path, "~/"):
		return filepath.Clean(filepath.Join(home, path[2:])), nil
	case path == "$HOME":
		return home, nil
	case strings.HasPrefix(path, "$HOME/"):
		return filepath.Clean(filepath.Join(home, path[6:])), nil
	case strings.HasPrefix(path, "$HOME\\"):
		return filepath.Clean(filepath.Join(home, path[6:])), nil
	}
	return path, nil
}

// InvalidHistoryCWD reports cwd values that should not be stored for shell history.
func InvalidHistoryCWD(path string) bool {
	if path == "" {
		return false
	}
	if MalformedCWDPath(path) {
		return true
	}
	clean := filepath.Clean(path)
	for _, prefix := range []string{"/sys/", "/proc/", "/dev/"} {
		if strings.HasPrefix(clean, prefix) {
			return true
		}
	}
	if len(clean) > 512 {
		return true
	}
	return false
}

// MalformedCWDPath reports paths that cannot represent a real working directory.
// It catches chained tilde segments such as /tmp/~/work produced by bad joins.
func MalformedCWDPath(path string) bool {
	if path == "" {
		return false
	}
	clean := filepath.Clean(path)
	if strings.Contains(clean, "/~/") {
		return true
	}
	sep := string(filepath.Separator)
	for _, part := range strings.Split(clean, sep) {
		if part == "" {
			continue
		}
		if !strings.HasPrefix(part, "~") {
			continue
		}
		// A tilde may only appear in the home-relative form "~/..." at the start of the path string.
		if strings.HasPrefix(clean, "~/") && part == "~" {
			continue
		}
		return true
	}
	return false
}

// NormalizeCWD returns a clean absolute path when path is a valid cwd, otherwise ("", false).
func NormalizeCWD(path, home string) (string, bool) {
	if path == "" {
		return "", true
	}
	if InvalidHistoryCWD(path) {
		return "", false
	}
	expanded, err := ExpandPathWithHome(path, home)
	if err != nil {
		return "", false
	}
	if expanded == "" && path != "" {
		expanded = path
	}
	if !filepath.IsAbs(expanded) {
		return "", false
	}
	clean := filepath.Clean(expanded)
	if MalformedCWDPath(clean) {
		return "", false
	}
	return clean, true
}

// SanitizeCWD normalizes a cwd or returns fallback when the path is empty or malformed.
func SanitizeCWD(path, home, fallback string) string {
	if fallback == "" {
		fallback = home
	}
	if fallback == "" {
		fallback = MustHome()
	}
	if norm, ok := NormalizeCWD(path, home); ok && norm != "" {
		return norm
	}
	if norm, ok := NormalizeCWD(fallback, home); ok && norm != "" {
		return norm
	}
	return filepath.Clean(fallback)
}

// MustHome returns the current user's home directory or "." if it is not known.
func MustHome() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "."
	}
	return home
}

// RelHome renders a path relative to the user's home directory when possible.
func RelHome(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	home := MustHome()
	if rel, err := filepath.Rel(home, abs); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return "~/" + filepath.ToSlash(rel)
	}
	return path
}

// EnsureDir creates the directory if it does not already exist.
func EnsureDir(path string) error {
	if path == "" {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}

// HashString returns a stable SHA-256 hex digest for deduplicating indexed data.
func HashString(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		io.WriteString(h, part)
		io.WriteString(h, "\x00")
	}
	return hex.EncodeToString(h.Sum(nil))
}

// NormalizeSearchText lowercases and collapses whitespace for fast search.
func NormalizeSearchText(s string) string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	})
	return strings.Join(fields, " ")
}

// SearchTokens splits a query into normalized tokens.
func SearchTokens(q string) []string {
	q = NormalizeSearchText(q)
	if q == "" {
		return nil
	}
	return strings.Fields(q)
}

// FormatError keeps CLI errors compact and human-readable.
func FormatError(err error) string {
	if err == nil {
		return ""
	}
	return "devgrep: " + err.Error()
}

// IsTerminal reports whether f is attached to a terminal.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// HumanDuration returns a compact relative time string.
func HumanDuration(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	if now.IsZero() {
		now = time.Now()
	}
	d := now.Sub(t)
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 14*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 10*7*24*time.Hour:
		return fmt.Sprintf("%d weeks ago", int(d.Hours()/(24*7)))
	case d < 24*30*24*time.Hour:
		return fmt.Sprintf("%d months ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%d years ago", int(d.Hours()/(24*365)))
	}
}

// HumanBytes formats a byte count for terminal output.
func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// Truncate shortens s to width runes and appends an ellipsis when needed.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= width {
		return string(r)
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}
	return string(r[:width-3]) + "..."
}

// CopyText copies text to the system clipboard using native command-line tools.
func CopyText(text string) error {
	if text == "" {
		return errors.New("nothing to copy")
	}
	candidates := clipboardCandidates()
	var errs []string
	for _, candidate := range candidates {
		cmd := exec.Command(candidate.name, candidate.args...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			errs = append(errs, candidate.name+": "+err.Error())
		}
	}
	if len(errs) == 0 {
		return errors.New("no clipboard command found")
	}
	return fmt.Errorf("copy failed: %s", strings.Join(errs, "; "))
}

type clipboardCommand struct {
	name string
	args []string
}

func clipboardCandidates() []clipboardCommand {
	switch runtime.GOOS {
	case "darwin":
		return []clipboardCommand{{name: "pbcopy"}}
	case "windows":
		return []clipboardCommand{{name: "clip"}}
	default:
		return []clipboardCommand{
			{name: "wl-copy"},
			{name: "xclip", args: []string{"-selection", "clipboard"}},
			{name: "xsel", args: []string{"--clipboard", "--input"}},
		}
	}
}
