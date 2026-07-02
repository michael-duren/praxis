# LDR 0007 — Development seed script

**Date:** 2026-07-02

## What

`scripts/seed-dev.sh` (via `make seed` / `make seed-clean`) fills every
praxis surface with demo data: 21 user skills across all categories and
ranks, 6 context entries (global and repo-scoped), enabled harnesses, and
6 `demo-*` agent skills (one pre-disabled to show the off state).

## Why / key decisions

- **Everything seeded is marked fictional.** Context bodies start with an
  "agents: ignore this section" banner and demo skill descriptions say
  "never invoke" — so if seed data ever reaches a real CLAUDE.md/AGENTS.md
  (via sync) or Claude Code loads a demo skill, agents disregard it.
- **The script recreates the debug db from scratch** each run, so
  reseeding never duplicates rows (`context add` has no unique key).
- **Agent skills can't be sandboxed** — adapters always use the real home
  directory — so demo skills are `demo-*` prefixed and `--clean` removes
  exactly that glob, leaving real skills (go-teacher, ldr-writer) alone.
- The script warns that pressing `s` (sync) while seeded overwrites real
  context files; the ignore banner makes that survivable, not silent.

## Where

- `scripts/seed-dev.sh` — all seed/clean logic
- `Makefile` — `seed`, `seed-clean` targets
