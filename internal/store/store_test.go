package store

import (
	"path/filepath"
	"testing"

	"github.com/michael-duren/praxis/internal/domain"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "praxis.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSkillsCRUD(t *testing.T) {
	s := openTestStore(t)

	id, err := s.UpsertSkill(domain.UserSkill{Name: "go", Category: "language", Rank: domain.RankBeginner})
	if err != nil {
		t.Fatalf("UpsertSkill: %v", err)
	}
	if id == 0 {
		t.Fatal("UpsertSkill returned id 0")
	}

	// Upsert by name updates rank rather than duplicating.
	if _, err := s.UpsertSkill(domain.UserSkill{Name: "go", Category: "language", Rank: domain.RankAdvanced}); err != nil {
		t.Fatalf("UpsertSkill update: %v", err)
	}

	skills, err := s.Skills()
	if err != nil {
		t.Fatalf("Skills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Rank != domain.RankAdvanced {
		t.Errorf("rank = %v, want advanced", skills[0].Rank)
	}
	if skills[0].Updated.IsZero() {
		t.Error("Updated should be set")
	}

	if err := s.DeleteSkill("go"); err != nil {
		t.Fatalf("DeleteSkill: %v", err)
	}
	skills, _ = s.Skills()
	if len(skills) != 0 {
		t.Errorf("got %d skills after delete, want 0", len(skills))
	}
}

func TestContextEntriesCRUD(t *testing.T) {
	s := openTestStore(t)

	id, err := s.UpsertContextEntry(domain.ContextEntry{
		Title: "Style", Body: "Prefer table tests.",
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if _, err := s.UpsertContextEntry(domain.ContextEntry{
		ID: id, Scope: domain.Scope{Repo: "/home/x/repo"}, Title: "Style", Body: "Updated.",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	entries, err := s.ContextEntries()
	if err != nil {
		t.Fatalf("ContextEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Body != "Updated." {
		t.Errorf("body = %q, want %q", entries[0].Body, "Updated.")
	}
	if entries[0].Scope.IsGlobal() {
		t.Error("scope should be repo-local after update")
	}

	if err := s.DeleteContextEntry(id); err != nil {
		t.Fatalf("DeleteContextEntry: %v", err)
	}
	entries, _ = s.ContextEntries()
	if len(entries) != 0 {
		t.Errorf("got %d entries after delete, want 0", len(entries))
	}
}

func TestHarnessEnablement(t *testing.T) {
	s := openTestStore(t)

	if err := s.SetHarnessEnabled("claude", true); err != nil {
		t.Fatalf("SetHarnessEnabled: %v", err)
	}
	if err := s.SetHarnessEnabled("copilot", false); err != nil {
		t.Fatalf("SetHarnessEnabled: %v", err)
	}
	if err := s.SetHarnessEnabled("claude", true); err != nil {
		t.Fatalf("SetHarnessEnabled twice: %v", err)
	}

	got, err := s.EnabledHarnesses()
	if err != nil {
		t.Fatalf("EnabledHarnesses: %v", err)
	}
	if !got["claude"] || got["copilot"] {
		t.Errorf("EnabledHarnesses = %v, want claude on, copilot off", got)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	s := openTestStore(t)

	// Default before anything saved.
	set, err := s.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings default: %v", err)
	}
	if set.GlobalAutonomy != domain.AutonomyGuided {
		t.Errorf("default autonomy = %v, want guided", set.GlobalAutonomy)
	}

	if err := s.SaveSettings(domain.Settings{GlobalAutonomy: domain.AutonomyManual}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	set, err = s.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if set.GlobalAutonomy != domain.AutonomyManual {
		t.Errorf("autonomy = %v, want manual", set.GlobalAutonomy)
	}
}
