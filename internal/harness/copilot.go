package harness

import (
	"path/filepath"

	"github.com/michael-duren/praxis/internal/domain"
)

// Copilot adapts GitHub Copilot. Copilot only reads per-repo custom
// instructions (.github/copilot-instructions.md); global scope writes to
// a praxis-managed file the user can symlink or copy, and it has no
// skills directory, so it does not implement SkillLister.
type Copilot struct {
	home string
}

func NewCopilot(home string) *Copilot { return &Copilot{home: home} }

func (c *Copilot) Name() string { return "copilot" }

func (c *Copilot) Detect() bool {
	return dirExists(filepath.Join(c.home, ".config", "github-copilot"))
}

func (c *Copilot) WriteContext(scope domain.Scope, doc string) (string, error) {
	if scope.IsGlobal() {
		// Copilot has no global instructions file; keep a copy in the
		// praxis config dir for the user to reference.
		return writeFile(filepath.Join(c.home, ".config", "praxis", "copilot-global.md"), doc)
	}
	return writeFile(filepath.Join(scope.Repo, ".github", "copilot-instructions.md"), doc)
}
