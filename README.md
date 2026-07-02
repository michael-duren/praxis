# praxis

Praxis makes **you** smarter while getting things done. It orchestrates the
context files your coding agents read (CLAUDE.md, AGENTS.md,
copilot-instructions.md, ...) so that agents explain their reasoning, have you
type code yourself when you're still learning, and quiz you on subjects you
don't know well — calibrated to your skill ranks and autonomy settings.

https://github.com/user-attachments/assets/efb45639-c374-48af-915a-6569d3431af8

## Features

- **User skills with ranks** — track what you're learning (languages,
  CS topics, system design, technologies) from novice to expert; ranks
  drive how much agents may do in that skill's territory.
- **Autonomy policies** — a global mode (`manual` / `guided` / `full`)
  combines with per-skill ranks to derive a policy per skill: may the
  agent edit files, must it explain first, should it quiz you, do you
  type the code yourself.
- **Context orchestration** — your context entries (global or per-repo)
  plus the derived skill policies render into one document, synced to
  every enabled harness's expected location. SQLite is the source of
  truth; harness files can be regenerated at any time.
- **Harness adapters** — Claude Code, OpenCode, and GitHub Copilot are
  built in; each implements only the capabilities its harness supports
  (small interfaces), so new harnesses are one file.
- **Agent skills** — see the skills installed for each harness and
  enable/disable them; disabling moves the skill directory out of the
  harness's search path so it genuinely stops loading.
- **Setup wizard** — `praxis setup` walks through skills (with suggested
  options per category or add your own), ranks, autonomy, and detected
  harnesses, one simple question per screen.
- **TUI** — full-screen tabbed dashboard with scrolling lists, context
  detail view, inline add/edit/delete, and 7 themes (dracula, tokyo
  night, gruvbox, nord, catppuccin, dark, light).
- **Web UI** — the same state served as an HTMX + templ + Tailwind
  dashboard, Khan Academy-styled with light/dark mode: `praxis web`.
- **Scriptable CLI** — every operation is available non-interactively
  (`skill`, `context`, `harness`, `sync`) for automation.

## Install

### Homebrew

```sh
brew install michael-duren/praxis-brew/praxis
```

Builds from source via the [praxis-brew](https://github.com/michael-duren/homebrew-praxis-brew)
tap.

### AUR (Arch Linux)

```sh
yay -S praxis-bin
```

Downloads the release binary for your architecture (x86_64/aarch64) and
verifies it against the published checksum. PKGBUILD source lives in the
[praxis-bin](https://aur.archlinux.org/packages/praxis-bin) AUR package
repo; see [Publishing to the AUR](#publishing-to-the-aur) below for
maintenance.

### Release binaries

Download the binary for your platform from the
[latest release](https://github.com/michael-duren/praxis/releases/latest)
(linux/darwin/windows, amd64/arm64), then:

```sh
install -m 755 praxis-v*-linux-amd64 ~/.local/bin/praxis
praxis version   # verify
```

Checksums are in the release's `SHA256SUMS`.

### go install

```sh
go install github.com/michael-duren/praxis/cmd/praxis@latest
```

Installs to `$GOBIN` (default `~/go/bin`). No extra tools needed — the
generated template code is checked in.

### From source

```sh
git clone https://github.com/michael-duren/praxis
cd praxis && make install
```

Requires Go 1.26+ and [templ](https://templ.guide)
(`go install github.com/a-h/templ/cmd/templ@latest`).

## Usage

```sh
praxis setup      # first-run wizard: pick skills, ranks, autonomy, harnesses
praxis            # open the TUI
praxis web        # serve the web UI on http://127.0.0.1:8642
praxis sync       # write context files to enabled harnesses
praxis help       # all subcommands (skill, context, harness, ...)
```

State lives in `~/.local/share/praxis/praxis.db` (override with
`PRAXIS_DB`; `--debug-db` uses a throwaway database for experimenting).

## Development

```sh
make help         # list all targets
make              # generate templ code, build, test
make ci           # everything CI runs (build, lint, test, vet, gopls, gofmt)
make run          # TUI against a throwaway debug database
make seed         # fill the debug db + demo agent skills with test data
make release VERSION=vX.Y.Z   # validate, tag, push — CI builds all platforms
```

Docs live in `docs/current.md` (codebase overview — start there) and
`docs/ldrs/` (lightweight decision records, one per iteration).

### Publishing to the AUR

The PKGBUILD lives only in the `praxis-bin` AUR git repo, not in this
source tree — same reasoning as the [praxis-brew](https://github.com/michael-duren/homebrew-praxis-brew)
tap being a separate repo from `praxis` itself. After `make release`
publishes a GitHub Release with linux binaries and `SHA256SUMS`:

1. Clone the AUR repo (requires an AUR account and SSH key):
   `git clone ssh://aur@aur.archlinux.org/praxis-bin.git`
2. Bump `pkgver` in `PKGBUILD` and update `sha256sums_x86_64`/
   `sha256sums_aarch64` from the release's `SHA256SUMS` (reset `pkgrel`
   to `1`).
3. Regenerate the checksum file: `makepkg --printsrcinfo > .SRCINFO`.
4. Optionally sanity-check locally: `makepkg -f` builds the package;
   `pacman -U praxis-bin-*.pkg.tar.zst` (or extracting the tarball and
   running `usr/bin/praxis version` directly) confirms it works before
   publishing.
5. `git add -A && git commit -m "vX.Y.Z" && git push`
