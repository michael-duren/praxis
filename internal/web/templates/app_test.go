package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/michael-duren/praxis/internal/domain"
)

func TestDashboardRendersAllSections(t *testing.T) {
	d := ViewData{
		Skills:      []domain.UserSkill{{Name: "go", Rank: domain.RankIntermediate}},
		Entries:     []domain.ContextEntry{{Title: "Style", Body: "Prefer stdlib."}},
		AgentSkills: []domain.AgentSkill{{Harness: "claude", Name: "deep-research"}},
		Harnesses:   []HarnessRow{{Name: "claude", Enabled: true, Detected: true}},
		Settings:    domain.Settings{GlobalAutonomy: domain.AutonomyGuided},
		Status:      "hello",
	}

	var b strings.Builder
	if err := Dashboard(d).Render(context.Background(), &b); err != nil {
		t.Fatalf("Render: %v", err)
	}
	html := b.String()

	for _, want := range []string{
		"praxis", "hello",
		"go", "intermediate",
		"Style", "Prefer stdlib.",
		"deep-research",
		"guided",
		"enabled", "detected",
		"Sync context to harnesses",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("dashboard html missing %q", want)
		}
	}
}
