// Package tui is the terminal UI for praxis, built on Bubble Tea.
package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/michael-duren/praxis/internal/domain"
	"github.com/michael-duren/praxis/internal/harness"
	"github.com/michael-duren/praxis/internal/orchestrator"
	"github.com/michael-duren/praxis/internal/store"
)

type tab int

const (
	tabSkills tab = iota
	tabContext
	tabAgentSkills
	tabAutonomy
	tabHarnesses
	tabCount
)

var tabNames = [tabCount]string{"Skills", "Context", "Agent Skills", "Autonomy", "Harnesses"}

// Model is the Bubble Tea model for the praxis TUI.
type Model struct {
	st       *store.Store
	adapters []harness.Adapter

	tab      tab
	cursor   int
	themeIdx int
	styles   styles
	width    int
	status   string

	skills      []domain.UserSkill
	entries     []domain.ContextEntry
	agentSkills []domain.AgentSkill
	enabled     map[string]bool
	settings    domain.Settings
}

// New builds the TUI model, loading all state from the store.
func New(st *store.Store, adapters []harness.Adapter) (Model, error) {
	m := Model{st: st, adapters: adapters, styles: newStyles(Themes[0])}
	if err := m.reload(); err != nil {
		return m, err
	}
	return m, nil
}

// Run starts the TUI event loop.
func Run(st *store.Store, adapters []harness.Adapter) error {
	m, err := New(st, adapters)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m *Model) reload() error {
	var err error
	if m.skills, err = m.st.Skills(); err != nil {
		return err
	}
	if m.entries, err = m.st.ContextEntries(); err != nil {
		return err
	}
	if m.enabled, err = m.st.EnabledHarnesses(); err != nil {
		return err
	}
	if m.settings, err = m.st.LoadSettings(); err != nil {
		return err
	}
	m.agentSkills = nil
	for _, a := range m.adapters {
		if l, ok := a.(harness.SkillLister); ok {
			skills, err := l.ListSkills()
			if err != nil {
				continue // a broken skills dir shouldn't kill the UI
			}
			m.agentSkills = append(m.agentSkills, skills...)
		}
	}
	return nil
}

func (m Model) Init() tea.Cmd { return nil }

// rowCount is how many selectable rows the current tab has.
func (m Model) rowCount() int {
	switch m.tab {
	case tabSkills:
		return len(m.skills)
	case tabContext:
		return len(m.entries)
	case tabAgentSkills:
		return len(m.agentSkills)
	case tabAutonomy:
		return 1
	case tabHarnesses:
		return len(m.adapters)
	}
	return 0
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "l", "right":
			m.tab = (m.tab + 1) % tabCount
			m.cursor = 0
		case "shift+tab", "h", "left":
			m.tab = (m.tab + tabCount - 1) % tabCount
			m.cursor = 0
		case "j", "down":
			if m.cursor < m.rowCount()-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "t":
			m.themeIdx = (m.themeIdx + 1) % len(Themes)
			m.styles = newStyles(Themes[m.themeIdx])
			m.status = "theme: " + Themes[m.themeIdx].Name
		case "+", "=":
			m.adjustRank(+1)
		case "-":
			m.adjustRank(-1)
		case " ", "enter":
			m.toggle()
		case "s":
			m.sync()
		}
	}
	return m, nil
}

// adjustRank bumps the selected skill's rank on the Skills tab.
func (m *Model) adjustRank(delta int) {
	if m.tab != tabSkills || m.cursor >= len(m.skills) {
		return
	}
	sk := m.skills[m.cursor]
	r := sk.Rank + domain.Rank(delta)
	if r < domain.RankNovice || r > domain.RankExpert {
		return
	}
	sk.Rank = r
	if _, err := m.st.UpsertSkill(sk); err != nil {
		m.status = "error: " + err.Error()
		return
	}
	m.reload()
	m.status = fmt.Sprintf("%s → %s", sk.Name, sk.Rank)
}

// toggle flips the selected harness or cycles the autonomy mode.
func (m *Model) toggle() {
	switch m.tab {
	case tabHarnesses:
		if m.cursor >= len(m.adapters) {
			return
		}
		name := m.adapters[m.cursor].Name()
		if err := m.st.SetHarnessEnabled(name, !m.enabled[name]); err != nil {
			m.status = "error: " + err.Error()
			return
		}
		m.reload()
	case tabAutonomy:
		next := (m.settings.GlobalAutonomy + 1) % 3
		if err := m.st.SaveSettings(domain.Settings{GlobalAutonomy: next}); err != nil {
			m.status = "error: " + err.Error()
			return
		}
		m.reload()
		m.status = "autonomy: " + next.String()
	}
}

// sync pushes the rendered context to all enabled harnesses (global scope
// plus each repo that has context entries).
func (m *Model) sync() {
	st := orchestrator.State{Skills: m.skills, Entries: m.entries, Settings: m.settings}
	scopes := map[domain.Scope]bool{{}: true}
	for _, e := range m.entries {
		scopes[e.Scope] = true
	}
	var n int
	for scope := range scopes {
		for _, r := range orchestrator.Sync(st, scope, m.adapters, m.enabled) {
			if r.Err != nil {
				m.status = fmt.Sprintf("sync error (%s): %v", r.Harness, r.Err)
				return
			}
			n++
		}
	}
	m.status = fmt.Sprintf("synced %d file(s)", n)
}

func (m Model) View() string {
	s := m.styles
	var b strings.Builder

	b.WriteString(s.title.Render("praxis") + s.muted.Render("  — learn while you build") + "\n\n")

	var tabs []string
	for i, name := range tabNames {
		if tab(i) == m.tab {
			tabs = append(tabs, s.tabActive.Render(name))
		} else {
			tabs = append(tabs, s.tabIdle.Render(name))
		}
	}
	b.WriteString(strings.Join(tabs, " ") + "\n\n")

	b.WriteString(s.pane.Render(m.viewTab()) + "\n")

	if m.status != "" {
		b.WriteString(s.warning.Render(m.status) + "\n")
	}
	b.WriteString(s.muted.Render("h/l tabs · j/k move · +/- rank · space toggle · s sync · t theme · q quit"))
	return b.String()
}

func (m Model) viewTab() string {
	s := m.styles
	var b strings.Builder

	line := func(i int, text string) {
		prefix := "  "
		style := s.item
		if i == m.cursor {
			prefix = "> "
			style = s.selected
		}
		b.WriteString(style.Render(prefix+text) + "\n")
	}

	switch m.tab {
	case tabSkills:
		if len(m.skills) == 0 {
			return s.muted.Render("No skills yet. Add one: praxis skill add <name> --rank <rank>")
		}
		for i, sk := range m.skills {
			bar := strings.Repeat("█", int(sk.Rank)+1) + strings.Repeat("░", 4-int(sk.Rank))
			line(i, fmt.Sprintf("%-16s %s %s  (%s)", sk.Name, bar, sk.Rank, sk.Category))
		}
	case tabContext:
		if len(m.entries) == 0 {
			return s.muted.Render("No context entries. Add one: praxis context add <title>")
		}
		for i, e := range m.entries {
			line(i, fmt.Sprintf("%-24s [%s]", e.Title, e.Scope))
		}
	case tabAgentSkills:
		if len(m.agentSkills) == 0 {
			return s.muted.Render("No agent skills found in enabled harness directories.")
		}
		for i, sk := range m.agentSkills {
			line(i, fmt.Sprintf("%-24s [%s]", sk.Name, sk.Harness))
		}
	case tabAutonomy:
		line(0, fmt.Sprintf("Global autonomy: %s (space to cycle)", m.settings.GlobalAutonomy))
		b.WriteString("\n" + s.muted.Render("manual: you type everything · guided: rank-based · full: agents edit freely"))
	case tabHarnesses:
		for i, a := range m.adapters {
			mark := s.warning.Render("○ disabled")
			if m.enabled[a.Name()] {
				mark = s.success.Render("● enabled")
			}
			detected := ""
			if a.Detect() {
				detected = " (detected)"
			}
			line(i, fmt.Sprintf("%-12s %s%s", a.Name(), mark, detected))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// Home returns the user's home dir, used by main to build adapters.
func Home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}
