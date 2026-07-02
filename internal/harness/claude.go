package harness

import (
	"path/filepath"

	"github.com/michael-duren/praxis/internal/domain"
)

// Claude adapts Claude Code. Global context lives in ~/.claude/CLAUDE.md,
// per-repo context in <repo>/CLAUDE.md, and skills under ~/.claude/skills.
type Claude struct {
	home string
}

func NewClaude(home string) *Claude { return &Claude{home: home} }

func (c *Claude) Name() string { return "claude" }

func (c *Claude) Detect() bool {
	return dirExists(filepath.Join(c.home, ".claude"))
}

func (c *Claude) WriteContext(scope domain.Scope, doc string) (string, error) {
	path := filepath.Join(c.home, ".claude", "CLAUDE.md")
	if !scope.IsGlobal() {
		path = filepath.Join(scope.Repo, "CLAUDE.md")
	}
	return writeFile(path, doc)
}

func (c *Claude) skillsRoot() string {
	return filepath.Join(c.home, ".claude", "skills")
}

func (c *Claude) ListSkills() ([]domain.AgentSkill, error) {
	return listSkills(c.Name(), c.skillsRoot())
}

func (c *Claude) SetSkillEnabled(name string, enabled bool) error {
	return setSkillEnabled(c.skillsRoot(), name, enabled)
}
