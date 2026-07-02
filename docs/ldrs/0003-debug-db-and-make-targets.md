# LDR 0003 — Debug database flag and dev make targets

**Date:** 2026-07-02

## What

- `--debug-db` flag: any praxis command runs against
  `$TMPDIR/praxis-debug.db` instead of the real database. Position-
  independent; beats `PRAXIS_DB`; announces itself on every run.
- Makefile grew a dev workflow: `run`/`web`/`setup`/`seed` all pass
  `--debug-db`; `lint`, `fmt`, `cover`, `tidy`, `ci` (mirrors CI exactly),
  and a self-documenting `help` target via `##` comments.
- `make release VERSION=vX.Y.Z` validates (semver, clean tree, on main,
  synced with origin), runs `make ci`, then tags and pushes — the tag
  triggers the release workflow (six-platform build, gated by a verify job).

## Why

Development should never touch `~/.local/share/praxis/praxis.db`. A stable
temp path (not per-run) lets you seed with the CLI and then browse in the
TUI/web. Caveat: `sync` still writes context files to the real home
directory — only the database is sandboxed.

## Where

- `cmd/praxis/main.go` — `stripFlag`, debug path selection
- `Makefile` — all targets; `make help` lists them
