# LDR 0006 — Context entry editing everywhere

**Date:** 2026-07-02

## What

Context entries can now be edited and deleted from all three surfaces:

- **CLI**: `context list` (shows IDs), `context edit <id> <title> <body>
  [repo]`, `context rm <id>`.
- **Web**: each entry renders as an inline form (title/repo/body) with
  Save and Delete (with `hx-confirm`) — `POST /context/{id}` and
  `POST /context/{id}/delete`.
- **TUI**: on the Context tab, `a` adds, `e` edits, `d` (pressed twice on
  the same entry) deletes. Add/edit is a three-field form filled one line
  at a time (title → body → repo, empty repo = global).

## Why / key decisions

- **`store.ErrNotFound`**: updates/deletes of missing rows now error
  instead of silently succeeding; the web layer maps it to 404 via
  `errors.Is`. Wrapped with `%w` so callers match the sentinel, not text.
- **TUI text input is hand-rolled** (same pattern as the setup wizard's
  add-your-own): one buffer, rune append/backspace, enter to advance.
  Single-line body only — long context bodies are better edited in the
  web UI or via CLI.
- **Double-`d` delete** instead of a modal: any other key resets the
  pending confirmation. Cursor clamps and re-scrolls after deletion.
- Footer key hints are now per-tab (`footerKeys`), since the one-line
  global list stopped fitting.

## Where

- `internal/store/store.go` — `ErrNotFound`, RowsAffected checks
- `cmd/praxis/cli.go` — context subcommands
- `internal/tui/model.go` — `handleEditKey`, `deleteContext`,
  `viewContextEdit`, `footerKeys`
- `internal/web/server.go` + `templates/app.templ` — update/delete
  handlers and inline entry forms
