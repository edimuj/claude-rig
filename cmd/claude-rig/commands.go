package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// cmdSetArgs sets default launch arguments globally or per-rig.
func cmdSetArgs(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig set-args [rig] <flags...>\n  Global: claude-rig set-args -- --dangerously-skip-permissions\n  Rig: claude-rig set-args minimal -- --dangerously-skip-permissions")
	}

	// Check if first arg is a rig name or a flag
	dir, rigName, flagArgs := resolveArgsTarget(args)
	if dir == "" {
		return fmt.Errorf("could not determine target directory")
	}

	argsStr := strings.Join(flagArgs, " ")
	file := filepath.Join(dir, "default-args")

	if len(flagArgs) == 0 {
		// Clear args
		os.Remove(file)
		if rigName != "" {
			fmt.Printf("Cleared default args for rig %q\n", rigName)
		} else {
			fmt.Println("Cleared global default args")
		}
		return nil
	}

	if err := os.WriteFile(file, []byte(argsStr+"\n"), 0644); err != nil {
		return fmt.Errorf("writing default-args: %w", err)
	}

	if rigName != "" {
		fmt.Printf("Rig %q default args: %s\n", rigName, argsStr)
	} else {
		fmt.Printf("Global default args: %s\n", argsStr)
	}
	return nil
}

// cmdShowArgs shows default launch arguments.
func cmdShowArgs(args []string) error {
	rig, _ := rigHome()

	// Show global
	globalArgs := loadLaunchArgs(rig)
	if globalArgs != nil {
		fmt.Printf("Global:  %s\n", strings.Join(globalArgs, " "))
	} else {
		fmt.Println("Global:  (none)")
	}

	// If rig specified, show just that one
	if len(args) > 0 {
		dir, err := rigDir(args[0])
		if err != nil {
			return err
		}
		rigArgs := loadLaunchArgs(dir)
		if rigArgs != nil {
			fmt.Printf("Rig %q: %s\n", args[0], strings.Join(rigArgs, " "))
		} else {
			fmt.Printf("Rig %q: (inherits global)\n", args[0])
		}
		return nil
	}

	// Show all rigs
	root, _ := rigsRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir, _ := rigDir(e.Name())
		rigArgs := loadLaunchArgs(dir)
		if rigArgs != nil {
			fmt.Printf("  %-20s %s\n", e.Name()+":", strings.Join(rigArgs, " "))
		}
	}
	return nil
}

// resolveArgsTarget figures out if the user is setting global or per-rig args.
func resolveArgsTarget(args []string) (dir string, rigName string, flagArgs []string) {
	// If first arg starts with - or is --, it's global
	if args[0] == "--" || strings.HasPrefix(args[0], "-") {
		rig, _ := rigHome()
		// Skip leading -- separator if present
		flags := args
		if args[0] == "--" {
			flags = args[1:]
		}
		return rig, "", flags
	}

	// First arg is a rig name
	rigName = args[0]
	d, err := rigDir(rigName)
	if err != nil {
		return "", "", nil
	}
	if _, err := os.Stat(d); os.IsNotExist(err) {
		return "", "", nil
	}

	flags := args[1:]
	// Skip -- separator if present
	if len(flags) > 0 && flags[0] == "--" {
		flags = flags[1:]
	}
	return d, rigName, flags
}

// cmdDoctor checks the health of the rig system.
func cmdDoctor() error {
	issues := 0
	warn := func(format string, args ...any) {
		issues++
		fmt.Printf("  ✗ "+format+"\n", args...)
	}
	ok := func(format string, args ...any) {
		fmt.Printf("  ✓ "+format+"\n", args...)
	}

	// Check ~/.claude/ exists
	fmt.Println("System:")
	home, err := claudeHome()
	if err != nil {
		return err
	}
	if _, err := os.Stat(home); os.IsNotExist(err) {
		warn("Claude config not found: %s", home)
	} else {
		ok("Claude config: %s", home)
	}

	// Check ~/.claude-rig/ exists
	rig, err := rigHome()
	if err != nil {
		return err
	}
	if _, err := os.Stat(rig); os.IsNotExist(err) {
		warn("Not initialized: %s (run: claude-rig init)", rig)
		return nil
	}
	ok("Rig home: %s", rig)

	// Check active rig
	active := getActiveRig()
	if active != "" {
		dir, _ := rigDir(active)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			warn("Active rig %q does not exist", active)
		} else {
			ok("Active rig: %s", active)
		}
	}

	// Check each rig
	root, _ := rigsRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		dir, _ := rigDir(name)

		fmt.Printf("\nRig %q:\n", name)

		// Check rig-specific items exist
		for _, item := range rigSpecificItems {
			path := filepath.Join(dir, item)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				warn("Missing: %s", item)
			}
		}

		// Walk all entries and check for broken symlinks
		dirEntries, err := os.ReadDir(dir)
		if err != nil {
			warn("Cannot read rig directory: %v", err)
			continue
		}

		brokenLinks := 0
		for _, de := range dirEntries {
			path := filepath.Join(dir, de.Name())
			info, err := os.Lstat(path)
			if err != nil {
				continue
			}
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			target, err := os.Readlink(path)
			if err != nil {
				warn("Unreadable symlink: %s", de.Name())
				brokenLinks++
				continue
			}
			if _, err := os.Stat(target); os.IsNotExist(err) {
				warn("Broken symlink: %s → %s", de.Name(), target)
				brokenLinks++
			}
		}

		if brokenLinks == 0 {
			ok("All symlinks valid")
		}
	}

	fmt.Println()
	if issues == 0 {
		fmt.Println("No issues found.")
	} else {
		fmt.Printf("Found %d issue(s).\n", issues)
	}
	return nil
}

// shellWrapperTemplate is the function added to shell rc files for --rig support.
// %s is replaced with any extra default flags from an existing alias.
const shellWrapperTemplate = `
# claude-rig: --rig flag support
unalias claude 2>/dev/null
claude() {
  for arg in "$@"; do
    if [[ "$arg" == --rig=* ]]; then
      claude-rig launch "${arg#--rig=}" "${@//$arg/}"
      return
    fi
  done
  command claude%s "$@"
}
`

// cmdInit sets up the rig system directory structure and shell integration.
func cmdInit() error {
	root, err := rigsRoot()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(root, 0755); err != nil {
		return fmt.Errorf("creating rigs directory: %w", err)
	}

	rig, _ := rigHome()
	fmt.Printf("Initialized rig system at %s\n", rig)

	// Shell integration
	rcFile := detectShellRC()
	if rcFile == "" {
		fmt.Println("Could not detect shell rc file — add the claude wrapper function manually.")
		fmt.Println("See: claude-rig help")
	} else if hasShellIntegration(rcFile) {
		fmt.Printf("Shell integration already present in %s\n", rcFile)
	} else {
		fmt.Printf("Add claude --rig wrapper to %s? [Y/n] ", rcFile)
		var confirm string
		fmt.Scanln(&confirm)
		if confirm == "" || strings.ToLower(confirm) == "y" {
			if err := installShellIntegration(rcFile); err != nil {
				return fmt.Errorf("adding shell integration: %w", err)
			}
			fmt.Printf("Added to %s — restart your shell or run: source %s\n", rcFile, rcFile)
		}
	}

	fmt.Println("\nNext: claude-rig create <name>")
	return nil
}

func detectShellRC() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	shell := os.Getenv("SHELL")
	switch {
	case strings.HasSuffix(shell, "/zsh"):
		return filepath.Join(home, ".zshrc")
	case strings.HasSuffix(shell, "/bash"):
		// Prefer .bashrc, fall back to .bash_profile on macOS
		bashrc := filepath.Join(home, ".bashrc")
		if _, err := os.Stat(bashrc); err == nil {
			return bashrc
		}
		profile := filepath.Join(home, ".bash_profile")
		if _, err := os.Stat(profile); err == nil {
			return profile
		}
		return bashrc
	default:
		return ""
	}
}

func hasShellIntegration(rcFile string) bool {
	data, err := os.ReadFile(rcFile)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "claude-rig")
}

func installShellIntegration(rcFile string) error {
	data, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Detect existing claude alias and extract its flags
	extraFlags := ""
	lines := strings.Split(string(data), "\n")
	var newLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if aliasFlags := parseClaudeAlias(trimmed); aliasFlags != "" {
			extraFlags = " " + aliasFlags
			fmt.Printf("Found existing alias: %s\n", trimmed)
			fmt.Printf("Folding flags into wrapper: %s\n", aliasFlags)
			// Comment out the old alias
			newLines = append(newLines, "# "+line+" # replaced by claude-rig wrapper")
		} else {
			newLines = append(newLines, line)
		}
	}

	wrapper := fmt.Sprintf(shellWrapperTemplate, extraFlags)
	output := strings.Join(newLines, "\n") + wrapper

	return os.WriteFile(rcFile, []byte(output), 0644)
}

// parseClaudeAlias extracts flags from an alias like: alias claude='claude --flag1 --flag2'
func parseClaudeAlias(line string) string {
	// Match: alias claude='claude ...' or alias claude="claude ..."
	for _, prefix := range []string{
		`alias claude='claude `,
		`alias claude="claude `,
	} {
		if strings.HasPrefix(line, prefix) {
			// Strip prefix and trailing quote
			rest := line[len(prefix):]
			if len(rest) > 0 {
				rest = rest[:len(rest)-1] // remove closing ' or "
			}
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// cmdCreate creates a new rig directory with rig-specific items
// and symlinks to shared items in ~/.claude/.
func cmdCreate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig create <name> [--link-auth]")
	}

	var name string
	var linkAuth bool
	for _, a := range args {
		if a == "--link-auth" {
			linkAuth = true
		} else if name == "" {
			name = a
		}
	}
	if name == "" {
		return fmt.Errorf("usage: claude-rig create <name> [--link-auth]")
	}

	if err := validateRigName(name); err != nil {
		return err
	}

	dir, err := rigDir(name)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("rig %q already exists", name)
	}

	root, err := rigsRoot()
	if err != nil {
		return err
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return fmt.Errorf("rig system not initialized — run: claude-rig init")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating rig directory: %w", err)
	}

	// Create rig-specific items
	for _, item := range rigSpecificItems {
		path := filepath.Join(dir, item)
		switch {
		case strings.HasSuffix(item, ".json"):
			if err := os.WriteFile(path, []byte("{}\n"), 0644); err != nil {
				return fmt.Errorf("creating %s: %w", item, err)
			}
		case strings.HasSuffix(item, ".md"):
			if err := os.WriteFile(path, []byte(""), 0644); err != nil {
				return fmt.Errorf("creating %s: %w", item, err)
			}
		default:
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("creating %s/: %w", item, err)
			}
		}
	}

	// Symlink shared items from ~/.claude/
	if err := syncSharedSymlinks(dir); err != nil {
		return fmt.Errorf("creating shared symlinks: %w", err)
	}

	// Optionally symlink auth files from ~/.claude/
	if linkAuth {
		if err := linkAuthFiles(dir); err != nil {
			return fmt.Errorf("linking auth files: %w", err)
		}
		fmt.Println("Linked auth from existing Claude config")
	}

	fmt.Printf("Created rig %q at %s\n", name, dir)
	fmt.Printf("Launch with: claude-rig launch %s\n", name)
	return nil
}

// cmdClone duplicates a rig. Symlinks are recreated, real files are copied.
// Use "default" as source to clone from ~/.claude/.
func cmdClone(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: claude-rig clone <source|default> <dest> [--link-auth]")
	}

	var positional []string
	var linkAuth bool
	for _, a := range args {
		if a == "--link-auth" {
			linkAuth = true
		} else {
			positional = append(positional, a)
		}
	}
	if len(positional) < 2 {
		return fmt.Errorf("usage: claude-rig clone <source|default> <dest> [--link-auth]")
	}
	srcName, destName := positional[0], positional[1]

	if err := validateRigName(destName); err != nil {
		return err
	}

	destDir, err := rigDir(destName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("rig %q already exists", destName)
	}

	if srcName == "default" {
		if err := cloneFromDefault(destDir); err != nil {
			os.RemoveAll(destDir)
			return fmt.Errorf("cloning from default: %w", err)
		}
	} else {
		srcDir, err := rigDir(srcName)
		if err != nil {
			return err
		}
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			return fmt.Errorf("rig %q does not exist", srcName)
		}
		if err := cloneDir(srcDir, destDir); err != nil {
			os.RemoveAll(destDir)
			return fmt.Errorf("cloning rig: %w", err)
		}
	}

	if linkAuth {
		if err := linkAuthFiles(destDir); err != nil {
			return fmt.Errorf("linking auth files: %w", err)
		}
		fmt.Println("Linked auth from existing Claude config")
	}

	fmt.Printf("Cloned %q → %q\n", srcName, destName)
	fmt.Printf("Launch with: claude-rig launch %s\n", destName)
	return nil
}

// cloneFromDefault creates a new rig by copying rig-specific items from ~/.claude/
// and symlinking everything else.
func cloneFromDefault(destDir string) error {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	srcDir := filepath.Join(userHome, ".claude")
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("~/.claude/ does not exist")
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Copy rig-specific items from ~/.claude/
	for _, item := range rigSpecificItems {
		srcPath := filepath.Join(srcDir, item)
		destPath := filepath.Join(destDir, item)

		info, err := os.Stat(srcPath)
		if os.IsNotExist(err) {
			// Item doesn't exist in default — create empty
			switch {
			case strings.HasSuffix(item, ".json"):
				os.WriteFile(destPath, []byte("{}\n"), 0644)
			case strings.HasSuffix(item, ".md"):
				os.WriteFile(destPath, []byte(""), 0644)
			default:
				os.MkdirAll(destPath, 0755)
			}
			continue
		}
		if err != nil {
			return err
		}

		if info.IsDir() {
			if err := cloneDir(srcPath, destPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(destPath, data, info.Mode()); err != nil {
				return err
			}
		}
	}

	// Symlink shared items from ~/.claude/
	return syncSharedSymlinks(destDir)
}

// cloneDir recursively copies a directory. Symlinks are recreated, files are copied.
func cloneDir(src, dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		destPath := filepath.Join(dest, e.Name())

		info, err := os.Lstat(srcPath)
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.Symlink(target, destPath); err != nil {
				return err
			}
			continue
		}

		if info.IsDir() {
			if err := cloneDir(srcPath, destPath); err != nil {
				return err
			}
			continue
		}

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(destPath, data, info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

// cmdLinkAuth links auth files into a rig from ~/.claude/ or another rig.
func cmdLinkAuth(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig link-auth <name> [--from <rig>]")
	}

	var name, fromRig string
	for i := 0; i < len(args); i++ {
		if args[i] == "--from" && i+1 < len(args) {
			fromRig = args[i+1]
			i++
		} else if name == "" {
			name = args[i]
		}
	}
	if name == "" {
		return fmt.Errorf("usage: claude-rig link-auth <name> [--from <rig>]")
	}

	dir, err := rigDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("rig %q does not exist", name)
	}

	// Resolve auth source directories
	var authHome, authUserHome string
	if fromRig != "" {
		if fromRig == name {
			return fmt.Errorf("cannot link auth from a rig to itself")
		}
		fromDir, err := rigDir(fromRig)
		if err != nil {
			return err
		}
		if _, err := os.Stat(fromDir); os.IsNotExist(err) {
			return fmt.Errorf("source rig %q does not exist", fromRig)
		}
		// All auth items live inside the rig dir
		authHome = fromDir
		authUserHome = fromDir
	} else {
		authHome, err = claudeHome()
		if err != nil {
			return err
		}
		authUserHome, err = os.UserHomeDir()
		if err != nil {
			return err
		}
	}

	for _, item := range authItems {
		// .claude.json lives in ~ when linking from default, but inside rig dir when linking from rig
		sourceDir := authHome
		if item == ".claude.json" && fromRig == "" {
			sourceDir = authUserHome
		}

		target := filepath.Join(sourceDir, item)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			continue
		}

		linkPath := filepath.Join(dir, item)
		info, err := os.Lstat(linkPath)
		if err == nil {
			// Already a symlink pointing to the right place — skip
			if info.Mode()&os.ModeSymlink != 0 {
				dest, _ := os.Readlink(linkPath)
				if dest == target {
					continue
				}
			}
			// Real file or symlink to somewhere else — warn
			fmt.Printf("Rig %q already has %s. Replace with shared auth? [y/N] ", name, item)
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Printf("Skipped %s\n", item)
				continue
			}
			os.RemoveAll(linkPath)
		}

		if err := os.Symlink(target, linkPath); err != nil {
			return fmt.Errorf("symlinking %s: %w", item, err)
		}
		fmt.Printf("Linked %s\n", item)
	}

	if fromRig != "" {
		fmt.Printf("Rig %q now uses auth from %q\n", name, fromRig)
	} else {
		fmt.Printf("Rig %q now uses shared auth\n", name)
	}
	return nil
}

// cmdUnlinkAuth removes shared auth from a rig so it gets fresh onboarding.
func cmdUnlinkAuth(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig unlink-auth <name>")
	}
	name := args[0]

	dir, err := rigDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("rig %q does not exist", name)
	}

	removed := 0
	for _, item := range authItems {
		linkPath := filepath.Join(dir, item)
		info, err := os.Lstat(linkPath)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			os.Remove(linkPath)
			fmt.Printf("Removed symlink: %s\n", item)
			removed++
		} else if item == ".claude.json" {
			// Real file — remove it so Claude starts fresh
			os.Remove(linkPath)
			fmt.Printf("Removed: %s\n", item)
			removed++
		}
	}

	// Clean up backup files that cache account data
	cleanAuthBackups(dir)

	if removed == 0 {
		fmt.Printf("Rig %q has no shared auth to remove\n", name)
	} else {
		fmt.Printf("Rig %q will get fresh onboarding on next launch\n", name)
	}
	return nil
}

// cmdList shows all rigs with auth status and item counts.
func cmdList() error {
	root, err := rigsRoot()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No rigs found. Run: claude-rig init")
			return nil
		}
		return err
	}

	active := getActiveRig()

	// First pass: collect names to resolve auth targets to rig names
	rigDirs := map[string]string{} // dir path → rig name
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir, _ := rigDir(e.Name())
		rigDirs[dir] = e.Name()
	}

	found := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		found = true
		name := e.Name()
		dir, _ := rigDir(name)

		marker := "  "
		if name == active {
			marker = "* "
		}

		auth := rigAuthStatus(dir, rigDirs)
		skills := countDirEntries(filepath.Join(dir, "skills"))
		plugins := countDirEntries(filepath.Join(dir, "plugins"))
		mcp := countMCPServers(filepath.Join(dir, "mcp.json"))

		fmt.Printf("%s%-20s auth: %-20s skills: %d  plugins: %d  mcp: %d\n",
			marker, name, auth, skills, plugins, mcp)
	}

	if !found {
		fmt.Println("No rigs found. Run: claude-rig create <name>")
	}
	return nil
}

// rigAuthStatus returns the auth status for a rig.
func rigAuthStatus(dir string, rigDirs map[string]string) string {
	credPath := filepath.Join(dir, ".credentials.json")
	info, err := os.Lstat(credPath)
	if err != nil {
		return "none"
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "own"
	}
	target, err := os.Readlink(credPath)
	if err != nil {
		return "linked"
	}
	// Resolve target to a friendly name
	targetDir := filepath.Dir(target)
	if name, ok := rigDirs[targetDir]; ok {
		return "linked (" + name + ")"
	}
	home, _ := claudeHome()
	if targetDir == home {
		return "linked (~/.claude)"
	}
	return "linked"
}

func countDirEntries(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	return len(entries)
}

func countMCPServers(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(data, &cfg) != nil {
		return 0
	}
	return len(cfg.MCPServers)
}

// cmdUse sets the active rig (for shell alias workflows).
func cmdUse(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig use <name>")
	}
	name := args[0]

	dir, err := rigDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("rig %q does not exist", name)
	}

	file, err := activeRigFile()
	if err != nil {
		return err
	}
	if err := os.WriteFile(file, []byte(name), 0644); err != nil {
		return fmt.Errorf("writing active rig: %w", err)
	}

	fmt.Printf("Active rig: %s\n", name)
	fmt.Printf("Launch with: CLAUDE_CONFIG_DIR=%s claude\n", dir)
	return nil
}

// cmdCurrent shows the active rig.
func cmdCurrent() error {
	active := getActiveRig()
	if active == "" {
		fmt.Println("No active rig set. Use: claude-rig use <name>")
	} else {
		fmt.Println(active)
	}
	return nil
}

// cmdDelete removes a rig directory.
func cmdDelete(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig delete <name>")
	}
	name := args[0]

	dir, err := rigDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("rig %q does not exist", name)
	}

	fmt.Printf("Delete rig %q and all its settings/skills/plugins? [y/N] ", name)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled.")
		return nil
	}

	// Only remove real files, not symlink targets
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing rig: %w", err)
	}

	// Clear active if this was it
	if getActiveRig() == name {
		file, _ := activeRigFile()
		os.Remove(file)
	}

	fmt.Printf("Deleted rig %q\n", name)
	return nil
}

// cmdLaunch starts Claude Code with the specified rig's config dir.
func cmdLaunch(args []string) error {
	var name string
	var extraArgs []string
	var rcPath string

	if len(args) >= 1 && !strings.HasPrefix(args[0], "-") {
		name = args[0]
		extraArgs = args[1:]
	} else {
		// No rig name — try RC file
		rig, path, err := findRC()
		if err != nil {
			return err
		}
		if rig == "" {
			return fmt.Errorf("usage: claude-rig launch <name> [claude-args...]")
		}
		name = rig
		rcPath = path
		extraArgs = args // all args are claude args
	}

	dir, err := rigDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("rig %q does not exist", name)
	}

	if rcPath != "" {
		fmt.Fprintf(os.Stderr, "Using rig %q from %s\n", name, rcPath)
	}

	// Refresh shared symlinks in case new files appeared in ~/.claude/
	if err := syncSharedSymlinks(dir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not sync shared symlinks: %v\n", err)
	}

	// Set active rig marker
	file, _ := activeRigFile()
	os.WriteFile(file, []byte(name), 0644)

	binary := claudeCodeBinary()
	binPath, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("claude binary not found: %w (set CLAUDE_BINARY to override)", err)
	}

	// Load default args: per-rig takes precedence, then global
	defaultArgs := loadLaunchArgs(dir)
	if defaultArgs == nil {
		rig, _ := rigHome()
		defaultArgs = loadLaunchArgs(rig)
	}

	// Replace this process with claude, passing the config dir via env
	env := os.Environ()
	env = setEnv(env, "CLAUDE_CONFIG_DIR", dir)

	// Migrate legacy symlinked CLAUDE.md to real file
	claudeMD := filepath.Join(dir, "CLAUDE.md")
	if info, err := os.Lstat(claudeMD); err == nil && info.Mode()&os.ModeSymlink != 0 {
		os.Remove(claudeMD)
		os.WriteFile(claudeMD, []byte(""), 0644)
	}

	// Always load global ~/.claude/ CLAUDE.md via --add-dir
	userHome, _ := os.UserHomeDir()
	if userHome != "" {
		canonicalClaudeHome := filepath.Join(userHome, ".claude")
		env = setEnv(env, "CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD", "1")
		extraArgs = append(extraArgs, "--add-dir", canonicalClaudeHome)
	}

	execArgs := append([]string{binary}, defaultArgs...)
	execArgs = append(execArgs, extraArgs...)
	return syscall.Exec(binPath, execArgs, env)
}

// findRC walks from cwd up to $HOME looking for a .claude-rig file.
func findRC() (rig string, path string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", "", err
	}

	for {
		candidate := filepath.Join(dir, ".claude-rig")
		if _, err := os.Stat(candidate); err == nil {
			rig, err := parseRC(candidate)
			if err != nil {
				return "", candidate, err
			}
			return rig, candidate, nil
		}

		if dir == home {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // filesystem root
		}
		dir = parent
	}
	return "", "", nil
}

// parseRC reads a .claude-rig file and returns the rig= value.
func parseRC(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "rig=") {
			val := strings.TrimSpace(line[4:])
			if val == "" {
				return "", fmt.Errorf("empty rig= value in %s", path)
			}
			return val, nil
		}
	}
	return "", fmt.Errorf("no rig= key found in %s", path)
}

// cmdRC shows or creates a .claude-rig file in the current directory.
func cmdRC(args []string) error {
	if len(args) == 0 {
		rig, path, err := findRC()
		if err != nil {
			return err
		}
		if rig == "" {
			fmt.Println("No .claude-rig file found")
			return nil
		}
		fmt.Printf("Rig %q from %s\n", rig, path)
		return nil
	}

	name := args[0]
	dir, err := rigDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("rig %q does not exist", name)
	}

	if err := os.WriteFile(".claude-rig", []byte("rig="+name+"\n"), 0644); err != nil {
		return fmt.Errorf("writing .claude-rig: %w", err)
	}
	fmt.Printf("Created .claude-rig with rig %q\n", name)
	return nil
}

// --- helpers ---

func linkAuthFiles(rigDir string) error {
	home, err := claudeHome()
	if err != nil {
		return err
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	for _, item := range authItems {
		// .claude.json lives in ~ rather than ~/.claude/
		sourceDir := home
		if item == ".claude.json" {
			sourceDir = userHome
		}

		target := filepath.Join(sourceDir, item)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			continue
		}
		linkPath := filepath.Join(rigDir, item)
		if info, err := os.Lstat(linkPath); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				dest, _ := os.Readlink(linkPath)
				if dest == target {
					continue // already linked correctly
				}
			}
			// Remove existing file/symlink to replace with correct link
			os.RemoveAll(linkPath)
		}
		if err := os.Symlink(target, linkPath); err != nil {
			return fmt.Errorf("symlinking %s: %w", item, err)
		}
	}

	// Clean up .claude.json backups that may hold stale auth data
	cleanAuthBackups(rigDir)
	return nil
}

// loadLaunchArgs reads default launch arguments from a directory's .launch-args file.
func loadLaunchArgs(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, "default-args"))
	if err != nil {
		return nil
	}
	line := strings.TrimSpace(string(data))
	if line == "" {
		return nil
	}
	return strings.Fields(line)
}

func cleanAuthBackups(rigDir string) {
	entries, err := os.ReadDir(rigDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".claude.json.backup.") {
			os.Remove(filepath.Join(rigDir, e.Name()))
		}
	}
}

func syncSharedSymlinks(rigDir string) error {
	home, err := claudeHome()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(home)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // ~/.claude/ doesn't exist yet, nothing to link
		}
		return err
	}

	for _, e := range entries {
		name := e.Name()

		// Skip rig-specific items and hidden files
		if isRigSpecific(name) || strings.HasPrefix(name, ".") {
			continue
		}

		linkPath := filepath.Join(rigDir, name)
		target := filepath.Join(home, name)

		// Skip if something already exists at this path
		if _, err := os.Lstat(linkPath); err == nil {
			continue
		}

		if err := os.Symlink(target, linkPath); err != nil {
			return fmt.Errorf("symlinking %s: %w", name, err)
		}
	}
	return nil
}

func getActiveRig() string {
	file, err := activeRigFile()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func validateRigName(name string) error {
	if name == "" {
		return fmt.Errorf("rig name cannot be empty")
	}
	if strings.ContainsAny(name, "/\\. ") {
		return fmt.Errorf("rig name cannot contain slashes, dots, or spaces")
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("rig name cannot start with a dash")
	}
	return nil
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
