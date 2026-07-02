package main

import (
	"fmt"
	"io"
	"strconv"

	"github.com/michael-duren/praxis/internal/domain"
	"github.com/michael-duren/praxis/internal/harness"
	"github.com/michael-duren/praxis/internal/orchestrator"
	"github.com/michael-duren/praxis/internal/store"
)

// cli implements the non-TUI subcommands so state can be scripted.
type cli struct {
	st       *store.Store
	adapters []harness.Adapter
	out      io.Writer
}

// printf writes CLI output. A failed write to the output stream is not
// actionable here, so the error is deliberately discarded (errcheck-clean).
func (c *cli) printf(format string, args ...any) {
	_, _ = fmt.Fprintf(c.out, format, args...)
}

func (c *cli) usage() {
	c.printf(`praxis — learn while you build

usage:
  praxis                          open the TUI
  praxis setup                    interactive first-run setup wizard
  praxis web [addr]               serve the web UI (default 127.0.0.1:8642)
  praxis skill add <name> [rank] [category]
  praxis skill list
  praxis context add <title> <body> [repo]
  praxis context list
  praxis context edit <id> <title> <body> [repo]
  praxis context rm <id>
  praxis harness list
  praxis harness enable <name>
  praxis harness disable <name>
  praxis sync                     write context files to enabled harnesses
  praxis version                  print version, commit, and build date

flags:
  --debug-db                      use a throwaway database ($TMPDIR/praxis-debug.db)
                                  instead of the real one; overrides PRAXIS_DB
`)
}

func (c *cli) skill(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("skill: want add or list")
	}
	switch args[0] {
	case "add":
		if len(args) < 2 {
			return fmt.Errorf("skill add: name required")
		}
		sk := domain.UserSkill{Name: args[1], Rank: domain.RankNovice}
		if len(args) > 2 {
			rank, err := domain.ParseRank(args[2])
			if err != nil {
				return err
			}
			sk.Rank = rank
		}
		if len(args) > 3 {
			sk.Category = args[3]
		}
		if _, err := c.st.UpsertSkill(sk); err != nil {
			return err
		}
		c.printf("added %s (%s)\n", sk.Name, sk.Rank)
		return nil
	case "list":
		skills, err := c.st.Skills()
		if err != nil {
			return err
		}
		for _, sk := range skills {
			c.printf("%-16s %-12s %s\n", sk.Name, sk.Rank, sk.Category)
		}
		return nil
	}
	return fmt.Errorf("skill: unknown subcommand %q", args[0])
}

func (c *cli) context(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("context: want add, list, edit, or rm")
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			return fmt.Errorf("context add: want <title> <body> [repo]")
		}
		e := domain.ContextEntry{Title: args[1], Body: args[2]}
		if len(args) > 3 {
			e.Scope.Repo = args[3]
		}
		if _, err := c.st.UpsertContextEntry(e); err != nil {
			return err
		}
		c.printf("added context %q (%s)\n", e.Title, e.Scope)
		return nil
	case "list":
		entries, err := c.st.ContextEntries()
		if err != nil {
			return err
		}
		for _, e := range entries {
			c.printf("%-4d %-28s %s\n", e.ID, e.Title, e.Scope)
		}
		return nil
	case "edit":
		if len(args) < 4 {
			return fmt.Errorf("context edit: want <id> <title> <body> [repo]")
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("context edit: bad id %q", args[1])
		}
		e := domain.ContextEntry{ID: id, Title: args[2], Body: args[3]}
		if len(args) > 4 {
			e.Scope.Repo = args[4]
		}
		if _, err := c.st.UpsertContextEntry(e); err != nil {
			return err
		}
		c.printf("updated context %d %q (%s)\n", e.ID, e.Title, e.Scope)
		return nil
	case "rm":
		if len(args) < 2 {
			return fmt.Errorf("context rm: want <id>")
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("context rm: bad id %q", args[1])
		}
		if err := c.st.DeleteContextEntry(id); err != nil {
			return err
		}
		c.printf("deleted context %d\n", id)
		return nil
	}
	return fmt.Errorf("context: unknown subcommand %q", args[0])
}

func (c *cli) harness(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("harness: want list, enable, or disable")
	}
	switch args[0] {
	case "list":
		enabled, err := c.st.EnabledHarnesses()
		if err != nil {
			return err
		}
		for _, a := range c.adapters {
			state := "disabled"
			if enabled[a.Name()] {
				state = "enabled"
			}
			detected := ""
			if a.Detect() {
				detected = " (detected)"
			}
			c.printf("%-12s %s%s\n", a.Name(), state, detected)
		}
		return nil
	case "enable", "disable":
		if len(args) < 2 {
			return fmt.Errorf("harness %s: name required", args[0])
		}
		if !c.knownHarness(args[1]) {
			return fmt.Errorf("unknown harness %q", args[1])
		}
		if err := c.st.SetHarnessEnabled(args[1], args[0] == "enable"); err != nil {
			return err
		}
		c.printf("%s %sd\n", args[1], args[0])
		return nil
	}
	return fmt.Errorf("harness: unknown subcommand %q", args[0])
}

func (c *cli) knownHarness(name string) bool {
	for _, a := range c.adapters {
		if a.Name() == name {
			return true
		}
	}
	return false
}

func (c *cli) sync() error {
	skills, err := c.st.Skills()
	if err != nil {
		return err
	}
	entries, err := c.st.ContextEntries()
	if err != nil {
		return err
	}
	settings, err := c.st.LoadSettings()
	if err != nil {
		return err
	}
	enabled, err := c.st.EnabledHarnesses()
	if err != nil {
		return err
	}

	st := orchestrator.State{Skills: skills, Entries: entries, Settings: settings}
	scopes := map[domain.Scope]bool{{}: true}
	for _, e := range entries {
		scopes[e.Scope] = true
	}
	var firstErr error
	for scope := range scopes {
		for _, r := range orchestrator.Sync(st, scope, c.adapters, enabled) {
			if r.Err != nil {
				c.printf("ERROR %s (%s): %v\n", r.Harness, r.Scope, r.Err)
				if firstErr == nil {
					firstErr = r.Err
				}
				continue
			}
			c.printf("wrote %s\n", r.Path)
		}
	}
	return firstErr
}
