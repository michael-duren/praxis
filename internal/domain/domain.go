// Package domain holds praxis's core types. Everything else (store,
// harness adapters, UIs) depends on this package; it depends on nothing.
package domain

import (
	"fmt"
	"strings"
	"time"
)

// Rank is how well the user knows a skill. It drives how much autonomy
// agents are given when working in that skill's territory.
type Rank int

const (
	RankNovice Rank = iota
	RankBeginner
	RankIntermediate
	RankAdvanced
	RankExpert
)

var rankNames = [...]string{"novice", "beginner", "intermediate", "advanced", "expert"}

func (r Rank) String() string {
	if r < RankNovice || r > RankExpert {
		return "unknown"
	}
	return rankNames[r]
}

// ParseRank converts a stored string back into a Rank.
func ParseRank(s string) (Rank, error) {
	for i, name := range rankNames {
		if name == s {
			return Rank(i), nil
		}
	}
	return RankNovice, fmt.Errorf("unknown rank %q", s)
}

// AutonomyMode is the global switch for how much agents may do on their own.
type AutonomyMode int

const (
	// AutonomyManual: agents never write files; they explain and the user types.
	AutonomyManual AutonomyMode = iota
	// AutonomyGuided: agents may edit, but must explain first and quiz on weak skills.
	AutonomyGuided
	// AutonomyFull: agents work autonomously; praxis still injects learning context.
	AutonomyFull
)

var autonomyNames = [...]string{"manual", "guided", "full"}

func (a AutonomyMode) String() string {
	if a < AutonomyManual || a > AutonomyFull {
		return "unknown"
	}
	return autonomyNames[a]
}

// ParseAutonomyMode converts a stored string back into an AutonomyMode.
func ParseAutonomyMode(s string) (AutonomyMode, error) {
	for i, name := range autonomyNames {
		if name == s {
			return AutonomyMode(i), nil
		}
	}
	return AutonomyManual, fmt.Errorf("unknown autonomy mode %q", s)
}

// UserSkill is a topic the user is learning, with a rank that agents
// use to calibrate explanation depth and editing permission.
type UserSkill struct {
	ID       int64
	Name     string // "go", "k8s", "bash"
	Category string // "language", "technology", ...
	Rank     Rank
	Notes    string
	Updated  time.Time
}

// Scope says where a context entry applies.
type Scope struct {
	// Repo is the absolute path of the repository the entry applies to.
	// Empty means the entry is global.
	Repo string
}

func (s Scope) IsGlobal() bool { return s.Repo == "" }

func (s Scope) String() string {
	if s.IsGlobal() {
		return "global"
	}
	return s.Repo
}

// ContextEntry is a piece of user-authored agent context, e.g. the body
// of an AGENTS.md section, tracked in praxis and synced to harnesses.
type ContextEntry struct {
	ID      int64
	Scope   Scope
	Title   string
	Body    string
	Updated time.Time
}

// AgentSkill is a skill package installed for a harness (e.g. an entry
// under .claude/skills). Praxis lists these; harness adapters find them.
type AgentSkill struct {
	Harness     string
	Name        string
	Description string
	Path        string
}

// Settings are praxis-wide knobs.
type Settings struct {
	GlobalAutonomy AutonomyMode
}

// Policy is the per-skill instruction set praxis renders into harness
// context files. It is derived, never stored.
type Policy struct {
	Skill       string
	AllowEdits  bool // may the agent write files touching this skill?
	Explain     bool // must the agent explain reasoning before acting?
	Quiz        bool // should the agent quiz the user on new concepts?
	UserTypes   bool // should the user type the code themselves?
	Description string
}

// PolicyFor derives the agent policy for one skill given the global
// autonomy mode. The global mode caps what any rank is allowed:
// manual mode means the user types everything regardless of rank.
func PolicyFor(skill UserSkill, mode AutonomyMode) Policy {
	p := Policy{Skill: skill.Name}

	switch mode {
	case AutonomyManual:
		p.AllowEdits = false
		p.UserTypes = true
		p.Explain = true
		p.Quiz = skill.Rank <= RankIntermediate
	case AutonomyGuided:
		p.AllowEdits = skill.Rank >= RankIntermediate
		p.UserTypes = skill.Rank <= RankBeginner
		p.Explain = skill.Rank <= RankAdvanced
		p.Quiz = skill.Rank <= RankBeginner
	case AutonomyFull:
		p.AllowEdits = true
		p.UserTypes = false
		p.Explain = skill.Rank <= RankBeginner
		p.Quiz = false
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Skill %q (rank: %s): ", skill.Name, skill.Rank)
	switch {
	case p.UserTypes:
		b.WriteString("do NOT write code for this skill — explain what to write and have the user type it. ")
	case !p.AllowEdits:
		b.WriteString("do not edit files for this skill without explicit approval. ")
	default:
		b.WriteString("you may edit files for this skill. ")
	}
	if p.Explain {
		b.WriteString("Explain your reasoning before acting. ")
	}
	if p.Quiz {
		b.WriteString("Quiz the user on concepts they haven't seen before.")
	}
	p.Description = strings.TrimSpace(b.String())
	return p
}
