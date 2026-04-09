package main

import (
	"embed"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
)

//go:embed all:bundled
var bundledPlugin embed.FS

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
	case "delete", "rm":
		err = cmdDelete(args)
	case "rename", "mv":
		err = cmdRename(args)
	case "sync":
		err = cmdSync(args)
	case "update":
		err = cmdUpdate(args)
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
	case "update-plugins":
		err = cmdUpdatePlugins(args)
	case "isolate":
		err = cmdIsolate(args)
	case "share":
		err = cmdShare(args)
	case "isolation":
		err = cmdIsolation(args)
	case "inherit":
		err = cmdInherit(args)
	case "uninherit":
		err = cmdUninherit(args)
	case "status":
		err = cmdStatus(args)
	case "diff":
		err = cmdDiff(args)
	case "export":
		err = cmdExport(args)
	case "import":
		err = cmdImport(args)
	case "plugin":
		err = cmdPlugin(args)
	case "settings":
		err = cmdSettings(args)
	case "blueprint":
		err = cmdBlueprint(args)
	case "doctor":
		err = cmdDoctor()
	case "versions":
		err = cmdVersions()
	case "pin":
		err = cmdPin(args)
	case "unpin":
		err = cmdUnpin(args)
	case "version", "--version", "-v":
		fmt.Printf("claude-rig %s\n", getVersion())
	case "help", "--help", "-h":
		printUsage()
	default:
		// Flag-like first arg (e.g., --resume) — try RC-based launch, forwarding all args to claude
		if strings.HasPrefix(cmd, "-") {
			rig, _, err := findRC()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if rig != "" {
				err = cmdLaunch(os.Args[1:])
				break
			}
			fmt.Fprintf(os.Stderr, "Unknown flag %q and no .claude-rig file found\n\n", cmd)
			printUsage()
			os.Exit(1)
		}
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
  create <name>           Create a new rig (isolates per-rig data by default)
    --link-auth            Symlink auth from existing ~/.claude/ (skip onboarding)
    --no-isolate-defaults  Don't isolate default items (share everything)
    --isolate-all          Isolate all isolatable items
    --isolate <items,...>  Isolate additional items (on top of defaults)
  clone <src|default> <dest>  Clone a rig or ~/.claude/ config (--link-auth, --no-isolate-defaults)
  link-auth <name>        Link rig to shared auth (--from <rig> for cross-rig)
  unlink-auth <name>      Remove shared auth, rig will use its own
  list                    List all rigs (* = running sessions)
  delete <name>           Delete a rig
  rename <old> <new>      Rename a rig
  sync [rig]              Sync symlinks, inherited items, plugins, MCP, settings (all rigs if none given)
    --no-plugins           Skip plugin sync
    --no-mcp               Skip MCP server sync
    --no-inherit           Skip inherited items sync
    --no-settings          Skip default settings sync
    --from <rig>           Use another rig as plugin/MCP source instead of global
  update                  Update Claude Code (forwards to claude update)
  versions                List available Claude Code versions on disk
  pin <version>           Pin rig to a specific Claude Code version (--rig <name>)
  unpin                   Remove version pin, use system default (--rig <name>)
  launch [name] [args]    Launch Claude Code with a specific rig
  rc [name]               Show or set .claude-rig file for current directory
  set-args [name] <args>  Set default launch args (global if no name, per-rig if given)
  show-args [name]        Show default launch args
  isolate <rig> <items>   Isolate items per rig (no sharing via symlinks)
  share <rig> <items>     Reverse isolation (delete local, recreate symlink)
  isolation [rig]         Show isolation status for one or all rigs
    --details              Show individual entries with source info
    --skills|--plugins|... Filter to specific categories
  inherit <items> [rig]   Inherit global skills/agents/hooks/commands from ~/.claude/
  uninherit <items> [rig] Stop inheriting (remove global symlinks)
  diff <rig1> <rig2>      Compare two rigs (auth, settings, plugins, MCP, etc.)
  export <rig> [file]     Export rig to .tar.gz (--include-auth, --include-data)
  import <file> <name>    Import rig from archive (--link-auth)
  status [rig]            Show rig status (disk, sessions, last used)
  plugin <subcommand>     Run claude plugin commands on active rig (--rig <name>)
  update-plugins [rigs]   Update marketplace plugins (all rigs if none given)
  settings <subcommand>   Manage default settings applied to all rigs
    set <key> <value>      Set a default and apply to all rigs immediately
    remove <key>           Remove a default and strip from all rigs
    list                   Show current default settings
    override <key> <value> [--rig <name>]  Pin a per-rig value (survives sync)
  blueprint <subcommand>  Shareable rig specifications
    create <name>          Create blueprint from active rig (--from <rig>)
    apply <source>         Create rig from blueprint (--as <name>, --link-auth, --skip-plugins)
    inspect <source>       Preview blueprint contents
    list                   List local blueprints
    pack <name> [file]     Pack blueprint into .tar.gz for sharing
  doctor                  Check rigs for broken symlinks and missing items
  init                    Initialize rig system (run once)
  version                 Show version
  help                    Show this help

Examples:
  claude-rig init                    # Set up the rig system
  claude-rig create minimal          # Create rig (per-rig data isolated by default)
  claude-rig create webappdev --link-auth  # Create rig, reuse existing auth
  claude-rig create shared --no-isolate-defaults  # Create rig, share everything
  claude-rig create myrig --inherit-all   # Create rig, inherit global skills/agents/hooks/commands
  claude-rig launch webappdev        # Launch Claude Code with this rig
  claude-rig list                    # See all rigs
  claude-rig rc minimal              # Bind current directory to a rig
  claude-rig                         # Auto-launch from .claude-rig file
  claude-rig --resume <id>           # Auto-launch and forward flags to claude

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
