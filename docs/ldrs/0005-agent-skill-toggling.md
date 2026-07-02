# LDR 0005 — Agent skill enable/disable

**Date:** 2026-07-02

## What

Agent skills (entries under a harness's skills directory) can now be
enabled and disabled from the TUI (space on the Agent Skills tab) and the
web UI (toggle button). `AgentSkill` gained an `Enabled` field, and a new
`SkillToggler` capability interface joins `ContextWriter`/`SkillLister`.

## Why / key decisions

- **The filesystem is the state, not the database.** Disabling moves the
  skill directory to a sibling `skills.disabled` directory (e.g.
  `~/.claude/skills.disabled/go-teacher`), which genuinely stops the
  harness from loading it. No praxis-only flag that the agent would
  ignore. Listing merges both directories, sorted by name so toggling
  doesn't reorder the list.
- Toggling is idempotent (already-disabled + disable = no-op) and unknown
  skills error.
- Claude and OpenCode implement `SkillToggler`; Copilot has no skills
  concept, so it implements neither `SkillLister` nor `SkillToggler` —
  the UIs discover this by type assertion, consistent with LDR 0001.

## Where

- `internal/harness/harness.go` — `SkillToggler`, `listSkills`,
  `setSkillEnabled`
- `internal/harness/{claude,opencode}.go` — `skillsRoot`, implementations
- `internal/tui/model.go` — `toggleAgentSkill`, on/off markers
- `internal/web/server.go` — `POST /agent-skill/{harness}/{name}/toggle`
