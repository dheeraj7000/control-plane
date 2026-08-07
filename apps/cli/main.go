// Command control-planectl is the operator CLI for the control plane.
// `migrate up` landed in Milestone 7 alongside Postgres persistence —
// real execution/workflow inspection and policy testing subcommands
// are still owed, pending the milestones/decisions (stored policy
// records, in particular) that would make them meaningful.
package main

import (
	"fmt"
	"os"

	"github.com/dheeraj7000/control-plane/internal/config"
	"github.com/dheeraj7000/control-plane/internal/storage"
)

var version = "0.1.0-dev"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return
	}

	switch args[0] {
	case "version":
		fmt.Println("control-planectl " + version)
	case "migrate":
		runMigrate(args[1:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "control-planectl: unknown command %q\n\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func runMigrate(args []string) {
	if len(args) != 1 || args[0] != "up" {
		fmt.Fprintln(os.Stderr, "usage: control-planectl migrate up")
		os.Exit(1)
	}

	// Reuses internal/config so this respects the same DATABASE_URL
	// (and its local-dev default) the server itself would use —
	// deliberately not a separate ad hoc flag/env var.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "control-planectl: "+err.Error())
		os.Exit(1)
	}

	if err := storage.Migrate(cfg.DatabaseURL); err != nil {
		fmt.Fprintln(os.Stderr, "control-planectl: migrate: "+err.Error())
		os.Exit(1)
	}
	fmt.Println("migrations applied")
}

func printUsage() {
	fmt.Println(`control-planectl - AI Agent Control Plane CLI

Usage:
  control-planectl <command>

Commands:
  version      Print the CLI version
  migrate up   Apply pending Postgres migrations (uses DATABASE_URL, see internal/config)
  help         Show this help text

Execution/workflow inspection and policy testing subcommands are still
owed — see docs/architecture.md's open questions.`)
}
