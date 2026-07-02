# praxis

Praxis makes **you** smarter while getting things done. It orchestrates the
context files your coding agents read (CLAUDE.md, AGENTS.md,
copilot-instructions.md, ...) so that agents explain their reasoning, have you
type code yourself when you're still learning, and quiz you on subjects you
don't know well — calibrated to your skill ranks and autonomy settings.

## Usage

```sh
praxis setup      # first-run wizard: pick skills, ranks, autonomy, harnesses
praxis            # open the TUI
praxis web        # serve the web UI on http://127.0.0.1:8642
praxis help       # CLI subcommands for scripting (skill, context, harness, sync)
```

## How it works

- **User skills** — topics you're learning (go, k8s, bash, ...) each with a
  rank from novice to expert.
- **Autonomy** — a global mode (`manual` / `guided` / `full`) combined with
  per-skill ranks derives a *policy* per skill: may the agent edit files, must
  it explain first, should it quiz you, do you type the code yourself.
- **Context** — your own agent context entries, global or per-repo.
- **Harnesses** — adapters for Claude Code, OpenCode, and GitHub Copilot know
  where each harness reads its context. Enable the ones you use; `sync`
  renders one document per scope and writes it everywhere.
- **State** — SQLite (`~/.local/share/praxis/praxis.db`, override with
  `PRAXIS_DB`) is the source of truth; harness files are derived and can be
  recreated at any point.

## Development

```sh
make          # generate templ code, build, test
make run      # build and open the TUI
make web      # build and serve the web UI
```

Requires Go 1.26+ and [templ](https://templ.guide) (`go install github.com/a-h/templ/cmd/templ@latest`).

Docs live in `docs/current.md` (codebase overview) and `docs/ldrs/`
(lightweight decision records, one per iteration).
