package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/michael-duren/praxis/internal/domain"
	"github.com/michael-duren/praxis/internal/harness"
	"github.com/michael-duren/praxis/internal/store"
)

func testModel(t *testing.T) (Model, string) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "praxis.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	home := t.TempDir()
	return New(st, harness.All(home)), home
}

// press feeds keys to the model. Multi-rune strings are sent as one
// KeyRunes message (like pasting); named keys are translated.
func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func TestFullFlowSavesEverything(t *testing.T) {
	m, home := testModel(t)

	m = press(t, m, "space", "enter")  // languages: select "go"
	m = press(t, m, "enter")           // computer science: none
	m = press(t, m, "enter")           // system design: none
	m = press(t, m, "enter")           // technologies: none
	m = press(t, m, "l", "l", "enter") // rank: go novice → intermediate
	m = press(t, m, "j", "enter")      // autonomy: guided
	m = press(t, m, "space", "enter")  // harnesses: enable claude
	m = press(t, m, "enter")           // sync now (first option)

	if !m.done {
		t.Fatal("wizard should be done after the last step")
	}
	paths, err := m.Save()
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	skills, _ := m.st.Skills()
	if len(skills) != 1 || skills[0].Name != "go" {
		t.Fatalf("skills = %+v, want just go", skills)
	}
	if skills[0].Rank != domain.RankIntermediate {
		t.Errorf("rank = %v, want intermediate", skills[0].Rank)
	}
	if skills[0].Category != "language" {
		t.Errorf("category = %q, want language", skills[0].Category)
	}

	set, _ := m.st.LoadSettings()
	if set.GlobalAutonomy != domain.AutonomyGuided {
		t.Errorf("autonomy = %v, want guided", set.GlobalAutonomy)
	}

	enabled, _ := m.st.EnabledHarnesses()
	if !enabled["claude"] || enabled["opencode"] || enabled["copilot"] {
		t.Errorf("enabled = %v, want only claude", enabled)
	}

	want := filepath.Join(home, ".claude", "CLAUDE.md")
	if len(paths) != 1 || paths[0] != want {
		t.Fatalf("paths = %v, want [%s]", paths, want)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("synced file: %v", err)
	}
	if !strings.Contains(string(data), `Skill "go"`) {
		t.Errorf("synced doc missing skill policy:\n%s", data)
	}
}

func TestAddYourOwnSkill(t *testing.T) {
	m, _ := testModel(t)

	m = press(t, m, "a", "zig", "enter") // custom language, auto-selected
	s := m.steps[0]
	last := len(s.options) - 1
	if s.options[last] != "zig" || !s.selected[last] {
		t.Fatalf("custom option not added+selected: %v %v", s.options, s.selected)
	}

	// Duplicates are not added twice.
	m = press(t, m, "a", "zig", "enter")
	if got := len(m.steps[0].options); got != last+1 {
		t.Errorf("duplicate was appended: %d options", got)
	}

	m = press(t, m, "enter", "enter", "enter", "enter") // through categories to rank
	m = press(t, m, "enter")                            // rank: keep novice
	m = press(t, m, "enter")                            // autonomy: manual (cursor 0)
	m = press(t, m, "enter")                            // harnesses: none
	m = press(t, m, "j", "enter")                       // sync later

	paths, err := m.Save()
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("sync-later should write nothing, wrote %v", paths)
	}
	skills, _ := m.st.Skills()
	if len(skills) != 1 || skills[0].Name != "zig" || skills[0].Rank != domain.RankNovice {
		t.Errorf("skills = %+v, want zig at novice", skills)
	}
	set, _ := m.st.LoadSettings()
	if set.GlobalAutonomy != domain.AutonomyManual {
		t.Errorf("autonomy = %v, want manual", set.GlobalAutonomy)
	}
}

func TestAbortSavesNothing(t *testing.T) {
	m, _ := testModel(t)
	m = press(t, m, "space", "q")
	if !m.aborted {
		t.Fatal("q should abort")
	}
	paths, err := m.Save()
	if err != nil || paths != nil {
		t.Errorf("aborted Save = (%v, %v), want (nil, nil)", paths, err)
	}
	skills, _ := m.st.Skills()
	if len(skills) != 0 {
		t.Errorf("aborted setup saved skills: %+v", skills)
	}
}

func TestRankStepSkippedWithNoSkills(t *testing.T) {
	m, _ := testModel(t)
	m = press(t, m, "enter", "enter", "enter", "enter")
	if m.idx != m.autonomyStep {
		t.Errorf("idx = %d, want autonomy step %d (rank skipped)", m.idx, m.autonomyStep)
	}
	// esc goes back past the empty rank step too.
	m = press(t, m, "esc")
	if m.idx != m.rankStep-1 {
		t.Errorf("idx after esc = %d, want %d", m.idx, m.rankStep-1)
	}
}

func TestDetectedHarnessPreselected(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "praxis.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	m := New(st, harness.All(home))
	hs := m.steps[m.harnessStep]
	if !hs.selected[0] {
		t.Error("detected claude should start selected")
	}
	if !strings.Contains(hs.options[0], "(detected)") {
		t.Errorf("label = %q, want (detected) marker", hs.options[0])
	}
	if hs.selected[2] {
		t.Error("undetected copilot should not start selected")
	}
}

func TestViewShowsQuestionAndOptions(t *testing.T) {
	m, _ := testModel(t)
	view := m.View()
	for _, want := range []string{"praxis setup", "step 1 of 8", "programming languages", "[ ] go", "q quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
	m = press(t, m, "a", "zi")
	if !strings.Contains(m.View(), "add your own: zi") {
		t.Error("adding view missing input line")
	}
}
