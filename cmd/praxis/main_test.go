package main

import (
	"bytes"
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
	for _, want := range []string{"praxis web", "skill add", "sync"} {
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
