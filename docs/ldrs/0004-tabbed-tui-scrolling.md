# LDR 0004 — Full-screen tabbed TUI with scrolling

**Date:** 2026-07-02

## What

Reworked the TUI: one tab at a time in a full-screen accent-bordered
pane, with scrolling lists. The active tab's list keeps the cursor in
view (`j`/`k`, plus `g`/`G` to jump top/bottom) and shows a `cursor/total`
position indicator on the tab bar when the list overflows the pane.

## Why / key decisions

- A multi-pane dashboard was tried first and rejected: navigating focus
  *spatially* (right column for Autonomy/Harnesses) was awkward. Tabs
  with h/l cycling won; the full-screen pane gives skills all the width
  they need.
- Scrolling is offset-based (`offset` + `scroll()` clamp) rather than a
  bubbles viewport — the lists are plain strings and the cursor logic
  needs to own the window anyway.
- Skill rows budget name/category columns from the pane width and
  truncate with `…` instead of wrapping, which broke row alignment.
- Kept from the dashboard experiment: accent-colored pane border,
  denser padding, colored rank bars, width-aware layout via
  `tea.WindowSizeMsg`, footer pinned to the last terminal row.

## Where

- `internal/tui/model.go` — `section` enum, `scroll`/`window` viewport
  math, per-section `view*` builders, `View` layout
- `internal/tui/theme.go` — tab styles, accent pane border
