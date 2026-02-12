package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

// version is set at build time via -ldflags "-X main.version=..."
// Falls back to Go module version info when installed via `go install`.
var version = ""

func getVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func main() {
	if len(os.Args) < 2 {
		// No subcommand — try RC file
		rig, _, err := findRC()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if rig == "" {
			printUsage()
			os.Exit(1)
		}
		if err := cmdLaunch(nil); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
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
	case "clone":
		err = cmdClone(args)
	case "link-auth":
		err = cmdLinkAuth(args)
	case "unlink-auth":
		err = cmdUnlinkAuth(args)
	case "init":
		err = cmdInit()
	case "set-args":
		err = cmdSetArgs(args)
	case "show-args":
		err = cmdShowArgs(args)
	case "rc":
		err = cmdRC(args)
	case "doctor":
		err = cmdDoctor()
	case "version", "--version", "-v":
		fmt.Printf("claude-rig %s\n", getVersion())
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
	fmt.Print(`claude-rig - Manage multiple Claude Code configuration rigs

Usage:
  claude-rig <command> [arguments]

Commands:
  create <name>           Create a new rig
    --link-auth            Symlink auth from existing ~/.claude/ (skip onboarding)
  clone <src> <dest>      Clone a rig (--link-auth to use shared auth)
  link-auth <name>        Link rig to shared auth (--from <rig> for cross-rig)
  unlink-auth <name>      Remove shared auth, rig will use its own
  list                    List all rigs and show active one
  use <name>              Set the active rig (for shell alias usage)
  current                 Show the currently active rig
  delete <name>           Delete a rig
  launch [name] [args]    Launch Claude Code with a specific rig
  rc [name]               Show or set .claude-rig file for current directory
  set-args [name] <args>  Set default launch args (global if no name, per-rig if given)
  show-args [name]        Show default launch args
  doctor                  Check rigs for broken symlinks and missing items
  init                    Initialize rig system (run once)
  version                 Show version
  help                    Show this help

Examples:
  claude-rig init                    # Set up the rig system
  claude-rig create minimal          # Create a new empty rig
  claude-rig create webappdev --link-auth  # Create rig, reuse existing auth
  claude-rig launch webappdev        # Launch Claude Code with this rig
  claude-rig list                    # See all rigs
  claude-rig rc minimal              # Bind current directory to a rig
  claude-rig                         # Auto-launch from .claude-rig file

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
