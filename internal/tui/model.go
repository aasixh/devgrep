package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/aasixh/devgrep/internal/config"
	searchpkg "github.com/aasixh/devgrep/internal/search"
	"github.com/aasixh/devgrep/internal/utils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Run starts the interactive devgrep TUI.
func Run(ctx context.Context, engine searchpkg.Engine, cfg config.Config, query string, opts searchpkg.Options) error {
	opts.Record = false
	model := newModel(ctx, engine, cfg, query, opts)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := program.Run()
	return err
}

type model struct {
	ctx        context.Context
	engine     searchpkg.Engine
	cfg        config.Config
	opts       searchpkg.Options
	styles     styles
	query      string
	results    []searchpkg.Result
	selected   int
	width      int
	height     int
	searchMode bool
	status     string
	loading    bool
	emptyDB    bool
	cursorOn   bool
	pendingG   bool
	err        error
}

type resultsMsg struct {
	query   string
	results []searchpkg.Result
	err     error
	emptyDB bool
}

type cursorMsg struct{}

type copiedMsg struct {
	err error
}

type executedMsg struct {
	err error
}

func newModel(ctx context.Context, engine searchpkg.Engine, cfg config.Config, query string, opts searchpkg.Options) model {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	return model{
		ctx:        ctx,
		engine:     engine,
		cfg:        cfg,
		opts:       opts,
		styles:     newStyles(cfg),
		query:      query,
		searchMode: query == "",
		status:     "ready",
		loading:    true,
		cursorOn:   true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.searchCmd(m.query), blinkCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case cursorMsg:
		m.cursorOn = !m.cursorOn
		return m, blinkCmd()
	case resultsMsg:
		if msg.query != m.query {
			return m, nil
		}
		m.loading = false
		m.err = msg.err
		m.results = msg.results
		m.emptyDB = msg.emptyDB
		if m.selected >= len(m.results) {
			m.selected = max(0, len(m.results)-1)
		}
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			if len(m.results) == 0 && msg.emptyDB {
				m.status = "No index found. Run: devgrep index"
			} else if len(m.results) == 0 {
				m.status = "No results found."
			} else {
				m.status = fmt.Sprintf("%d result(s)", len(m.results))
			}
		}
		return m, nil
	case copiedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			m.status = "copied"
		}
		return m, nil
	case executedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			m.status = "command finished"
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.searchMode {
		switch msg.Type {
		case tea.KeyEsc:
			m.searchMode = false
			m.status = "search canceled"
			return m, nil
		case tea.KeyEnter:
			m.searchMode = false
			m.loading = true
			return m, m.searchCmd(m.query)
		case tea.KeyBackspace:
			return m.withQuery(backspaceQuery(m.query))
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyCtrlU:
			return m.withQuery("")
		case tea.KeySpace:
			return m.withQuery(m.query + " ")
		case tea.KeyRunes:
			return m.withQuery(m.query + string(msg.Runes))
		default:
			return m, nil
		}
	}

	if msg.Type == tea.KeyCtrlU {
		return m.withQuery("")
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		return m, tea.Quit
	case "/":
		m.searchMode = true
		m.status = "live search"
		return m, nil
	case "j", "down":
		m.pendingG = false
		if m.selected < len(m.results)-1 {
			m.selected++
		}
		return m, nil
	case "k", "up":
		m.pendingG = false
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
	case "G", "end":
		m.pendingG = false
		if len(m.results) > 0 {
			m.selected = len(m.results) - 1
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.selected = 0
			m.pendingG = false
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "home":
		m.selected = 0
		return m, nil
	case "y":
		m.pendingG = false
		if current, ok := m.current(); ok {
			return m, copyCmd(current.Document.Content)
		}
		return m, nil
	case "enter":
		m.pendingG = false
		if current, ok := m.current(); ok {
			if current.Document.SourceType != searchpkg.SourceHistory {
				m.status = "enter executes history commands only"
				return m, nil
			}
			return m, executeCmd(current.Document.Content)
		}
		return m, nil
	default:
		if msg.Type == tea.KeySpace {
			m.searchMode = true
			return m.withQuery(m.query + " ")
		}
		if msg.Type == tea.KeyRunes {
			m.searchMode = true
			return m.withQuery(m.query + string(msg.Runes))
		}
		m.pendingG = false
		return m, nil
	}
}

func (m model) withQuery(query string) (tea.Model, tea.Cmd) {
	if query == m.query && !m.loading {
		return m, nil
	}
	m.query = query
	m.loading = true
	return m, m.searchCmd(m.query)
}

func backspaceQuery(query string) string {
	if query == "" {
		return ""
	}
	r := []rune(query)
	return string(r[:len(r)-1])
}

func (m model) View() string {
	if m.width == 0 {
		return "loading devgrep..."
	}
	footer := m.renderFooter()
	bodyHeight := max(1, m.height-lipgloss.Height(footer))
	listWidth := clamp(m.width*45/100, 34, max(34, m.width-24))
	previewWidth := max(20, m.width-listWidth-1)
	searchBar := m.renderSearchBar(listWidth)
	listHeight := max(1, bodyHeight-lipgloss.Height(searchBar))
	leftPane := lipgloss.JoinVertical(
		lipgloss.Left,
		searchBar,
		m.renderList(listWidth, listHeight),
	)
	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPane,
		m.renderPreview(previewWidth, bodyHeight),
	)
	return lipgloss.JoinVertical(lipgloss.Left, body, footer)
}

func (m model) renderSearchBar(width int) string {
	cursor := " "
	if m.searchMode && m.cursorOn {
		cursor = "|"
	}

	queryText := "Search: " + m.query + cursor
	cwdText := utils.RelHome(currentWorkingDirectory())
	innerWidth := max(1, width-4)
	spacer := "  "

	line := m.styles.Query.Render(utils.Truncate(queryText, innerWidth))
	remaining := innerWidth - len([]rune(queryText)) - len([]rune(spacer))
	if cwdText != "" && remaining >= 8 {
		line = lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.styles.Query.Render(queryText),
			spacer,
			m.styles.Muted.Render(utils.Truncate(cwdText, remaining)),
		)
	}
	return m.styles.SearchBar.Width(width).Render(line)
}

func (m model) renderList(width, height int) string {
	innerWidth := max(10, width-4)
	rows := make([]string, 0, height)
	if m.loading {
		rows = append(rows, m.styles.Muted.Render("Searching local index..."))
	} else if len(m.results) == 0 {
		if m.emptyDB {
			rows = append(rows, m.styles.Section.Render("No index found."))
			rows = append(rows, m.styles.Muted.Render("Run: devgrep index"))
		} else {
			rows = append(rows, m.styles.Section.Render("No results found."))
			rows = append(rows, m.styles.Muted.Render("Try a broader query."))
		}
	}
	for i, result := range m.results {
		doc := result.Document
		prefix := "  "
		style := m.styles.ListItem
		if i == m.selected {
			prefix = "> "
			style = m.styles.Selected
		}
		label := "[" + doc.SourceType + "] "
		title := utils.Truncate(label+doc.Content, innerWidth-7)
		meta := fmt.Sprintf("score %d", result.Score)
		rows = append(rows, style.Width(innerWidth).Render(prefix+title))
		rows = append(rows, m.styles.Meta.Width(innerWidth).Render("  "+utils.Truncate(meta, innerWidth-2)))
		if len(rows) >= height-2 {
			break
		}
	}
	for len(rows) < height-2 {
		rows = append(rows, "")
	}
	return m.styles.Panel.Width(width).Height(panelContentHeight(height)).Render(strings.Join(rows, "\n"))
}

func (m model) renderPreview(width, height int) string {
	current, ok := m.current()
	if !ok {
		if m.emptyDB {
			return m.styles.Panel.Width(width).Height(panelContentHeight(height)).Render(m.styles.Section.Render("No index found.") + "\n" + m.styles.Muted.Render("Run:\ndevgrep index"))
		}
		return m.styles.Panel.Width(width).Height(panelContentHeight(height)).Render(m.styles.Section.Render("No results found.") + "\n" + m.styles.Muted.Render("Try a broader query."))
	}
	doc := current.Document
	innerWidth := max(10, width-4)
	sections := []string{
		m.styles.Section.Render("[" + doc.SourceType + "]"),
		wrap(doc.Content, innerWidth),
		"",
		m.styles.Section.Render("[score]"),
		fmt.Sprintf("%d", current.Score),
	}
	if doc.SourceType == searchpkg.SourceHistory {
		sections = append(sections,
			"",
			m.styles.Section.Render("[last used]"),
			utils.HumanDuration(doc.EventTime, m.engineNow()),
			"",
			m.styles.Section.Render("[directory]"),
			utils.RelHome(doc.CWD),
		)
	} else {
		source := utils.RelHome(doc.Path)
		if doc.Line > 0 {
			source = fmt.Sprintf("%s:%d", source, doc.Line)
		}
		sections = append(sections,
			"",
			m.styles.Section.Render("[source]"),
			source,
		)
		if doc.Severity != "" {
			sections = append(sections, "", m.styles.Section.Render("[severity]"), doc.Severity)
		}
		if context := previewContextLines(doc.Path, doc.Line, innerWidth, 2); context != "" {
			sections = append(sections, "", m.styles.Section.Render("[context]"), context)
		}
	}
	content := strings.Join(sections, "\n")
	return m.styles.Panel.Width(width).Height(panelContentHeight(height)).Render(content)
}

func (m model) renderFooter() string {
	keys := "/ search • enter open • y copy • esc quit"
	status := m.status
	if status == "" {
		status = "ready"
	}
	line := lipgloss.JoinHorizontal(lipgloss.Center, m.styles.Muted.Render(keys), "  ", m.styles.Status.Render(status))
	return m.styles.Footer.Width(m.width).Render(line)
}

func (m model) current() (searchpkg.Result, bool) {
	if m.selected < 0 || m.selected >= len(m.results) {
		return searchpkg.Result{}, false
	}
	return m.results[m.selected], true
}

func (m model) searchCmd(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := m.engine.Query(m.ctx, query, m.opts)
		emptyDB := false
		if err == nil && len(results) == 0 && m.engine.Store != nil {
			if count, countErr := m.engine.Store.DocumentCount(m.ctx); countErr == nil && count == 0 {
				emptyDB = true
			}
		}
		return resultsMsg{query: query, results: results, err: err, emptyDB: emptyDB}
	}
}

func blinkCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return cursorMsg{}
	})
}

func copyCmd(text string) tea.Cmd {
	return func() tea.Msg {
		return copiedMsg{err: utils.CopyText(text)}
	}
}

func executeCmd(command string) tea.Cmd {
	shell := os.Getenv("SHELL")
	args := []string{"-lc", command}
	if runtime.GOOS == "windows" {
		shell = os.Getenv("COMSPEC")
		args = []string{"/C", command}
	}
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell, args...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return executedMsg{err: err}
	})
}

func (m model) engineNow() time.Time {
	if m.engine.Now != nil {
		return m.engine.Now()
	}
	return time.Now()
}

func currentWorkingDirectory() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

func wrap(s string, width int) string {
	if width <= 0 {
		return ""
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	line := ""
	for _, word := range words {
		if len([]rune(word)) > width {
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
			lines = append(lines, utils.Truncate(word, width))
			continue
		}
		if line == "" {
			line = word
			continue
		}
		if len([]rune(line))+1+len([]rune(word)) > width {
			lines = append(lines, line)
			line = word
		} else {
			line += " " + word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func previewContextLines(path string, line int, width int, radius int) string {
	if path == "" || line <= 0 {
		return ""
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	start := line - radius
	if start < 1 {
		start = 1
	}
	end := line + radius
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var lines []string
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if lineNo < start {
			continue
		}
		if lineNo > end {
			break
		}
		prefix := fmt.Sprintf("%4d  ", lineNo)
		if lineNo == line {
			prefix = fmt.Sprintf("%4d> ", lineNo)
		}
		lines = append(lines, utils.Truncate(prefix+scanner.Text(), width))
	}
	return strings.Join(lines, "\n")
}

func panelContentHeight(totalHeight int) int {
	return max(1, totalHeight-2)
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
