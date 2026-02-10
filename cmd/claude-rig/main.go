package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "create":
		err = cmdCreate(args)
	case "list", "ls":
		err = cmdList()
	case "use":
		err = cmdUse(args)
	case "current":
		err = cmdCurrent()
	case "delete", "rm":
		err = cmdDelete(args)
	case "launch":
		err = cmdLaunch(args)
	case "init":
		err = cmdInit()
	case "version", "--version", "-v":
		fmt.Printf("claude-rig %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`claude-rig - Manage Claude Code configuration profiles

Usage:
  claude-rig <command> [arguments]

Commands:
  create <name>           Create a new profile (optionally from current config)
  list                    List all profiles and show active one
  use <name>              Set the active profile (for shell alias usage)
  current                 Show the currently active profile
  delete <name>           Delete a profile
  launch <name> [args]    Launch Claude Code with a specific profile
  init                    Initialize profile system (run once)
  version                 Show version
  help                    Show this help

Examples:
  claude-rig init                    # Set up the profile system
  claude-rig create minimal          # Create a new empty profile
  claude-rig create webappdev        # Create another profile
  claude-rig launch webappdev        # Launch Claude Code with this profile
  claude-rig list                    # See all profiles

Shell integration (add to ~/.bashrc or ~/.zshrc):
  alias claude-minimal='claude-rig launch minimal'
  alias claude-webdev='claude-rig launch webappdev'

Or use a wrapper function for --rig flag:
  claude() {
    for arg in "$@"; do
      if [[ "$arg" == --rig=* ]]; then
        claude-rig launch "${arg#--rig=}" "${@//$arg/}"
        return
      fi
    done
    command claude "$@"
  }
`)
}
