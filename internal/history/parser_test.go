package history

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aasixh/devgrep/internal/config"
	searchpkg "github.com/aasixh/devgrep/internal/search"
	"github.com/aasixh/devgrep/internal/storage"
	"github.com/aasixh/devgrep/internal/utils"
)

const testHome = "/home/alice"

func TestParseBashTimestampsAndCWD(t *testing.T) {
	input := strings.NewReader("#1700000000\ncd projects/api\n#1700000100\ndocker compose up -d postgres\n")
	result, err := ParseBash(input, testHome, testHome)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 2 {
		t.Fatalf("records = %d, want 2", len(result.Records))
	}
	if result.Records[1].Command != "docker compose up -d postgres" {
		t.Fatalf("command = %q", result.Records[1].Command)
	}
	if result.Records[1].CWD != "/home/alice/projects/api" {
		t.Fatalf("cwd = %q", result.Records[1].CWD)
	}
	if got := result.Records[1].Timestamp.Unix(); got != 1700000100 {
		t.Fatalf("timestamp = %d", got)
	}
}

func TestParseZshExtendedHistory(t *testing.T) {
	input := strings.NewReader(": 1700000200:0;git status\n: 1700000300:0;cd /tmp\nmake test\n")
	result, err := ParseZsh(input, testHome, testHome)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 3 {
		t.Fatalf("records = %d, want 3", len(result.Records))
	}
	if !result.Records[0].Timestamp.Equal(time.Unix(1700000200, 0)) {
		t.Fatalf("timestamp = %v", result.Records[0].Timestamp)
	}
	if result.Records[2].CWD != "/tmp" {
		t.Fatalf("cwd = %q", result.Records[2].CWD)
	}
}

func TestParseZshExtendedHistoryWithEmbeddedCWD(t *testing.T) {
	input := strings.NewReader(": 1700000400:5;/home/alice/projects/api;git status\n")
	result, err := ParseZsh(input, testHome, testHome)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(result.Records))
	}
	if result.Records[0].Command != "git status" {
		t.Fatalf("command = %q", result.Records[0].Command)
	}
}

func TestApplyDirectoryCommandTildeExpansion(t *testing.T) {
	current, _ := applyDirectoryCommand("cd ~/work/portfolio", "/tmp", "/tmp", testHome)
	if current != "/home/alice/work/portfolio" {
		t.Fatalf("cwd = %q", current)
	}
	if utils.MalformedCWDPath(current) {
		t.Fatalf("malformed cwd: %q", current)
	}
}

func TestApplyDirectoryCommandHOMEExpansion(t *testing.T) {
	current, _ := applyDirectoryCommand("cd $HOME/work/devgrep", "/tmp", "/tmp", testHome)
	want := filepath.Join(testHome, "work", "devgrep")
	if current != want {
		t.Fatalf("cwd = %q, want %q", current, want)
	}
}

func TestApplyDirectoryCommandRelativeAfterTilde(t *testing.T) {
	current, _ := applyDirectoryCommand("cd ~/work/url-shortner", "/tmp", "/tmp", testHome)
	current, _ = applyDirectoryCommand("cd work/algo-lang", current, "/tmp", testHome)
	want := filepath.Join(testHome, "work", "url-shortner", "work", "algo-lang")
	if current != want {
		t.Fatalf("cwd = %q, want %q", current, want)
	}
	if strings.Contains(current, "/~/") {
		t.Fatalf("chained tilde path: %q", current)
	}
}

func TestApplyDirectoryCommandQuotedPath(t *testing.T) {
	current, _ := applyDirectoryCommand(`cd "~/work/my project"`, "/tmp", "/tmp", testHome)
	want := filepath.Join(testHome, "work", "my project")
	if current != want {
		t.Fatalf("cwd = %q, want %q", current, want)
	}
}

func TestApplyDirectoryCommandMalformedDoesNotCorrupt(t *testing.T) {
	current := "/tmp/~/work/url-shortner"
	next, _ := applyDirectoryCommand("cd work/nvim", current, "/tmp", testHome)
	if next != current {
		t.Fatalf("expected cwd to remain %q, got %q", current, next)
	}
}

func TestParseBashDoesNotProduceChainedTildePaths(t *testing.T) {
	input := strings.NewReader(strings.Join([]string{
		"cd /tmp",
		"cd ~/work/url-shortner",
		"cd work/algo-lang",
		"cd work/nvim",
		"cd algo-lang",
		"cd ~/work/work/algo-lang",
		"git status",
	}, "\n") + "\n")
	result, err := ParseBash(input, testHome, testHome)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range result.Records {
		if utils.MalformedCWDPath(record.CWD) {
			t.Fatalf("malformed cwd %q for command %q", record.CWD, record.Command)
		}
		if strings.Contains(record.CWD, "/~/") {
			t.Fatalf("chained cwd %q for command %q", record.CWD, record.Command)
		}
	}
	wantEnd := filepath.Join(testHome, "work", "work", "algo-lang")
	if result.EndCWD != wantEnd {
		t.Fatalf("EndCWD = %q, want %q", result.EndCWD, wantEnd)
	}
}

func TestResumeCWDRejectsMalformedState(t *testing.T) {
	got := ResumeCWD("/tmp/~/work/url-shortner", testHome)
	if got != testHome {
		t.Fatalf("ResumeCWD = %q, want %q", got, testHome)
	}
}

func TestSanitizeRecordCWD(t *testing.T) {
	if got := SanitizeRecordCWD("/tmp/~/work/foo", testHome); got != "" {
		t.Fatalf("SanitizeRecordCWD = %q, want empty", got)
	}
	if got := SanitizeRecordCWD(filepath.Join(testHome, "work", "devgrep"), testHome); got != filepath.Join(testHome, "work", "devgrep") {
		t.Fatalf("SanitizeRecordCWD = %q", got)
	}
}

func TestShellHistoryIndexerReplayAfterRepair(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv("HOME", home)
	historyPath := filepath.Join(home, ".bash_history")
	if err := os.WriteFile(historyPath, []byte("#1700000000\ncd project\nmake test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(ctx, filepath.Join(home, "devgrep.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	corrupt := "/tmp/~/work/bad"
	_, err = store.UpsertDocuments(ctx, []storage.Document{{
		SourceType: searchpkg.SourceHistory,
		SourceName: "bash",
		Content:    "stale command",
		Normalized: "stale command " + corrupt,
		CWD:        corrupt,
		Path:       historyPath,
		EventTime:  time.Now(),
		Hash:       "stale-row",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSourceState(ctx, storage.SourceState{
		SourceName: "bash",
		Path:       historyPath,
		Size:       int64(len("#1700000000\ncd project\nmake test\n")),
		ModTime:    time.Now(),
		LineOffset: 99,
		Metadata:   map[string]string{"cwd": corrupt},
	}); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.DatabasePath = filepath.Join(home, "devgrep.db")
	idx := &ShellHistoryIndexer{Store: store, Config: cfg, Home: home}
	if err := idx.Index(ctx); err != nil {
		t.Fatal(err)
	}
	if idx.LastCount() == 0 {
		t.Fatal("expected history replay to index records")
	}
	results, err := idx.Search(ctx, "make test")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results after replay")
	}
	if utils.InvalidHistoryCWD(results[0].Document.CWD) {
		t.Fatalf("cwd still invalid after replay: %q", results[0].Document.CWD)
	}
}

func TestShellHistoryIndexerIndexesAndSearches(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), []byte("#1700000000\ncd project\nmake test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(ctx, filepath.Join(home, "devgrep.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := config.Default()
	cfg.DatabasePath = filepath.Join(home, "devgrep.db")
	idx := &ShellHistoryIndexer{Store: store, Config: cfg, Home: home}
	if idx.Name() == "" {
		t.Fatal("empty name")
	}
	if err := idx.Index(ctx); err != nil {
		t.Fatal(err)
	}
	if idx.LastCount() != 2 {
		t.Fatalf("count = %d", idx.LastCount())
	}
	results, err := idx.Search(ctx, "make test")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 || results[0].Document.SourceType != searchpkg.SourceHistory {
		t.Fatalf("results = %#v", results)
	}
	if utils.MalformedCWDPath(results[0].Document.CWD) {
		t.Fatalf("stored cwd = %q", results[0].Document.CWD)
	}
}
