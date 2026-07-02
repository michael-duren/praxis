package orchestrator

import (
	"errors"
	"strings"
	"testing"

	"github.com/michael-duren/praxis/internal/domain"
	"github.com/michael-duren/praxis/internal/harness"
)

func testState() State {
	return State{
		Skills: []domain.UserSkill{
			{Name: "k8s", Rank: domain.RankNovice},
			{Name: "go", Rank: domain.RankAdvanced},
		},
		Entries: []domain.ContextEntry{
			{Title: "Global rule", Body: "Prefer stdlib."},
			{Scope: domain.Scope{Repo: "/repo/a"}, Title: "Repo rule", Body: "Use templ."},
		},
		Settings: domain.Settings{GlobalAutonomy: domain.AutonomyGuided},
	}
}

func TestRenderGlobalScope(t *testing.T) {
	doc := Render(testState(), domain.Scope{})

	for _, want := range []string{
		"Managed by praxis",
		"## Autonomy: guided",
		`Skill "go" (rank: advanced)`,
		`Skill "k8s" (rank: novice)`,
		"### Global rule",
		"Prefer stdlib.",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("doc missing %q\n---\n%s", want, doc)
		}
	}
	if strings.Contains(doc, "Repo rule") {
		t.Error("global doc should not include repo-scoped entries")
	}
	// Skills should be sorted by name.
	if strings.Index(doc, `"go"`) > strings.Index(doc, `"k8s"`) {
		t.Error("skills not sorted by name")
	}
}

func TestRenderRepoScopeIncludesGlobalEntries(t *testing.T) {
	doc := Render(testState(), domain.Scope{Repo: "/repo/a"})
	if !strings.Contains(doc, "Global rule") {
		t.Error("repo doc should include global entries")
	}
	if !strings.Contains(doc, "Repo rule") {
		t.Error("repo doc should include its own entries")
	}
}

func TestRenderManualModeHeader(t *testing.T) {
	st := testState()
	st.Settings.GlobalAutonomy = domain.AutonomyManual
	doc := Render(st, domain.Scope{})
	if !strings.Contains(doc, "Never write or edit files") {
		t.Errorf("manual doc missing hard no-edit rule\n---\n%s", doc)
	}
}

// fakeAdapter implements Adapter + ContextWriter for Sync tests.
type fakeAdapter struct {
	name  string
	wrote string
	err   error
}

func (f *fakeAdapter) Name() string { return f.name }
func (f *fakeAdapter) Detect() bool { return true }
func (f *fakeAdapter) WriteContext(scope domain.Scope, doc string) (string, error) {
	f.wrote = doc
	return "/fake/" + f.name, f.err
}

// bareAdapter implements only Adapter — Sync must skip it.
type bareAdapter struct{}

func (bareAdapter) Name() string { return "bare" }
func (bareAdapter) Detect() bool { return true }

func TestSync(t *testing.T) {
	ok := &fakeAdapter{name: "claude"}
	failing := &fakeAdapter{name: "opencode", err: errors.New("disk full")}
	disabled := &fakeAdapter{name: "copilot"}

	adapters := []harness.Adapter{ok, failing, disabled, bareAdapter{}}
	enabled := map[string]bool{"claude": true, "opencode": true, "bare": true}

	results := Sync(testState(), domain.Scope{}, adapters, enabled)

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (disabled and non-writer skipped): %+v", len(results), results)
	}
	if results[0].Harness != "claude" || results[0].Err != nil {
		t.Errorf("claude result = %+v", results[0])
	}
	if results[1].Harness != "opencode" || results[1].Err == nil {
		t.Errorf("opencode should carry its error: %+v", results[1])
	}
	if disabled.wrote != "" {
		t.Error("disabled adapter should not be written")
	}
	if !strings.Contains(ok.wrote, "Agent Context (praxis)") {
		t.Error("written doc should be the rendered document")
	}
}
