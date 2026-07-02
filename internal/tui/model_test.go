package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/michael-duren/praxis/internal/domain"
	"github.com/michael-duren/praxis/internal/harness"
	"github.com/michael-duren/praxis/internal/store"
)

func testModel(t *testing.T) Model {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "praxis.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if _, err := st.UpsertSkill(domain.UserSkill{Name: "go", Category: "language", Rank: domain.RankBeginner}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetHarnessEnabled("claude", true); err != nil {
		t.Fatal(err)
	}

	m, err := New(st, harness.All(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func key(s string) tea.KeyMsg {
	switch s {
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func update(m Model, msgs ...tea.Msg) Model {
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func TestFocusNavigationWraps(t *testing.T) {
	m := testModel(t)
	if m.focus != secSkills {
		t.Fatalf("initial focus = %v", m.focus)
	}
	m = update(m, key("h"))
	if m.focus != secHarnesses {
		t.Errorf("h from first section should wrap to last, got %v", m.focus)
	}
	m = update(m, key("l"))
	if m.focus != secSkills {
		t.Errorf("l should wrap back to first, got %v", m.focus)
	}
}

func TestViewShowsSkillsAndFooter(t *testing.T) {
	m := testModel(t)
	view := m.View()
	for _, want := range []string{"praxis", "go", "beginner", "q quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestTabBarListsAllSections(t *testing.T) {
	m := testModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)
	view := m.View()
	for _, want := range []string{"Skills", "Context", "Agent Skills", "Autonomy", "Harnesses"} {
		if !strings.Contains(view, want) {
			t.Errorf("tab bar missing %q", want)
		}
	}
	// Full-screen: the footer is pinned to the terminal's last row.
	if lines := strings.Count(view, "\n") + 1; lines != 40 {
		t.Errorf("view has %d lines, want exactly the terminal height 40", lines)
	}
}

func TestScrollingKeepsCursorVisible(t *testing.T) {
	m := testModel(t)
	for i := range 30 {
		if _, err := m.st.UpsertSkill(domain.UserSkill{
			Name: fmt.Sprintf("skill-%02d", i), Category: "test", Rank: domain.RankNovice,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.reload(); err != nil {
		t.Fatal(err)
	}

	// Height 12 → paneHeight 7 visible rows for 31 skills ("go" + 30).
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	m = next.(Model)
	for range 20 {
		m = update(m, key("j"))
	}
	if m.cursor != 20 {
		t.Fatalf("cursor = %d, want 20", m.cursor)
	}
	if want := 20 - 7 + 1; m.offset != want {
		t.Errorf("offset = %d, want %d (cursor on last visible row)", m.offset, want)
	}

	view := m.View()
	if !strings.Contains(view, "skill-19") { // skills[20] after sorted "go"
		t.Error("view should show the cursor row skill-19")
	}
	if strings.Contains(view, "skill-00") {
		t.Error("rows above the window should be scrolled out")
	}
	if !strings.Contains(view, "21/31") {
		t.Error("view should show the scroll position indicator 21/31")
	}

	// G jumps to the bottom, g back to the top.
	m = update(m, key("G"))
	if m.cursor != 30 || m.offset != 31-7 {
		t.Errorf("after G: cursor=%d offset=%d, want 30/%d", m.cursor, m.offset, 31-7)
	}
	m = update(m, key("g"))
	if m.cursor != 0 || m.offset != 0 {
		t.Errorf("after g: cursor=%d offset=%d, want 0/0", m.cursor, m.offset)
	}
}

func TestThemeCycling(t *testing.T) {
	m := testModel(t)
	m = update(m, key("t"))
	if m.themeIdx != 1 {
		t.Errorf("themeIdx = %d, want 1", m.themeIdx)
	}
	if !strings.Contains(m.status, Themes[1].Name) {
		t.Errorf("status = %q, want theme name", m.status)
	}
}

func TestRankAdjustPersists(t *testing.T) {
	m := testModel(t)
	m = update(m, key("+"))
	skills, err := m.st.Skills()
	if err != nil {
		t.Fatal(err)
	}
	if skills[0].Rank != domain.RankIntermediate {
		t.Errorf("rank = %v, want intermediate", skills[0].Rank)
	}

	// Can't go below novice.
	m = update(m, key("-"))
	m = update(m, key("-"))
	m = update(m, key("-"))
	skills, _ = m.st.Skills()
	if skills[0].Rank != domain.RankNovice {
		t.Errorf("rank = %v, want novice (floor)", skills[0].Rank)
	}
}

func TestHarnessToggle(t *testing.T) {
	m := testModel(t)
	// Move focus to Harnesses (last section), cursor on first adapter (claude).
	m = update(m, key("h"))
	if m.focus != secHarnesses {
		t.Fatalf("focus = %v", m.focus)
	}
	m = update(m, key("space"))
	enabled, err := m.st.EnabledHarnesses()
	if err != nil {
		t.Fatal(err)
	}
	if enabled["claude"] {
		t.Error("claude should be disabled after toggle")
	}
}

func TestContextAddEditDelete(t *testing.T) {
	m := testModel(t)
	m = update(m, key("l")) // focus Context

	// Add: a → title, body, empty repo.
	m = update(m, key("a"))
	if !m.editing {
		t.Fatal("a should open the entry form")
	}
	view := m.View()
	if !strings.Contains(view, "new context entry") {
		t.Errorf("edit form not rendered:\n%s", view)
	}
	m = update(m, key("Style"))
	m = update(m, key("enter"))
	m = update(m, key("Prefer stdlib."))
	m = update(m, key("enter"))
	m = update(m, key("enter")) // empty repo = global
	if m.editing {
		t.Fatal("form should close after the last field")
	}
	entries, _ := m.st.ContextEntries()
	if len(entries) != 1 || entries[0].Title != "Style" || !entries[0].Scope.IsGlobal() {
		t.Fatalf("entries = %+v", entries)
	}

	// Empty title is rejected, esc cancels without saving.
	m = update(m, key("a"))
	m = update(m, key("enter"))
	if !m.editing {
		t.Fatal("empty title should keep the form open")
	}
	m = update(m, key("esc"))
	entries, _ = m.st.ContextEntries()
	if len(entries) != 1 {
		t.Fatalf("esc should not save, entries = %+v", entries)
	}

	// Edit: prefilled title gets appended text.
	m = update(m, key("e"))
	m = update(m, key(" v2"))
	m = update(m, key("enter"), key("enter"), key("enter"))
	entries, _ = m.st.ContextEntries()
	if entries[0].Title != "Style v2" {
		t.Errorf("title = %q, want %q", entries[0].Title, "Style v2")
	}
	if entries[0].Body != "Prefer stdlib." {
		t.Errorf("body should be preserved through edit, got %q", entries[0].Body)
	}

	// Delete needs a confirming second d.
	m = update(m, key("d"))
	if entries, _ := m.st.ContextEntries(); len(entries) != 1 {
		t.Fatal("first d should not delete")
	}
	if !strings.Contains(m.status, "press d again") {
		t.Errorf("status = %q", m.status)
	}
	m = update(m, key("d"))
	if entries, _ := m.st.ContextEntries(); len(entries) != 0 {
		t.Fatal("second d should delete")
	}

	// A j/k press between the two d's resets the confirmation.
	m = update(m, key("a"), key("X"), key("enter"), key("Y"), key("enter"), key("enter"))
	m = update(m, key("d"), key("k"), key("d"))
	if entries, _ := m.st.ContextEntries(); len(entries) != 1 {
		t.Error("interrupted double-d should not delete")
	}
}

func TestContextDetailView(t *testing.T) {
	m := testModel(t)
	for _, e := range []domain.ContextEntry{
		{Title: "First", Body: "Body of the first entry."},
		{Title: "Second", Body: "Body of the second entry."},
	} {
		if _, err := m.st.UpsertContextEntry(e); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.reload(); err != nil {
		t.Fatal(err)
	}

	m = update(m, key("l")) // focus Context
	view := m.View()
	if strings.Contains(view, "Body of the first entry.") {
		t.Fatal("list view should not show bodies")
	}

	m = update(m, key("enter"))
	if !m.viewing {
		t.Fatal("enter should open the detail view")
	}
	view = m.View()
	if !strings.Contains(view, "Body of the first entry.") {
		t.Errorf("detail view missing body:\n%s", view)
	}
	if !strings.Contains(view, "1/2") {
		t.Error("detail view missing position 1/2")
	}

	// j flips to the next entry without leaving the detail view.
	m = update(m, key("j"))
	if !m.viewing {
		t.Fatal("j should keep the detail view open")
	}
	if !strings.Contains(m.View(), "Body of the second entry.") {
		t.Error("j should show the next entry's body")
	}

	// q closes the view without quitting the app.
	m = update(m, key("q"))
	if m.viewing {
		t.Error("q should close the detail view")
	}

	// e from the detail view opens the editor prefilled.
	m = update(m, key("enter"), key("e"))
	if m.viewing || !m.editing {
		t.Fatalf("e should switch from viewing to editing (viewing=%v editing=%v)", m.viewing, m.editing)
	}
	if m.editBuf != "Second" {
		t.Errorf("editBuf = %q, want prefilled %q", m.editBuf, "Second")
	}
}

func TestAgentSkillToggle(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "praxis.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	home := t.TempDir()
	skillDir := filepath.Join(home, ".claude", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	m, err := New(st, harness.All(home))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.agentSkills) != 1 || !m.agentSkills[0].Enabled {
		t.Fatalf("agentSkills = %+v, want one enabled", m.agentSkills)
	}

	m = update(m, key("l")) // Context
	m = update(m, key("l")) // Agent Skills
	if m.focus != secAgentSkills {
		t.Fatalf("focus = %v", m.focus)
	}
	m = update(m, key("space"))

	if len(m.agentSkills) != 1 || m.agentSkills[0].Enabled {
		t.Errorf("agentSkills = %+v, want one disabled after toggle", m.agentSkills)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Error("skill dir should have moved to skills.disabled")
	}
	if !strings.Contains(m.status, "demo disabled") {
		t.Errorf("status = %q", m.status)
	}

	// Toggle back on.
	m = update(m, key("space"))
	if !m.agentSkills[0].Enabled {
		t.Error("skill should be enabled again")
	}
	if _, err := os.Stat(skillDir); err != nil {
		t.Errorf("skill dir should be back: %v", err)
	}
}

func TestAutonomyCycle(t *testing.T) {
	m := testModel(t)
	for m.focus != secAutonomy {
		m = update(m, key("l"))
	}
	m = update(m, key("space"))
	set, err := m.st.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if set.GlobalAutonomy != domain.AutonomyFull {
		t.Errorf("autonomy = %v, want full (guided → full)", set.GlobalAutonomy)
	}
}
