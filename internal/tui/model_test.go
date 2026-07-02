package tui

import (
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
	t.Cleanup(func() { st.Close() })

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

func TestTabNavigationWraps(t *testing.T) {
	m := testModel(t)
	if m.tab != tabSkills {
		t.Fatalf("initial tab = %v", m.tab)
	}
	m = update(m, key("h"))
	if m.tab != tabHarnesses {
		t.Errorf("h from first tab should wrap to last, got %v", m.tab)
	}
	m = update(m, key("l"))
	if m.tab != tabSkills {
		t.Errorf("l should wrap back to first, got %v", m.tab)
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
	// Move to the Harnesses tab (last), cursor on first adapter (claude).
	m = update(m, key("h"))
	if m.tab != tabHarnesses {
		t.Fatalf("tab = %v", m.tab)
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
	for m.tab != tabAutonomy {
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
