# LDR 0002 — Setup wizard

**Date:** 2026-07-02

## What

`praxis setup`: a first-run wizard as its own small Bubble Tea program
(`internal/setup`). One question per screen: four skill-category
multi-selects (programming languages, computer science, system design,
technologies & tools) each with preset options plus add-your-own, a rank
step for the chosen skills, autonomy choice, harness enablement, and an
optional immediate sync.

## Why / key decisions

- **Separate package from `internal/tui`** — the wizard is linear
  question/answer, the main TUI is tab-based browsing; sharing a model
  would complicate both. They share only the store and adapters.
- **Nothing persists until the end** — quitting midway (q/ctrl+c) saves
  nothing. `Model.Save` runs *after* the Bubble Tea program exits, keeping
  I/O out of the update loop and making the whole flow unit-testable by
  feeding key messages and then calling `Save`.
- **Detected harnesses are preselected** — adapters' `Detect()` marks and
  preselects harnesses found on the machine.
- **Custom skills via a modal input**, not a bubbles dependency: `a` opens
  a one-line input handled by ~20 lines of key handling.
- **Rank step is skipped** (forward and backward) when no skills were
  selected.

## Where

- `internal/setup/setup.go` — wizard model, steps, Save
- `cmd/praxis/main.go` — `setup` subcommand dispatch
