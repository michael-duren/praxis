package tui

import (
	"fmt"
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
	if s == "space" {
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func update(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
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
