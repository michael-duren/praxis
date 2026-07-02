package main

import (
	"fmt"
	"io"

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

func (c *cli) usage() {
	fmt.Fprint(c.out, `praxis — learn while you build

usage:
  praxis                          open the TUI
  praxis web [addr]               serve the web UI (default 127.0.0.1:8642)
  praxis skill add <name> [rank] [category]
  praxis skill list
  praxis context add <title> <body> [repo]
  praxis harness list
  praxis harness enable <name>
  praxis harness disable <name>
  praxis sync                     write context files to enabled harnesses
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
		fmt.Fprintf(c.out, "added %s (%s)\n", sk.Name, sk.Rank)
		return nil
	case "list":
		skills, err := c.st.Skills()
		if err != nil {
			return err
		}
		for _, sk := range skills {
			fmt.Fprintf(c.out, "%-16s %-12s %s\n", sk.Name, sk.Rank, sk.Category)
		}
		return nil
	}
	return fmt.Errorf("skill: unknown subcommand %q", args[0])
}

func (c *cli) context(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return fmt.Errorf("context: want add <title> <body> [repo]")
	}
	if len(args) < 3 {
		return fmt.Errorf("context add: title and body required")
	}
	e := domain.ContextEntry{Title: args[1], Body: args[2]}
	if len(args) > 3 {
		e.Scope.Repo = args[3]
	}
	if _, err := c.st.UpsertContextEntry(e); err != nil {
		return err
	}
	fmt.Fprintf(c.out, "added context %q (%s)\n", e.Title, e.Scope)
	return nil
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
			fmt.Fprintf(c.out, "%-12s %s%s\n", a.Name(), state, detected)
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
		fmt.Fprintf(c.out, "%s %sd\n", args[1], args[0])
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
				fmt.Fprintf(c.out, "ERROR %s (%s): %v\n", r.Harness, r.Scope, r.Err)
				if firstErr == nil {
					firstErr = r.Err
				}
				continue
			}
			fmt.Fprintf(c.out, "wrote %s\n", r.Path)
		}
	}
	return firstErr
}
