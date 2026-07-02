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

func (o *OpenCode) skillsRoot() string {
	return filepath.Join(o.home, ".config", "opencode", "skills")
}

func (o *OpenCode) ListSkills() ([]domain.AgentSkill, error) {
	return listSkills(o.Name(), o.skillsRoot())
}

func (o *OpenCode) SetSkillEnabled(name string, enabled bool) error {
	return setSkillEnabled(o.skillsRoot(), name, enabled)
}
