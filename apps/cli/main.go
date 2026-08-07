// Command control-planectl is the operator CLI for the control plane.
// It is a placeholder in Milestone 1 — real subcommands (execution
// inspection, workflow registration, policy testing) land alongside
// the domain milestones that back them.
package main

import (
	"fmt"
	"os"
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
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "control-planectl: unknown command %q\n\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`control-planectl - AI Agent Control Plane CLI

Usage:
  control-planectl <command>

Commands:
  version   Print the CLI version
  help      Show this help text

More commands (executions, workflows, policies) arrive with the
milestones that implement those subsystems.`)
}
