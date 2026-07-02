package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI executes the praxis CLI against a throwaway database.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	t.Setenv("PRAXIS_DB", filepath.Join(t.TempDir(), "praxis.db"))
	var buf bytes.Buffer
	err := run(args, &buf)
	return buf.String(), err
}

func TestDebugDBFlag(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp) // os.TempDir() honors TMPDIR on unix
	// The flag must win over PRAXIS_DB, so point that somewhere else.
	t.Setenv("PRAXIS_DB", filepath.Join(t.TempDir(), "real.db"))

	var buf bytes.Buffer
	// Flag position should not matter.
	if err := run([]string{"skill", "add", "go", "--debug-db"}, &buf); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(buf.String(), "debug db:") {
		t.Errorf("output should announce the debug db, got %q", buf.String())
	}
	if _, err := os.Stat(filepath.Join(tmp, "praxis-debug.db")); err != nil {
		t.Errorf("debug db not created in TMPDIR: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "..", "real.db")); err == nil {
		t.Error("PRAXIS_DB path should be untouched when --debug-db is set")
	}

	// The seeded skill is visible on a second run against the same debug db.
	buf.Reset()
	if err := run([]string{"--debug-db", "skill", "list"}, &buf); err != nil {
		t.Fatalf("run list: %v", err)
	}
	if !strings.Contains(buf.String(), "go") {
		t.Errorf("debug db should persist between runs, got %q", buf.String())
	}
}

func TestVersionCommand(t *testing.T) {
	out, err := runCLI(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	// Field alignment matters: release.yml's sanity check greps for
	// "commit:  " (two spaces) and "built:   " (three spaces).
	for _, want := range []string{"praxis dev", "commit:  none", "built:   unknown"} {
		if !strings.Contains(out, want) {
			t.Errorf("version output missing %q:\n%s", want, out)
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	out, err := runCLI(t, "bogus")
	if err == nil {
		t.Error("want error for unknown command")
	}
	if !strings.Contains(out, "usage") {
		t.Errorf("unknown command should print usage, got %q", out)
	}
}

func TestHelp(t *testing.T) {
	out, err := runCLI(t, "help")
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, want := range []string{"praxis setup", "praxis web", "skill add", "sync"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestSkillAddAndList(t *testing.T) {
	db := filepath.Join(t.TempDir(), "praxis.db")
	t.Setenv("PRAXIS_DB", db)

	var buf bytes.Buffer
	if err := run([]string{"skill", "add", "go", "beginner", "language"}, &buf); err != nil {
		t.Fatalf("skill add: %v", err)
	}
	buf.Reset()
	if err := run([]string{"skill", "list"}, &buf); err != nil {
		t.Fatalf("skill list: %v", err)
	}
	if !strings.Contains(buf.String(), "go") || !strings.Contains(buf.String(), "beginner") {
		t.Errorf("skill list = %q", buf.String())
	}
}

func TestSkillAddBadRank(t *testing.T) {
	_, err := runCLI(t, "skill", "add", "go", "wizard")
	if err == nil || !strings.Contains(err.Error(), "unknown rank") {
		t.Errorf("err = %v, want unknown rank", err)
	}
}

func TestContextAddRequiresBody(t *testing.T) {
	if _, err := runCLI(t, "context", "add", "title-only"); err == nil {
		t.Error("want error when body missing")
	}
}

func TestHarnessEnableUnknown(t *testing.T) {
	_, err := runCLI(t, "harness", "enable", "nope")
	if err == nil || !strings.Contains(err.Error(), "unknown harness") {
		t.Errorf("err = %v, want unknown harness", err)
	}
}

func TestHarnessList(t *testing.T) {
	out, err := runCLI(t, "harness", "list")
	if err != nil {
		t.Fatalf("harness list: %v", err)
	}
	for _, want := range []string{"claude", "opencode", "copilot"} {
		if !strings.Contains(out, want) {
			t.Errorf("harness list missing %q: %q", want, out)
		}
	}
}
