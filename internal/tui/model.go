// Package tui is the terminal UI for praxis, built on Bubble Tea. One
// tab is shown at a time in a full-screen pane; long lists scroll, with
// the cursor kept in view and a position indicator beside the tab bar.
package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/michael-duren/praxis/internal/domain"
	"github.com/michael-duren/praxis/internal/harness"
	"github.com/michael-duren/praxis/internal/orchestrator"
	"github.com/michael-duren/praxis/internal/store"
)

// section identifies one tab. h/l/tab cycle through them in order.
type section int

const (
	secSkills section = iota
	secContext
	secAgentSkills
	secAutonomy
	secHarnesses
	sectionCount
)

var sectionNames = [sectionCount]string{"Skills", "Context", "Agent Skills", "Autonomy", "Harnesses"}

// Model is the Bubble Tea model for the praxis TUI.
type Model struct {
	st       *store.Store
	adapters []harness.Adapter

	focus    section // active tab
	cursor   int
	offset   int // first visible row of the active tab's list
	themeIdx int
	styles   styles
	width    int
	height   int
	status   string

	// Context-entry editing (a/e on the Context tab): a three-field
	// form (title, body, repo) filled one line at a time.
	editing   bool
	editNew   bool
	editFld   int
	editEntry domain.ContextEntry
	editBuf   string

	// d must be pressed twice on the same entry to delete it.
	pendingDelete int64

	// viewing shows the selected context entry's full body (enter on
	// the Context tab); j/k flip entries, e edits, esc closes.
	viewing bool

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

// rowCount is how many selectable rows the active tab has.
func (m Model) rowCount() int {
	switch m.focus {
	case secSkills:
		return len(m.skills)
	case secContext:
		return len(m.entries)
	case secAgentSkills:
		return len(m.agentSkills)
	case secAutonomy:
		return 1
	case secHarnesses:
		return len(m.adapters)
	}
	return 0
}

// effWidth/effHeight fall back to a sane size before the first
// WindowSizeMsg arrives (also what the tests render with).
func (m Model) effWidth() int {
	if m.width <= 0 {
		return 100
	}
	return m.width
}

func (m Model) effHeight() int {
	if m.height <= 0 {
		return 24
	}
	return m.height
}

// paneHeight is how many list rows fit in the pane: total height minus
// header, tab bar, two border lines, and the footer.
func (m Model) paneHeight() int {
	return max(3, m.effHeight()-5)
}

// scroll keeps the cursor inside the visible window.
func (m *Model) scroll() {
	visible := m.paneHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
	m.offset = max(0, min(m.offset, m.rowCount()-visible))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scroll()
	case tea.KeyMsg:
		if m.editing {
			return m.handleEditKey(msg), nil
		}
		if m.viewing {
			return m.handleViewKey(msg), nil
		}
		if msg.String() != "d" {
			m.pendingDelete = 0
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "l", "right":
			m.focus = (m.focus + 1) % sectionCount
			m.cursor, m.offset = 0, 0
		case "shift+tab", "h", "left":
			m.focus = (m.focus + sectionCount - 1) % sectionCount
			m.cursor, m.offset = 0, 0
		case "j", "down":
			if m.cursor < m.rowCount()-1 {
				m.cursor++
				m.scroll()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.scroll()
			}
		case "g":
			m.cursor = 0
			m.scroll()
		case "G":
			m.cursor = max(0, m.rowCount()-1)
			m.scroll()
		case "t":
			m.themeIdx = (m.themeIdx + 1) % len(Themes)
			m.styles = newStyles(Themes[m.themeIdx])
			m.status = "theme: " + Themes[m.themeIdx].Name
		case "+", "=":
			m.adjustRank(+1)
		case "-":
			m.adjustRank(-1)
		case " ", "enter":
			if m.focus == secContext {
				if m.cursor < len(m.entries) {
					m.viewing = true
				}
			} else {
				m.toggle()
			}
		case "a":
			if m.focus == secContext {
				m.editing, m.editNew, m.editFld = true, true, 0
				m.editEntry, m.editBuf = domain.ContextEntry{}, ""
			}
		case "e":
			if m.focus == secContext && m.cursor < len(m.entries) {
				m.editing, m.editNew, m.editFld = true, false, 0
				m.editEntry = m.entries[m.cursor]
				m.editBuf = m.editEntry.Title
			}
		case "d":
			m.deleteContext()
		case "s":
			m.sync()
		}
	}
	return m, nil
}

// handleViewKey drives the context detail view: j/k flip entries, e
// jumps into editing the shown entry, anything closing-ish closes.
func (m Model) handleViewKey(key tea.KeyMsg) Model {
	switch key.String() {
	case "j", "down":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			m.scroll()
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.scroll()
		}
	case "e":
		if m.cursor < len(m.entries) {
			m.viewing = false
			m.editing, m.editNew, m.editFld = true, false, 0
			m.editEntry = m.entries[m.cursor]
			m.editBuf = m.editEntry.Title
		}
	case "esc", "enter", "q", " ":
		m.viewing = false
	}
	return m
}

// handleEditKey runs the context-entry form: enter commits the current
// field and advances (saving after the last), esc cancels.
func (m Model) handleEditKey(key tea.KeyMsg) Model {
	switch key.Type {
	case tea.KeyEsc:
		m.editing = false
		m.status = "edit cancelled"
	case tea.KeyBackspace:
		if r := []rune(m.editBuf); len(r) > 0 {
			m.editBuf = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		m.editBuf += " "
	case tea.KeyRunes:
		m.editBuf += string(key.Runes)
	case tea.KeyEnter:
		val := strings.TrimSpace(m.editBuf)
		switch m.editFld {
		case 0:
			if val == "" {
				m.status = "title is required"
				return m
			}
			m.editEntry.Title = val
			m.editFld, m.editBuf = 1, m.editEntry.Body
		case 1:
			if val == "" {
				m.status = "body is required"
				return m
			}
			m.editEntry.Body = val
			m.editFld, m.editBuf = 2, m.editEntry.Scope.Repo
		case 2:
			m.editEntry.Scope.Repo = val
			m.editing = false
			if _, err := m.st.UpsertContextEntry(m.editEntry); err != nil {
				m.status = "error: " + err.Error()
				return m
			}
			if err := m.reload(); err != nil {
				m.status = "error: " + err.Error()
				return m
			}
			verb := "updated"
			if m.editNew {
				verb = "added"
			}
			m.status = fmt.Sprintf("%s context %q", verb, m.editEntry.Title)
		}
	}
	return m
}

// deleteContext removes the selected entry after a confirming second d.
func (m *Model) deleteContext() {
	if m.focus != secContext || m.cursor >= len(m.entries) {
		return
	}
	e := m.entries[m.cursor]
	if m.pendingDelete != e.ID {
		m.pendingDelete = e.ID
		m.status = fmt.Sprintf("press d again to delete %q", e.Title)
		return
	}
	m.pendingDelete = 0
	if err := m.st.DeleteContextEntry(e.ID); err != nil {
		m.status = "error: " + err.Error()
		return
	}
	if err := m.reload(); err != nil {
		m.status = "error: " + err.Error()
		return
	}
	if m.cursor >= len(m.entries) && m.cursor > 0 {
		m.cursor--
	}
	m.scroll()
	m.status = fmt.Sprintf("deleted %q", e.Title)
}

// adjustRank bumps the selected skill's rank on the Skills tab.
func (m *Model) adjustRank(delta int) {
	if m.focus != secSkills || m.cursor >= len(m.skills) {
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
	if err := m.reload(); err != nil {
		m.status = "error: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("%s → %s", sk.Name, sk.Rank)
}

// toggle flips the selected harness or agent skill, or cycles autonomy.
func (m *Model) toggle() {
	switch m.focus {
	case secAgentSkills:
		m.toggleAgentSkill()
	case secHarnesses:
		if m.cursor >= len(m.adapters) {
			return
		}
		name := m.adapters[m.cursor].Name()
		if err := m.st.SetHarnessEnabled(name, !m.enabled[name]); err != nil {
			m.status = "error: " + err.Error()
			return
		}
		if err := m.reload(); err != nil {
			m.status = "error: " + err.Error()
		}
	case secAutonomy:
		next := (m.settings.GlobalAutonomy + 1) % 3
		if err := m.st.SaveSettings(domain.Settings{GlobalAutonomy: next}); err != nil {
			m.status = "error: " + err.Error()
			return
		}
		if err := m.reload(); err != nil {
			m.status = "error: " + err.Error()
			return
		}
		m.status = "autonomy: " + next.String()
	}
}

// toggleAgentSkill enables/disables the selected agent skill by moving
// its directory via the owning adapter's SkillToggler capability.
func (m *Model) toggleAgentSkill() {
	if m.cursor >= len(m.agentSkills) {
		return
	}
	sk := m.agentSkills[m.cursor]
	for _, a := range m.adapters {
		if a.Name() != sk.Harness {
			continue
		}
		tg, ok := a.(harness.SkillToggler)
		if !ok {
			m.status = sk.Harness + " does not support toggling skills"
			return
		}
		if err := tg.SetSkillEnabled(sk.Name, !sk.Enabled); err != nil {
			m.status = "error: " + err.Error()
			return
		}
		if err := m.reload(); err != nil {
			m.status = "error: " + err.Error()
			return
		}
		state := "disabled"
		if !sk.Enabled {
			state = "enabled"
		}
		m.status = fmt.Sprintf("%s %s (%s)", sk.Name, state, sk.Harness)
		return
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

// window is the visible slice bounds of the active tab's list.
func (m Model) window() (int, int) {
	start := m.offset
	end := min(m.rowCount(), start+m.paneHeight())
	return start, end
}

// marker prefixes the cursor row.
func (m Model) marker(i int) (string, lipgloss.Style) {
	if i == m.cursor {
		return "> ", m.styles.selected
	}
	return "  ", m.styles.item
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}

func (m Model) viewSkills(width int) string {
	if len(m.skills) == 0 {
		return m.styles.muted.Render("No skills yet — run `praxis setup` or: praxis skill add <name> <rank>")
	}
	// Row layout: marker(2) name(nameW) sp bar(10) sp rank(12) sp category(catW).
	// The fixed pieces cost 27 columns; name and category share the rest.
	budget := width - 27
	nameW := max(12, min(28, budget-18))
	catW := max(0, budget-nameW)
	var b strings.Builder
	start, end := m.window()
	for i := start; i < end; i++ {
		sk := m.skills[i]
		prefix, style := m.marker(i)
		bar := m.styles.success.Render(strings.Repeat("█", (int(sk.Rank)+1)*2)) +
			m.styles.muted.Render(strings.Repeat("░", (4-int(sk.Rank))*2))
		b.WriteString(style.Render(fmt.Sprintf("%s%-*s ", prefix, nameW, truncate(sk.Name, nameW))))
		b.WriteString(bar)
		b.WriteString(style.Render(fmt.Sprintf(" %-12s %s", sk.Rank, truncate(sk.Category, catW))))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// viewContextEdit renders the three-field entry form, highlighting the
// field being typed and dimming the ones not reached yet.
func (m Model) viewContextEdit() string {
	s := m.styles
	labels := [3]string{"Title:", "Body: ", "Repo: "}
	values := [3]string{m.editEntry.Title, m.editEntry.Body, m.editEntry.Scope.Repo}
	var b strings.Builder
	head := "edit context entry"
	if m.editNew {
		head = "new context entry"
	}
	b.WriteString(s.title.Render(head))
	b.WriteString("\n\n")
	for i := range labels {
		switch {
		case i < m.editFld: // committed
			b.WriteString(s.muted.Render("  " + labels[i] + " "))
			b.WriteString(s.item.Render(values[i]))
		case i == m.editFld: // being typed
			b.WriteString(s.selected.Render("> " + labels[i] + " "))
			b.WriteString(s.item.Render(m.editBuf))
			b.WriteString(s.selected.Render("▌"))
			if i == 2 && m.editBuf == "" {
				b.WriteString(s.muted.Render(" (empty = global)"))
			}
		default: // not reached yet
			b.WriteString(s.muted.Render("  " + labels[i] + " " + values[i]))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(s.muted.Render("enter next/save · esc cancel"))
	return b.String()
}

// viewContextDetail is the read-only full view of one entry.
func (m Model) viewContextDetail(width int) string {
	s := m.styles
	e := m.entries[m.cursor]
	var b strings.Builder
	b.WriteString(s.title.Render(e.Title))
	b.WriteString("\n")
	meta := fmt.Sprintf("%s · updated %s · %d/%d", e.Scope, e.Updated.Format("2006-01-02 15:04"), m.cursor+1, len(m.entries))
	b.WriteString(s.muted.Render(meta))
	b.WriteString("\n\n")
	b.WriteString(m.styles.item.Width(width).Render(e.Body))
	b.WriteString("\n\n")
	b.WriteString(s.muted.Render("j/k next/prev · e edit · esc close"))
	return b.String()
}

func (m Model) viewContext(width int) string {
	if m.editing {
		return m.viewContextEdit()
	}
	if m.viewing && m.cursor < len(m.entries) {
		return m.viewContextDetail(width)
	}
	if len(m.entries) == 0 {
		return m.styles.muted.Render("No context entries. Press a to add one.")
	}
	var b strings.Builder
	start, end := m.window()
	for i := start; i < end; i++ {
		e := m.entries[i]
		prefix, style := m.marker(i)
		b.WriteString(style.Render(fmt.Sprintf("%s%-28s", prefix, truncate(e.Title, 28))))
		b.WriteString(m.styles.muted.Render(" " + e.Scope.String()))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) viewAgentSkills() string {
	if len(m.agentSkills) == 0 {
		return m.styles.muted.Render("No agent skills found in harness directories.")
	}
	var b strings.Builder
	start, end := m.window()
	for i := start; i < end; i++ {
		sk := m.agentSkills[i]
		prefix, style := m.marker(i)
		mark := m.styles.warning.Render("○ off")
		if sk.Enabled {
			mark = m.styles.success.Render("● on ")
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%-28s ", prefix, truncate(sk.Name, 28))))
		b.WriteString(mark)
		b.WriteString(m.styles.muted.Render("  " + sk.Harness))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) viewAutonomy() string {
	prefix, style := m.marker(0)
	var b strings.Builder
	b.WriteString(style.Render(prefix + "global: "))
	b.WriteString(m.styles.warning.Render(m.settings.GlobalAutonomy.String()))
	b.WriteString("\n")
	b.WriteString(m.styles.muted.Render("  space to cycle: manual → guided → full"))
	b.WriteString("\n\n")
	b.WriteString(m.styles.muted.Render("  manual: you type everything · guided: rank-based · full: agents edit freely"))
	return b.String()
}

func (m Model) viewHarnesses() string {
	var b strings.Builder
	start, end := m.window()
	for i := start; i < end; i++ {
		a := m.adapters[i]
		prefix, style := m.marker(i)
		mark := m.styles.warning.Render("○ off")
		if m.enabled[a.Name()] {
			mark = m.styles.success.Render("● on ")
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%-12s ", prefix, a.Name())))
		b.WriteString(mark)
		if a.Detect() {
			b.WriteString(m.styles.muted.Render(" detected"))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) viewSection(width int) string {
	switch m.focus {
	case secSkills:
		return m.viewSkills(width)
	case secContext:
		return m.viewContext(width)
	case secAgentSkills:
		return m.viewAgentSkills()
	case secAutonomy:
		return m.viewAutonomy()
	case secHarnesses:
		return m.viewHarnesses()
	}
	return ""
}

// footerKeys is the tab-specific slice of the key help line.
func (m Model) footerKeys() string {
	switch m.focus {
	case secSkills:
		return "+/- rank"
	case secContext:
		return "enter view · a add · e edit · d delete"
	case secAutonomy:
		return "space cycle"
	default: // agent skills, harnesses
		return "space toggle"
	}
}

func (m Model) View() string {
	s := m.styles
	width := m.effWidth()

	title := s.title.Render("praxis") + s.muted.Render("  — learn while you build")
	theme := s.muted.Render("theme: " + Themes[m.themeIdx].Name)
	header := title + strings.Repeat(" ", max(1, width-lipgloss.Width(title)-lipgloss.Width(theme))) + theme

	var tabs []string
	for i, name := range sectionNames {
		if section(i) == m.focus {
			tabs = append(tabs, s.tabActive.Render(name))
		} else {
			tabs = append(tabs, s.tabIdle.Render(name))
		}
	}
	tabBar := strings.Join(tabs, " ")
	// Scroll position indicator, right-aligned on the tab line.
	if n := m.rowCount(); n > m.paneHeight() {
		pos := s.muted.Render(fmt.Sprintf("%d/%d", m.cursor+1, n))
		tabBar += strings.Repeat(" ", max(1, width-lipgloss.Width(tabBar)-lipgloss.Width(pos))) + pos
	}

	// One full-screen pane for the active tab. Width/Height exclude the
	// border (2 each); Padding(0,1) eats 2 more columns of content.
	pane := s.pane.Width(width - 2).Height(m.paneHeight()).
		Render(m.viewSection(width - 4))

	footer := s.muted.Render("tab/h/l switch · j/k move · g/G jump · " + m.footerKeys() + " · s sync · t theme · q quit")
	if m.status != "" {
		footer = s.warning.Render(m.status) + "  " + footer
	}

	body := lipgloss.JoinVertical(lipgloss.Left, header, tabBar, pane)
	// Pin the footer to the bottom row of the terminal.
	if pad := m.effHeight() - lipgloss.Height(body) - 1; pad > 0 {
		body += strings.Repeat("\n", pad)
	}
	return body + "\n" + footer
}

// Home returns the user's home dir, used by main to build adapters.
func Home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}
