// Package setup is the first-run wizard: a deliberately simple TUI that
// asks one question per screen and saves everything at the end. Nothing
// is written until the final step completes, so quitting midway is safe.
package setup

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/michael-duren/praxis/internal/domain"
	"github.com/michael-duren/praxis/internal/harness"
	"github.com/michael-duren/praxis/internal/orchestrator"
	"github.com/michael-duren/praxis/internal/store"
)

type kind int

const (
	kindMulti  kind = iota // checkbox list, space toggles
	kindSingle             // radio list, enter chooses
	kindRank               // per-skill rank sliders
)

// step is one question screen.
type step struct {
	kind     kind
	title    string
	hint     string
	category string // skill category saved with selections ("" = not a skill step)
	allowAdd bool   // 'a' opens the add-your-own input
	options  []string
	selected map[int]bool
}

// skillChoice is a skill the user picked, carrying the rank they assign.
type skillChoice struct {
	name     string
	category string
	rank     domain.Rank
}

// Model is the Bubble Tea model for the setup wizard.
type Model struct {
	st       *store.Store
	adapters []harness.Adapter

	steps  []step
	idx    int
	cursor int

	// indices of the special steps, resolved once in New.
	rankStep     int
	autonomyStep int
	harnessStep  int
	syncStep     int

	adding bool // typing a custom option
	input  string

	skills  []skillChoice
	aborted bool
	done    bool
}

func skillStep(topic, category string, options []string) step {
	return step{
		kind:     kindMulti,
		category: category,
		allowAdd: true,
		title:    fmt.Sprintf("Which %s are you working on?", topic),
		hint:     "space select · a add your own · enter continue",
		options:  options,
		selected: map[int]bool{},
	}
}

// New builds the wizard. Detected harnesses start preselected.
func New(st *store.Store, adapters []harness.Adapter) Model {
	harnesses := step{
		kind:     kindMulti,
		title:    "Which harnesses should praxis write to?",
		hint:     "space select · enter continue",
		selected: map[int]bool{},
	}
	for i, a := range adapters {
		label := a.Name()
		if a.Detect() {
			label += "  (detected)"
			harnesses.selected[i] = true
		}
		harnesses.options = append(harnesses.options, label)
	}

	steps := []step{
		skillStep("programming languages", "language",
			[]string{"go", "typescript", "python", "rust", "c", "bash", "sql"}),
		skillStep("computer science topics", "computer-science",
			[]string{"algorithms", "data-structures", "networking", "operating-systems", "databases", "concurrency"}),
		skillStep("system design topics", "system-design",
			[]string{"microservices", "distributed-systems", "caching", "message-queues", "api-design", "observability"}),
		skillStep("technologies & tools", "technology",
			[]string{"docker", "kubernetes", "git", "linux", "ci-cd", "neovim"}),
		{kind: kindRank,
			title: "How well do you know each skill?",
			hint:  "j/k move · h/l adjust rank · enter continue"},
		{kind: kindSingle,
			title: "How much should agents do on their own?",
			hint:  "j/k move · enter choose",
			options: []string{
				"manual — agents never write files; you type everything",
				"guided — autonomy scales with your rank per skill",
				"full — agents edit freely but still teach",
			},
			selected: map[int]bool{}},
		harnesses,
		{kind: kindSingle,
			title:    "Write context files to your harnesses now?",
			hint:     "j/k move · enter choose",
			options:  []string{"yes — sync now", "no — I'll run `praxis sync` later"},
			selected: map[int]bool{}},
	}

	return Model{
		st: st, adapters: adapters, steps: steps,
		rankStep: 4, autonomyStep: 5, harnessStep: 6, syncStep: 7,
	}
}

// Run starts the wizard, then saves and reports if it completed.
func Run(st *store.Store, adapters []harness.Adapter) error {
	final, err := tea.NewProgram(New(st, adapters)).Run()
	if err != nil {
		return err
	}
	m, ok := final.(Model)
	if !ok || m.aborted {
		fmt.Println("setup aborted — nothing was saved")
		return nil
	}
	paths, err := m.Save()
	if err != nil {
		return err
	}
	fmt.Printf("setup complete: %d skill(s) saved\n", len(m.skills))
	for _, p := range paths {
		fmt.Println("wrote", p)
	}
	if len(paths) == 0 {
		fmt.Println("run `praxis sync` when you want to write context files")
	}
	return nil
}

func (m Model) Init() tea.Cmd { return nil }

// rows is how many selectable rows the current step shows.
func (m Model) rows() int {
	if m.steps[m.idx].kind == kindRank {
		return len(m.skills)
	}
	return len(m.steps[m.idx].options)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.adding {
		return m.updateAdding(key), nil
	}

	s := &m.steps[m.idx]
	switch key.String() {
	case "ctrl+c", "q":
		m.aborted = true
		return m, tea.Quit
	case "j", "down":
		if m.cursor < m.rows()-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case " ":
		if s.kind == kindMulti && m.cursor < len(s.options) {
			s.selected[m.cursor] = !s.selected[m.cursor]
		}
	case "a":
		if s.allowAdd {
			m.adding = true
		}
	case "h", "left":
		m.adjustRank(-1)
	case "l", "right":
		m.adjustRank(+1)
	case "esc":
		m.back()
	case "enter":
		if s.kind == kindSingle {
			s.selected = map[int]bool{m.cursor: true}
		}
		if m.idx == len(m.steps)-1 {
			m.done = true
			return m, tea.Quit
		}
		m.advance()
	}
	return m, nil
}

// updateAdding handles keystrokes while typing a custom option.
func (m Model) updateAdding(key tea.KeyMsg) Model {
	s := &m.steps[m.idx]
	switch key.Type {
	case tea.KeyEnter:
		name := strings.ToLower(strings.TrimSpace(m.input))
		if name != "" && !slices.Contains(s.options, name) {
			s.options = append(s.options, name)
			s.selected[len(s.options)-1] = true
		}
		m.adding, m.input = false, ""
	case tea.KeyEsc:
		m.adding, m.input = false, ""
	case tea.KeyBackspace:
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		m.input += " "
	case tea.KeyRunes:
		m.input += string(key.Runes)
	}
	return m
}

func (m *Model) adjustRank(delta int) {
	if m.steps[m.idx].kind != kindRank || m.cursor >= len(m.skills) {
		return
	}
	r := m.skills[m.cursor].rank + domain.Rank(delta)
	if r >= domain.RankNovice && r <= domain.RankExpert {
		m.skills[m.cursor].rank = r
	}
}

// advance moves to the next step, rebuilding the skill list (and skipping
// the rank step entirely) when arriving there.
func (m *Model) advance() {
	m.idx++
	m.cursor = 0
	if m.idx == m.rankStep {
		m.buildSkills()
		if len(m.skills) == 0 {
			m.idx++
		}
	}
}

// back returns to the previous step, skipping an empty rank step.
func (m *Model) back() {
	if m.idx == 0 {
		return
	}
	m.idx--
	if m.idx == m.rankStep && len(m.skills) == 0 {
		m.idx--
	}
	m.cursor = 0
}

// buildSkills collects every selected skill option, preserving ranks the
// user already assigned if they navigated back and forth.
func (m *Model) buildSkills() {
	prev := map[string]domain.Rank{}
	for _, sc := range m.skills {
		prev[sc.name] = sc.rank
	}
	m.skills = nil
	for _, s := range m.steps {
		if s.category == "" {
			continue
		}
		for i, opt := range s.options {
			if s.selected[i] {
				m.skills = append(m.skills, skillChoice{name: opt, category: s.category, rank: prev[opt]})
			}
		}
	}
}

// singleChoice returns the selected index of a single-select step,
// falling back to def when nothing was chosen.
func (m Model) singleChoice(stepIdx, def int) int {
	for i, on := range m.steps[stepIdx].selected {
		if on {
			return i
		}
	}
	return def
}

// Save persists everything the wizard collected and, if the user opted
// in, syncs context files. It returns the paths written. Safe to call on
// an aborted model: it does nothing.
func (m Model) Save() ([]string, error) {
	if m.aborted || !m.done {
		return nil, nil
	}

	for _, sc := range m.skills {
		if _, err := m.st.UpsertSkill(domain.UserSkill{Name: sc.name, Category: sc.category, Rank: sc.rank}); err != nil {
			return nil, err
		}
	}

	mode := domain.AutonomyMode(m.singleChoice(m.autonomyStep, int(domain.AutonomyGuided)))
	settings := domain.Settings{GlobalAutonomy: mode}
	if err := m.st.SaveSettings(settings); err != nil {
		return nil, err
	}

	enabled := map[string]bool{}
	for i, a := range m.adapters {
		on := m.steps[m.harnessStep].selected[i]
		if err := m.st.SetHarnessEnabled(a.Name(), on); err != nil {
			return nil, err
		}
		enabled[a.Name()] = on
	}

	if m.singleChoice(m.syncStep, 1) != 0 { // 0 = "yes — sync now"
		return nil, nil
	}
	skills, err := m.st.Skills()
	if err != nil {
		return nil, err
	}
	entries, err := m.st.ContextEntries()
	if err != nil {
		return nil, err
	}
	state := orchestrator.State{Skills: skills, Entries: entries, Settings: settings}
	var paths []string
	for _, r := range orchestrator.Sync(state, domain.Scope{}, m.adapters, enabled) {
		if r.Err != nil {
			return paths, r.Err
		}
		paths = append(paths, r.Path)
	}
	return paths, nil
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	hintStyle  = lipgloss.NewStyle().Faint(true)
	activeRow  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
)

func (m Model) View() string {
	if m.done || m.aborted {
		return ""
	}
	s := m.steps[m.idx]
	var b strings.Builder

	b.WriteString(titleStyle.Render("praxis setup"))
	b.WriteString(hintStyle.Render(fmt.Sprintf("  step %d of %d", m.idx+1, len(m.steps))))
	b.WriteString("\n\n")
	b.WriteString(s.title)
	b.WriteString("\n")
	b.WriteString(hintStyle.Render(s.hint + " · esc back · q quit"))
	b.WriteString("\n\n")

	if m.adding {
		b.WriteString("add your own: ")
		b.WriteString(m.input)
		b.WriteString("▌\n")
		b.WriteString(hintStyle.Render("enter to add · esc to cancel"))
		b.WriteString("\n")
		return b.String()
	}

	writeRow := func(i int, text string) {
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			prefix = "> "
			style = activeRow
		}
		b.WriteString(style.Render(prefix + text))
		b.WriteString("\n")
	}

	switch s.kind {
	case kindRank:
		for i, sc := range m.skills {
			bar := strings.Repeat("█", int(sc.rank)+1) + strings.Repeat("░", 4-int(sc.rank))
			writeRow(i, fmt.Sprintf("%-20s %s %s", sc.name, bar, sc.rank))
		}
	case kindMulti:
		for i, opt := range s.options {
			mark := "[ ]"
			if s.selected[i] {
				mark = "[x]"
			}
			writeRow(i, mark+" "+opt)
		}
	case kindSingle:
		for i, opt := range s.options {
			mark := "( )"
			if s.selected[i] {
				mark = "(•)"
			}
			writeRow(i, mark+" "+opt)
		}
	}
	return b.String()
}
