// Command praxis is the entrypoint: `praxis` opens the TUI, `praxis web`
// serves the web UI, and a few subcommands allow scripting the same state.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/michael-duren/praxis/internal/harness"
	"github.com/michael-duren/praxis/internal/store"
	"github.com/michael-duren/praxis/internal/tui"
	"github.com/michael-duren/praxis/internal/web"
)

// Build metadata, overridden at release time via
// -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "praxis:", err)
		os.Exit(1)
	}
}

// run dispatches CLI args. It is separated from main for testing.
func run(args []string, out io.Writer) error {
	// version and help need no database; handle them before opening one.
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version":
			_, err := fmt.Fprintf(out, "praxis %s\ncommit:  %s\nbuilt:   %s\n", version, commit, date)
			return err
		}
	}

	dbPath, err := store.DefaultPath()
	if err != nil {
		return err
	}
	if env := os.Getenv("PRAXIS_DB"); env != "" {
		dbPath = env
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	adapters := harness.All(tui.Home())
	cli := &cli{st: st, adapters: adapters, out: out}

	if len(args) == 0 {
		return tui.Run(st, adapters)
	}
	switch args[0] {
	case "web":
		addr := "127.0.0.1:8642"
		if len(args) > 1 {
			addr = args[1]
		}
		return web.New(st, adapters).ListenAndServe(addr)
	case "skill":
		return cli.skill(args[1:])
	case "context":
		return cli.context(args[1:])
	case "harness":
		return cli.harness(args[1:])
	case "sync":
		return cli.sync()
	case "help", "-h", "--help":
		cli.usage()
		return nil
	default:
		cli.usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}
