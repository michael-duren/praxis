package harness

import (
	"path/filepath"

	"github.com/michael-duren/praxis/internal/domain"
)

// OpenCode adapts the OpenCode harness. Global context lives in
// ~/.config/opencode/AGENTS.md, per-repo context in <repo>/AGENTS.md.
type OpenCode struct {
	home string
}

func NewOpenCode(home string) *OpenCode { return &OpenCode{home: home} }

func (o *OpenCode) Name() string { return "opencode" }

func (o *OpenCode) Detect() bool {
	return dirExists(filepath.Join(o.home, ".config", "opencode"))
}

func (o *OpenCode) WriteContext(scope domain.Scope, doc string) (string, error) {
	path := filepath.Join(o.home, ".config", "opencode", "AGENTS.md")
	if !scope.IsGlobal() {
		path = filepath.Join(scope.Repo, "AGENTS.md")
	}
	return writeFile(path, doc)
}

func (o *OpenCode) ListSkills() ([]domain.AgentSkill, error) {
	return listSkillDirs(o.Name(), filepath.Join(o.home, ".config", "opencode", "skills"))
}
