package tui

import (
	"github.com/aasixh/devgrep/internal/config"
	"github.com/charmbracelet/lipgloss"
)

type styles struct {
	Header    lipgloss.Style
	Footer    lipgloss.Style
	Panel     lipgloss.Style
	SearchBar lipgloss.Style
	Title     lipgloss.Style
	Query     lipgloss.Style
	Muted     lipgloss.Style
	Status    lipgloss.Style
	Section   lipgloss.Style
	ListItem  lipgloss.Style
	Selected  lipgloss.Style
	Meta      lipgloss.Style
}

func newStyles(cfg config.Config) styles {
	accent := lipgloss.Color(cfg.TUI.Accent)
	muted := lipgloss.Color(cfg.TUI.Muted)
	return styles{
		Header: lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("#111827")),
		Footer: lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("#111827")),
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")).
			Padding(0, 1),
		SearchBar: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")).
			Padding(0, 1),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent),
		Query: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")),
		Muted: lipgloss.NewStyle().
			Foreground(muted),
		Status: lipgloss.NewStyle().
			Foreground(accent),
		Section: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent),
		ListItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#111827")).
			Background(accent).
			Bold(true),
		Meta: lipgloss.NewStyle().
			Foreground(muted),
	}
}
