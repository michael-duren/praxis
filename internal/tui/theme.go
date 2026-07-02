package tui

import "github.com/charmbracelet/lipgloss"

// Theme is a named set of colors the TUI styles are built from.
type Theme struct {
	Name       string
	Background lipgloss.Color
	Foreground lipgloss.Color
	Accent     lipgloss.Color // highlights, active tab
	Muted      lipgloss.Color // borders, help text
	Success    lipgloss.Color
	Warning    lipgloss.Color
}

// Themes are the built-in themes, in cycle order.
var Themes = []Theme{
	{
		Name:       "dracula",
		Background: "#282a36",
		Foreground: "#f8f8f2",
		Accent:     "#bd93f9",
		Muted:      "#6272a4",
		Success:    "#50fa7b",
		Warning:    "#ffb86c",
	},
	{
		Name:       "tokyo-night",
		Background: "#1a1b26",
		Foreground: "#c0caf5",
		Accent:     "#7aa2f7",
		Muted:      "#565f89",
		Success:    "#9ece6a",
		Warning:    "#e0af68",
	},
	{
		Name:       "gruvbox",
		Background: "#282828",
		Foreground: "#ebdbb2",
		Accent:     "#fabd2f",
		Muted:      "#928374",
		Success:    "#b8bb26",
		Warning:    "#fe8019",
	},
	{
		Name:       "nord",
		Background: "#2e3440",
		Foreground: "#eceff4",
		Accent:     "#88c0d0",
		Muted:      "#4c566a",
		Success:    "#a3be8c",
		Warning:    "#ebcb8b",
	},
	{
		Name:       "catppuccin",
		Background: "#1e1e2e",
		Foreground: "#cdd6f4",
		Accent:     "#cba6f7",
		Muted:      "#6c7086",
		Success:    "#a6e3a1",
		Warning:    "#fab387",
	},
	{
		Name:       "dark",
		Background: "#0f172a",
		Foreground: "#e2e8f0",
		Accent:     "#38bdf8",
		Muted:      "#475569",
		Success:    "#4ade80",
		Warning:    "#facc15",
	},
	{
		Name:       "light",
		Background: "#ffffff",
		Foreground: "#1e293b",
		Accent:     "#1865f2", // Khan Academy blue
		Muted:      "#94a3b8",
		Success:    "#16a34a",
		Warning:    "#d97706",
	},
}

// ThemeByName returns the named theme, falling back to the first theme.
func ThemeByName(name string) Theme {
	for _, t := range Themes {
		if t.Name == name {
			return t
		}
	}
	return Themes[0]
}

// styles holds the lipgloss styles derived from a theme.
type styles struct {
	title     lipgloss.Style
	tabActive lipgloss.Style
	tabIdle   lipgloss.Style
	item      lipgloss.Style
	selected  lipgloss.Style
	muted     lipgloss.Style
	success   lipgloss.Style
	warning   lipgloss.Style
	pane      lipgloss.Style
}

func newStyles(t Theme) styles {
	return styles{
		title:     lipgloss.NewStyle().Bold(true).Foreground(t.Accent),
		tabActive: lipgloss.NewStyle().Bold(true).Foreground(t.Background).Background(t.Accent).Padding(0, 1),
		tabIdle:   lipgloss.NewStyle().Foreground(t.Muted).Padding(0, 1),
		item:      lipgloss.NewStyle().Foreground(t.Foreground),
		selected:  lipgloss.NewStyle().Bold(true).Foreground(t.Accent),
		muted:     lipgloss.NewStyle().Foreground(t.Muted),
		success:   lipgloss.NewStyle().Foreground(t.Success),
		warning:   lipgloss.NewStyle().Foreground(t.Warning),
		pane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Muted).
			Padding(1, 2),
	}
}
