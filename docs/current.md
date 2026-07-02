# praxis — codebase overview

*Last updated: 2026-07-02 (LDR 0001)*

Praxis manages the context files coding-agent harnesses read, so agents
teach the user while working. State lives in SQLite; context files in each
harness are derived from it and can be regenerated at any time with `sync`.

## Data flow

```
user edits state (TUI / web / CLI)
        ↓
internal/store (SQLite: skills, context entries, harnesses, settings)
        ↓ load into
internal/orchestrator.State ── Render(scope) → markdown doc
        ↓ Sync
internal/harness adapters → ~/.claude/CLAUDE.md, AGENTS.md,
                            .github/copilot-instructions.md, ...
```

## Where to edit what

| I want to change... | Go to |
|---|---|
| Skill ranks, autonomy modes, policy rules | `internal/domain/domain.go` (`PolicyFor`) |
| Database schema or queries | `internal/store/store.go` (`migrate`) |
| What the generated context doc says | `internal/orchestrator/orchestrator.go` (`Render`) |
| Where a harness's files live / add a harness | `internal/harness/<name>.go`; register in `All()` |
| TUI behavior/keys | `internal/tui/model.go` |
| TUI colors/themes | `internal/tui/theme.go` (`Themes`) |
| Web pages/markup | `internal/web/templates/app.templ`, then `make generate` |
| Web routes/handlers | `internal/web/server.go` (`Handler`) |
| CLI subcommands | `cmd/praxis/cli.go` |

## Key concepts

- **Rank** (novice→expert) per user skill; **AutonomyMode**
  (manual/guided/full) globally. `domain.PolicyFor(skill, mode)` derives what
  agents may do: edit files, explain first, quiz, or make the user type.
- **Scope**: a context entry is global (empty repo path) or per-repo.
  Rendering a repo scope includes global entries too.
- **Adapter capabilities are interfaces**: `ContextWriter`, `SkillLister`.
  The orchestrator type-asserts, so a harness lacking a capability is
  skipped, not special-cased.

## Conventions

- Every `.go` file has a `_test.go` next to it; tests use `t.TempDir()`
  (never the real home dir or DB).
- `PRAXIS_DB` overrides the database path.
- `make` = generate + build + test. templ output (`*_templ.go`) is checked in.
