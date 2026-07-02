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
}

func TestCopilotDoesNotListSkills(t *testing.T) {
	var a Adapter = NewCopilot(t.TempDir())
	if _, ok := a.(SkillLister); ok {
		t.Error("copilot should not implement SkillLister")
	}
	if _, ok := a.(ContextWriter); !ok {
		t.Error("copilot should implement ContextWriter")
	}
}
