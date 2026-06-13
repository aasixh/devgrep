package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aasixh/devgrep/internal/config"
	searchpkg "github.com/aasixh/devgrep/internal/search"
	"github.com/aasixh/devgrep/internal/storage"
	"github.com/aasixh/devgrep/internal/utils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestModelRendersResultAndNavigation(t *testing.T) {
	m := newModel(context.Background(), searchpkg.Engine{}, config.Default(), "docker", searchpkg.Options{})
	m.width = 100
	m.height = 30
	m.searchMode = false
	m.results = []searchpkg.Result{
		{Document: storage.Document{SourceType: searchpkg.SourceHistory, Content: "docker compose up -d postgres", CWD: "/tmp", EventTime: time.Now()}, Score: 92},
		{Document: storage.Document{SourceType: searchpkg.SourceHistory, Content: "kubectl get pods", CWD: "/tmp", EventTime: time.Now()}, Score: 50},
	}
	view := m.View()
	if !strings.Contains(view, "Search: docker") || !strings.Contains(view, "docker compose") {
		t.Fatalf("view did not include expected content: %s", view)
	}
	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	updated := next.(model)
	if updated.selected != 1 {
		t.Fatalf("selected = %d, want 1", updated.selected)
	}
	updated.searchMode = true
	next, cmd := updated.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd == nil {
		t.Fatal("expected live search command")
	}
	updated = next.(model)
	if !strings.HasSuffix(updated.query, "x") {
		t.Fatalf("query = %q", updated.query)
	}
	next, cmd = updated.handleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	if cmd == nil {
		t.Fatal("expected clear search command")
	}
	updated = next.(model)
	if updated.query != "" {
		t.Fatalf("query after ctrl-u = %q", updated.query)
	}
	next, _ = updated.Update(resultsMsg{query: updated.query, results: updated.results})
	updated = next.(model)
	if updated.loading {
		t.Fatal("loading stayed true after results")
	}
	next, _ = updated.Update(copiedMsg{err: errors.New("copy failed")})
	updated = next.(model)
	if !strings.Contains(updated.status, "copy failed") {
		t.Fatalf("status = %q", updated.status)
	}
}

func TestViewFitsTerminalWithoutClippingSearchBar(t *testing.T) {
	m := newModel(context.Background(), searchpkg.Engine{}, config.Default(), "python3 test.py", searchpkg.Options{})
	m.width = 100
	m.height = 24
	m.loading = false
	m.searchMode = true
	m.cursorOn = true
	m.results = []searchpkg.Result{
		{Document: storage.Document{SourceType: searchpkg.SourceHistory, Content: "python3 test.py", CWD: "/tmp", EventTime: time.Now()}, Score: 90},
	}

	view := m.View()
	if got := lipgloss.Height(view); got > m.height {
		t.Fatalf("view height = %d, want <= %d:\n%s", got, m.height, view)
	}
	if !strings.Contains(view, "Search: python3 test.py|") {
		t.Fatalf("search query was not visible in fitted view:\n%s", view)
	}
}

func TestSearchBarPreservesSpacesInTypedQuery(t *testing.T) {
	m := newModel(context.Background(), searchpkg.Engine{}, config.Default(), "", searchpkg.Options{})
	m.width = 120
	m.height = 24
	m.loading = false
	m.searchMode = true
	m.cursorOn = true

	updated := m
	for _, r := range "python3 test.py" {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		if r == ' ' {
			msg = tea.KeyMsg{Type: tea.KeySpace}
		}
		next, _ := updated.Update(msg)
		updated = next.(model)
	}

	if updated.query != "python3 test.py" {
		t.Fatalf("query = %q, want python3 test.py", updated.query)
	}
	if view := updated.View(); !strings.Contains(view, "Search: python3 test.py|") {
		t.Fatalf("search bar did not preserve spaced query:\n%s", view)
	}
}

func TestSearchBarTracksActiveQueryThroughUpdate(t *testing.T) {
	m := newModel(context.Background(), searchpkg.Engine{}, config.Default(), "", searchpkg.Options{})
	m.width = 140
	m.height = 30
	m.loading = false
	m.searchMode = true
	m.cursorOn = true

	updated := m
	var cmd tea.Cmd
	for _, r := range "python" {
		next, nextCmd := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		cmd = nextCmd
		updated = next.(model)
	}
	if cmd == nil {
		t.Fatal("expected search command after typing")
	}
	if updated.query != "python" {
		t.Fatalf("query = %q, want python", updated.query)
	}
	view := updated.View()
	if !strings.Contains(view, "Search: python|") {
		t.Fatalf("search bar did not show typed query:\n%s", view)
	}
	if cwd, err := os.Getwd(); err == nil && !strings.Contains(view, utils.RelHome(cwd)) {
		t.Fatalf("search bar did not show current directory %q:\n%s", utils.RelHome(cwd), view)
	}

	next, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if cmd == nil {
		t.Fatal("expected search command after backspace")
	}
	updated = next.(model)
	if updated.query != "pytho" {
		t.Fatalf("query = %q, want pytho", updated.query)
	}
	if view := updated.View(); !strings.Contains(view, "Search: pytho|") {
		t.Fatalf("search bar did not show backspaced query:\n%s", view)
	}

	next, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if cmd == nil {
		t.Fatal("expected search command after ctrl-u")
	}
	updated = next.(model)
	if updated.query != "" {
		t.Fatalf("query = %q, want empty", updated.query)
	}
	if view := updated.View(); !strings.Contains(view, "Search: |") {
		t.Fatalf("search bar did not clear query:\n%s", view)
	}
}

func TestPreviewContextLinesAndEmptyState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\nfive\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	contextText := previewContextLines(path, 3, 80, 1)
	if !strings.Contains(contextText, "3> three") {
		t.Fatalf("context = %q", contextText)
	}
	m := newModel(context.Background(), searchpkg.Engine{}, config.Default(), "", searchpkg.Options{})
	m.width = 80
	m.height = 20
	m.loading = false
	m.emptyDB = true
	view := m.View()
	if !strings.Contains(view, "No index found") {
		t.Fatalf("view = %s", view)
	}
}

func TestModelInitSearchAndCopyCommand(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "devgrep.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.UpsertDocuments(ctx, []storage.Document{{
		SourceType: searchpkg.SourceHistory,
		SourceName: "bash",
		Content:    "docker compose up",
		Normalized: "docker compose up",
		EventTime:  time.Now(),
		Hash:       "tui-docker",
	}}); err != nil {
		t.Fatal(err)
	}
	m := newModel(ctx, searchpkg.Engine{Store: store, Config: config.Default()}, config.Default(), "docker", searchpkg.Options{Limit: 5})
	batch, ok := m.Init()().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("init message = %#v", batch)
	}
	msg := batch[0]()
	results, ok := msg.(resultsMsg)
	if !ok || len(results.results) == 0 {
		t.Fatalf("message = %#v", msg)
	}
	if msg := copyCmd("")(); msg.(copiedMsg).err == nil {
		t.Fatal("expected copy error")
	}
}
