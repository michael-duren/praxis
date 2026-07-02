// Package orchestrator turns praxis state into context documents and
// pushes them to enabled harnesses. This is the "context orchestration"
// core: SQLite is the source of truth, the files in each harness are the
// derived, working state.
package orchestrator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/michael-duren/praxis/internal/domain"
	"github.com/michael-duren/praxis/internal/harness"
)

// State is everything needed to render context documents. Callers load
// it from the store; the orchestrator never touches the database itself.
type State struct {
	Skills   []domain.UserSkill
	Entries  []domain.ContextEntry
	Settings domain.Settings
}

// Result records one harness write.
type Result struct {
	Harness string
	Scope   domain.Scope
	Path    string
	Err     error
}

// Render produces the markdown context document for one scope: the
// autonomy header, per-skill policies, then the user's context entries
// for that scope (global entries are always included).
func Render(st State, scope domain.Scope) string {
	var b strings.Builder

	b.WriteString("<!-- Managed by praxis. Edit with `praxis`; manual changes will be overwritten. -->\n")
	b.WriteString("# Agent Context (praxis)\n\n")

	fmt.Fprintf(&b, "## Autonomy: %s\n\n", st.Settings.GlobalAutonomy)
	switch st.Settings.GlobalAutonomy {
	case domain.AutonomyManual:
		b.WriteString("The user does ALL file editing themselves. Never write or edit files; explain what to change and let the user type it.\n\n")
	case domain.AutonomyGuided:
		b.WriteString("Follow the per-skill rules below. When in doubt, explain before editing.\n\n")
	case domain.AutonomyFull:
		b.WriteString("You may work autonomously, but still follow the per-skill teaching rules below.\n\n")
	}

	if len(st.Skills) > 0 {
		b.WriteString("## Skill policies\n\n")
		b.WriteString("The user is actively learning. Calibrate to their rank per skill:\n\n")
		skills := append([]domain.UserSkill(nil), st.Skills...)
		sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
		for _, sk := range skills {
			p := domain.PolicyFor(sk, st.Settings.GlobalAutonomy)
			fmt.Fprintf(&b, "- %s\n", p.Description)
		}
		b.WriteString("\n")
	}

	var sections []domain.ContextEntry
	for _, e := range st.Entries {
		if e.Scope.IsGlobal() || e.Scope == scope {
			sections = append(sections, e)
		}
	}
	if len(sections) > 0 {
		b.WriteString("## Context\n\n")
		for _, e := range sections {
			fmt.Fprintf(&b, "### %s\n\n%s\n\n", e.Title, strings.TrimSpace(e.Body))
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// Sync renders and writes the document for scope to every enabled
// adapter that can write context. It returns one Result per adapter
// written (or attempted); a failing harness does not stop the others.
func Sync(st State, scope domain.Scope, adapters []harness.Adapter, enabled map[string]bool) []Result {
	doc := Render(st, scope)
	var results []Result
	for _, a := range adapters {
		if !enabled[a.Name()] {
			continue
		}
		w, ok := a.(harness.ContextWriter)
		if !ok {
			continue
		}
		path, err := w.WriteContext(scope, doc)
		results = append(results, Result{Harness: a.Name(), Scope: scope, Path: path, Err: err})
	}
	return results
}
