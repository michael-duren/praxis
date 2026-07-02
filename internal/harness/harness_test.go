package harness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/michael-duren/praxis/internal/domain"
)

func TestAllAdaptersHaveUniqueNames(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range All(t.TempDir()) {
		if seen[a.Name()] {
			t.Errorf("duplicate adapter name %q", a.Name())
		}
		seen[a.Name()] = true
	}
	if len(seen) != 3 {
		t.Errorf("got %d adapters, want 3", len(seen))
	}
}

func TestDetect(t *testing.T) {
	home := t.TempDir()
	c := NewClaude(home)
	if c.Detect() {
		t.Error("Detect should be false with no ~/.claude")
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !c.Detect() {
		t.Error("Detect should be true once ~/.claude exists")
	}
}

func TestWriteContextPaths(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()

	tests := []struct {
		adapter  ContextWriter
		scope    domain.Scope
		wantPath string
	}{
		{NewClaude(home), domain.Scope{}, filepath.Join(home, ".claude", "CLAUDE.md")},
		{NewClaude(home), domain.Scope{Repo: repo}, filepath.Join(repo, "CLAUDE.md")},
		{NewOpenCode(home), domain.Scope{}, filepath.Join(home, ".config", "opencode", "AGENTS.md")},
		{NewOpenCode(home), domain.Scope{Repo: repo}, filepath.Join(repo, "AGENTS.md")},
		{NewCopilot(home), domain.Scope{}, filepath.Join(home, ".config", "praxis", "copilot-global.md")},
		{NewCopilot(home), domain.Scope{Repo: repo}, filepath.Join(repo, ".github", "copilot-instructions.md")},
	}

	for _, tt := range tests {
		got, err := tt.adapter.WriteContext(tt.scope, "# doc\n")
		if err != nil {
			t.Fatalf("WriteContext(%v): %v", tt.scope, err)
		}
		if got != tt.wantPath {
			t.Errorf("WriteContext(%v) path = %q, want %q", tt.scope, got, tt.wantPath)
		}
		data, err := os.ReadFile(got)
		if err != nil {
			t.Fatalf("read back %s: %v", got, err)
		}
		if string(data) != "# doc\n" {
			t.Errorf("content = %q", data)
		}
	}
}

func TestListSkills(t *testing.T) {
	home := t.TempDir()
	c := NewClaude(home)

	// Missing skills dir is not an error, just empty.
	skills, err := c.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills (missing dir): %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("got %d skills, want 0", len(skills))
	}

	dir := filepath.Join(home, ".claude", "skills", "deep-research")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A stray file should be ignored; only directories are skills.
	if err := os.WriteFile(filepath.Join(home, ".claude", "skills", "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err = c.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 1 || skills[0].Name != "deep-research" || skills[0].Harness != "claude" {
		t.Errorf("skills = %+v", skills)
	}
	if !skills[0].Enabled {
		t.Error("skill in the live dir should be Enabled")
	}
}

func TestSetSkillEnabled(t *testing.T) {
	home := t.TempDir()
	c := NewClaude(home)
	dir := filepath.Join(home, ".claude", "skills", "go-teacher")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Disable: moves to skills.disabled, listed as Enabled=false.
	if err := c.SetSkillEnabled("go-teacher", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("skill dir should be gone from the live skills dir")
	}
	moved := filepath.Join(home, ".claude", "skills.disabled", "go-teacher", "SKILL.md")
	if _, err := os.Stat(moved); err != nil {
		t.Errorf("skill content should survive the move: %v", err)
	}
	skills, err := c.ListSkills()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].Enabled {
		t.Fatalf("skills = %+v, want one disabled entry", skills)
	}

	// Disabling again is a no-op, not an error.
	if err := c.SetSkillEnabled("go-teacher", false); err != nil {
		t.Errorf("double disable: %v", err)
	}

	// Enable: moves it back.
	if err := c.SetSkillEnabled("go-teacher", true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	skills, _ = c.ListSkills()
	if len(skills) != 1 || !skills[0].Enabled {
		t.Fatalf("skills = %+v, want one enabled entry", skills)
	}

	// Unknown skills are an error.
	if err := c.SetSkillEnabled("nope", false); err == nil {
		t.Error("toggling an unknown skill should error")
	}
}

func TestListSkillsSortedAcrossStates(t *testing.T) {
	home := t.TempDir()
	c := NewClaude(home)
	for _, name := range []string{"bravo", "alpha", "charlie"} {
		if err := os.MkdirAll(filepath.Join(home, ".claude", "skills", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.SetSkillEnabled("bravo", false); err != nil {
		t.Fatal(err)
	}
	skills, err := c.ListSkills()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, sk := range skills {
		names = append(names, sk.Name)
	}
	want := []string{"alpha", "bravo", "charlie"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v (sorted despite bravo being disabled)", names, want)
		}
	}
}

func TestAdapterCapabilities(t *testing.T) {
	var copilot Adapter = NewCopilot(t.TempDir())
	if _, ok := copilot.(SkillLister); ok {
		t.Error("copilot should not implement SkillLister")
	}
	if _, ok := copilot.(SkillToggler); ok {
		t.Error("copilot should not implement SkillToggler")
	}
	if _, ok := copilot.(ContextWriter); !ok {
		t.Error("copilot should implement ContextWriter")
	}
	var claude Adapter = NewClaude(t.TempDir())
	if _, ok := claude.(SkillToggler); !ok {
		t.Error("claude should implement SkillToggler")
	}
	var oc Adapter = NewOpenCode(t.TempDir())
	if _, ok := oc.(SkillToggler); !ok {
		t.Error("opencode should implement SkillToggler")
	}
}
