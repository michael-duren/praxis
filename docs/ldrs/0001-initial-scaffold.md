# LDR 0001 — Initial scaffold

**Date:** 2026-07-02

## What

First working version of praxis: domain model, SQLite store, harness
adapters, context orchestrator, Bubble Tea TUI with 7 themes, templ/HTMX
web UI, and a small CLI. Every `.go` file has a `_test.go` beside it.

## Why / key decisions

- **`internal/domain` has zero dependencies** — every other package depends
  on it, never the reverse. Policies (`PolicyFor`) are *derived* from rank +
  autonomy mode, never stored, so there's one source of truth.
- **SQLite via `modernc.org/sqlite`** (pure Go, no cgo) at
  `~/.local/share/praxis/praxis.db`; `PRAXIS_DB` env var overrides (tests use
  temp dirs). DB is the source of truth; harness files are derived state.
- **Small harness interfaces** (`Adapter`, `ContextWriter`, `SkillLister` in
  `internal/harness`) — adapters implement only what their harness supports
  (Copilot has no skills dir, so no `SkillLister`). New harnesses = one file.
- **Orchestrator is pure** — `Render(state, scope)` returns markdown;
  `Sync` writes it via whatever adapters are enabled. No DB access inside.
- **Tailwind via CDN** in the web layout for now — no node toolchain needed.
  Swap for the standalone tailwind CLI when styles grow.
- **templ generated code is checked in** so `go build` works without templ
  installed; `make generate` regenerates.

## Where

- `internal/domain` — types, ranks, autonomy, policy derivation
- `internal/store` — SQLite persistence
- `internal/harness` — adapter interfaces + claude/opencode/copilot
- `internal/orchestrator` — render + sync context documents
- `internal/tui` — Bubble Tea UI, `theme.go` for themes
- `internal/web` — HTTP server; `templates/app.templ` for markup
- `cmd/praxis` — entrypoint + CLI subcommands
