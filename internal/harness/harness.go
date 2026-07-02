// Package harness defines the small interfaces a coding-agent harness
// (Claude Code, OpenCode, Copilot, ...) can implement, plus the built-in
// adapters. Each capability is its own interface so an adapter implements
// only what its harness supports, and new harnesses slot in by
// implementing Adapter plus whichever capabilities apply.
package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/michael-duren/praxis/internal/domain"
)

// Adapter identifies a harness. Every harness implements this.
type Adapter interface {
	// Name is the stable identifier stored in the database ("claude").
	Name() string
	// Detect reports whether the harness appears to be installed/used
	// on this machine (e.g. its config directory exists).
	Detect() bool
}

// ContextWriter writes a rendered context document into the harness's
// expected location for the given scope, returning the path written.
type ContextWriter interface {
	WriteContext(scope domain.Scope, doc string) (string, error)
}

// SkillLister reports agent skills installed for the harness.
type SkillLister interface {
	ListSkills() ([]domain.AgentSkill, error)
}

// SkillToggler enables or disables an installed agent skill. Disabling
// moves the skill directory out of the harness's search path (into a
// sibling skills.disabled directory), so the agent genuinely stops
// loading it; enabling moves it back.
type SkillToggler interface {
	SetSkillEnabled(name string, enabled bool) error
}

// All returns the built-in adapters. home is the user's home directory,
// injected so tests can point adapters at a temp dir.
func All(home string) []Adapter {
	return []Adapter{
		NewClaude(home),
		NewOpenCode(home),
		NewCopilot(home),
	}
}

// writeFile creates parent directories and writes doc atomically enough
// for config files (write then rename would be overkill here).
func writeFile(path, doc string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// dirExists is the common Detect implementation.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// disabledDir is where a harness's disabled skills are parked, as a
// sibling of its skills directory.
func disabledDir(root string) string { return root + ".disabled" }

// listSkillDirs treats each subdirectory of root as one installed skill.
func listSkillDirs(harnessName, root string, enabled bool) ([]domain.AgentSkill, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []domain.AgentSkill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		out = append(out, domain.AgentSkill{
			Harness: harnessName,
			Name:    e.Name(),
			Path:    filepath.Join(root, e.Name()),
			Enabled: enabled,
		})
	}
	return out, nil
}

// listSkills combines the enabled and disabled skill dirs under root,
// sorted by name so toggling doesn't reorder the list.
func listSkills(harnessName, root string) ([]domain.AgentSkill, error) {
	on, err := listSkillDirs(harnessName, root, true)
	if err != nil {
		return nil, err
	}
	off, err := listSkillDirs(harnessName, disabledDir(root), false)
	if err != nil {
		return nil, err
	}
	all := append(on, off...)
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return all, nil
}

// setSkillEnabled moves a skill between root and its disabled sibling.
// Already being in the desired state is not an error.
func setSkillEnabled(root, name string, enabled bool) error {
	from := filepath.Join(disabledDir(root), name)
	to := filepath.Join(root, name)
	if !enabled {
		from, to = to, from
	}
	if dirExists(to) {
		return nil // already in the desired state
	}
	if !dirExists(from) {
		return fmt.Errorf("skill %q not found", name)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.Rename(from, to)
}
