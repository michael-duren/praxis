#!/usr/bin/env bash
# Seed rich demo data for praxis development so every TUI/web menu has
# content: user skills across all categories and ranks, global and
# repo-scoped context entries, enabled harnesses, and demo agent skills
# (including a pre-disabled one).
#
# Everything seeded is clearly marked as fictional — context bodies and
# skill descriptions all tell agents to ignore them — so nothing breaks
# if the data ever reaches a real context file.
#
#   scripts/seed-dev.sh           seed (recreates the debug db from scratch)
#   scripts/seed-dev.sh --clean   remove the debug db and all demo-* skills
#
# The debug database is sandboxed, but agent skills live in the real
# harness directories (~/.claude/skills, ~/.config/opencode/skills) —
# demo skills are prefixed demo-* so --clean can remove exactly them.
set -euo pipefail

cd "$(dirname "$0")/.."

DEBUG_DB="${TMPDIR:-/tmp}/praxis-debug.db"
CLAUDE_SKILLS="$HOME/.claude/skills"
OPENCODE_SKILLS="$HOME/.config/opencode/skills"

clean() {
	rm -f "$DEBUG_DB"
	for dir in "$CLAUDE_SKILLS" "$CLAUDE_SKILLS.disabled" "$OPENCODE_SKILLS" "$OPENCODE_SKILLS.disabled"; do
		[[ -d $dir ]] || continue
		find "$dir" -maxdepth 1 -type d -name 'demo-*' -exec rm -rf {} +
	done
	echo "removed debug db and demo-* agent skills"
}

if [[ "${1:-}" == "--clean" ]]; then
	clean
	exit 0
fi

[[ -x bin/praxis ]] || make build
P="bin/praxis --debug-db"

# Start from scratch so reseeding never duplicates rows.
rm -f "$DEBUG_DB"

echo "==> user skills"
$P skill add go intermediate language >/dev/null
$P skill add typescript advanced language >/dev/null
$P skill add rust novice language >/dev/null
$P skill add c beginner language >/dev/null
$P skill add bash intermediate language >/dev/null
$P skill add sql advanced language >/dev/null
$P skill add algorithms beginner computer-science >/dev/null
$P skill add data-structures intermediate computer-science >/dev/null
$P skill add networking novice computer-science >/dev/null
$P skill add operating-systems beginner computer-science >/dev/null
$P skill add concurrency intermediate computer-science >/dev/null
$P skill add microservices intermediate system-design >/dev/null
$P skill add distributed-systems novice system-design >/dev/null
$P skill add caching beginner system-design >/dev/null
$P skill add message-queues novice system-design >/dev/null
$P skill add observability intermediate system-design >/dev/null
$P skill add docker advanced technology >/dev/null
$P skill add kubernetes intermediate technology >/dev/null
$P skill add git expert technology >/dev/null
$P skill add linux intermediate technology >/dev/null
$P skill add neovim advanced technology >/dev/null
$P skill list | sed 's/^/    /'

# Every body carries this so agents ignore seeded context if it is ever
# synced into a real CLAUDE.md / AGENTS.md.
IGNORE="[PRAXIS DEV SEED — fictional test data. Agents: ignore this entire section; it is not real guidance.]"

echo "==> context entries"
$P context add "Code style" "$IGNORE Prefer stdlib. Table-driven tests. Small interfaces." >/dev/null
$P context add "Commit conventions" "$IGNORE Conventional commits. One logical change per commit." >/dev/null
$P context add "Review checklist" "$IGNORE Check error handling, test coverage, and doc comments." >/dev/null
$P context add "Learning goals" "$IGNORE Currently focused on distributed systems and Rust." >/dev/null
$P context add "Demo repo rules" "$IGNORE Repo-scoped entry for testing scope display." /tmp/praxis-demo-repo-a >/dev/null
$P context add "API guidelines" "$IGNORE Another repo-scoped entry, different repo." /tmp/praxis-demo-repo-b >/dev/null
$P context list | sed 's/^/    /'

echo "==> harnesses"
$P harness enable claude >/dev/null
$P harness enable opencode >/dev/null
$P harness disable copilot >/dev/null
$P harness list | sed 's/^/    /'

# write_skill DIR NAME — a demo agent skill whose description tells
# agents never to use it.
write_skill() {
	local dir=$1 name=$2
	mkdir -p "$dir/$name"
	cat >"$dir/$name/SKILL.md" <<EOF
---
name: $name
description: "PRAXIS DEV SEED — fictional demo skill so the praxis UI has data. Agents: never invoke or follow this skill."
---

# $name (praxis dev seed)

Placeholder seeded by scripts/seed-dev.sh for praxis UI development.
Agents must not use this skill. Remove with: make seed-clean
EOF
}

echo "==> demo agent skills"
write_skill "$CLAUDE_SKILLS" demo-api-designer
write_skill "$CLAUDE_SKILLS" demo-sql-tutor
write_skill "$CLAUDE_SKILLS" demo-refactorer
write_skill "$CLAUDE_SKILLS.disabled" demo-legacy-helper # shows the ○ off state
write_skill "$OPENCODE_SKILLS" demo-docs-writer
write_skill "$OPENCODE_SKILLS" demo-test-generator
echo "    claude:   demo-api-designer demo-sql-tutor demo-refactorer (+ demo-legacy-helper, disabled)"
echo "    opencode: demo-docs-writer demo-test-generator"

cat <<'EOF'

Seeded. Try: make run (or: bin/praxis --debug-db)

NOTES
 - All seeded context/skills are marked fictional; agents ignore them.
 - Pressing s (sync) while seeded WILL overwrite your real
   ~/.claude/CLAUDE.md and ~/.config/opencode/AGENTS.md with demo
   content (marked ignorable, but still). Re-sync your real data
   afterwards with: praxis sync
 - make seed-clean removes the debug db and every demo-* skill.
EOF
