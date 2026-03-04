package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// rigConfig holds per-rig configuration stored in rig.json.
type rigConfig struct {
	Isolate []string `json:"isolate,omitempty"`
	Inherit []string `json:"inherit,omitempty"`
}

func loadRigConfig(rigDir string) rigConfig {
	data, err := os.ReadFile(rigConfigPath(rigDir))
	if err != nil {
		return rigConfig{}
	}
	var cfg rigConfig
	json.Unmarshal(data, &cfg)
	return cfg
}

func saveRigConfig(rigDir string, cfg rigConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rigConfigPath(rigDir), append(data, '\n'), 0644)
}

func (c rigConfig) isIsolated(name string) bool {
	for _, item := range c.Isolate {
		if item == name {
			return true
		}
	}
	return false
}

func (c rigConfig) isInherited(name string) bool {
	for _, item := range c.Inherit {
		if item == name {
			return true
		}
	}
	return false
}

// cmdIsolate marks items as isolated for a rig.
func cmdIsolate(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: claude-rig isolate <rig> <items...>\nIsolatable: %s", strings.Join(isolatableItems, ", "))
	}

	name := args[0]
	dir, err := rigDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("rig %q does not exist", name)
	}

	home, err := claudeHome()
	if err != nil {
		return err
	}

	cfg := loadRigConfig(dir)
	items := args[1:]

	for _, item := range items {
		if !isIsolatable(item) {
			return fmt.Errorf("%q is not isolatable. Valid items: %s", item, strings.Join(isolatableItems, ", "))
		}
		if cfg.isIsolated(item) {
			fmt.Printf("  %s — already isolated\n", item)
			continue
		}

		linkPath := filepath.Join(dir, item)

		// Remove existing symlink if present
		if info, err := os.Lstat(linkPath); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				os.Remove(linkPath)
			} else {
				fmt.Printf("  %s — already a local file/dir, marking isolated\n", item)
				cfg.Isolate = append(cfg.Isolate, item)
				continue
			}
		}

		// Create empty local file or directory based on what it is in ~/.claude/
		srcPath := filepath.Join(home, item)
		srcInfo, err := os.Stat(srcPath)
		if err == nil && srcInfo.IsDir() {
			os.MkdirAll(linkPath, 0755)
		} else if strings.HasSuffix(item, ".json") || strings.HasSuffix(item, ".jsonl") {
			os.WriteFile(linkPath, []byte(""), 0644)
		} else {
			// Assume directory for anything without a file extension
			if strings.Contains(item, ".") {
				os.WriteFile(linkPath, []byte(""), 0644)
			} else {
				os.MkdirAll(linkPath, 0755)
			}
		}

		cfg.Isolate = append(cfg.Isolate, item)
		fmt.Printf("  %s — isolated\n", item)
	}

	if err := saveRigConfig(dir, cfg); err != nil {
		return fmt.Errorf("saving rig config: %w", err)
	}

	fmt.Printf("Rig %q: %d item(s) isolated\n", name, len(cfg.Isolate))
	return nil
}

// cmdShare reverses isolation — deletes local copy and recreates symlink.
func cmdShare(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: claude-rig share <rig> <items...>")
	}

	name := args[0]
	dir, err := rigDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("rig %q does not exist", name)
	}

	home, err := claudeHome()
	if err != nil {
		return err
	}

	cfg := loadRigConfig(dir)
	items := args[1:]

	for _, item := range items {
		if !isIsolatable(item) {
			return fmt.Errorf("%q is not isolatable. Valid items: %s", item, strings.Join(isolatableItems, ", "))
		}
		if !cfg.isIsolated(item) {
			fmt.Printf("  %s — already shared\n", item)
			continue
		}

		target := filepath.Join(home, item)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			fmt.Printf("  %s — skipped (does not exist in ~/.claude/)\n", item)
			continue
		}

		linkPath := filepath.Join(dir, item)

		// Remove local copy
		os.RemoveAll(linkPath)

		// Recreate symlink
		if err := os.Symlink(target, linkPath); err != nil {
			return fmt.Errorf("symlinking %s: %w", item, err)
		}

		// Remove from isolate list
		for i, v := range cfg.Isolate {
			if v == item {
				cfg.Isolate = append(cfg.Isolate[:i], cfg.Isolate[i+1:]...)
				break
			}
		}

		fmt.Printf("  %s — shared\n", item)
	}

	if err := saveRigConfig(dir, cfg); err != nil {
		return fmt.Errorf("saving rig config: %w", err)
	}

	fmt.Printf("Rig %q: %d item(s) isolated\n", name, len(cfg.Isolate))
	return nil
}

// cmdIsolation shows isolation status for one or all rigs.
func cmdIsolation(args []string) error {
	root, err := rigsRoot()
	if err != nil {
		return err
	}

	// Single rig
	if len(args) > 0 {
		name := args[0]
		dir, err := rigDir(name)
		if err != nil {
			return err
		}
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("rig %q does not exist", name)
		}

		cfg := loadRigConfig(dir)
		printIsolationStatus(name, cfg)
		return nil
	}

	// All rigs
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No rigs found.")
			return nil
		}
		return err
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir, _ := rigDir(e.Name())
		cfg := loadRigConfig(dir)
		printIsolationStatus(e.Name(), cfg)
	}
	return nil
}

func printIsolationStatus(name string, cfg rigConfig) {
	fmt.Printf("%s:\n", name)
	for _, item := range isolatableItems {
		status := "shared"
		if cfg.isIsolated(item) {
			status = "isolated"
		}
		fmt.Printf("  %-20s %s\n", item, status)
	}
	if len(cfg.Inherit) > 0 {
		fmt.Printf("  Inheriting: %s\n", strings.Join(cfg.Inherit, ", "))
	}
}

// cmdInherit enables global inheritance for skills/agents/hooks/commands.
func cmdInherit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig inherit <items...|--all> [rig]\nInheritable: %s", strings.Join(inheritableItems, ", "))
	}

	var items []string
	var name string
	var all bool
	for _, a := range args {
		if a == "--all" {
			all = true
		} else if isInheritable(a) {
			items = append(items, a)
		} else if name == "" {
			name = a
		}
	}
	if all {
		items = inheritableItems
	}
	if len(items) == 0 {
		return fmt.Errorf("specify items to inherit or --all.\nInheritable: %s", strings.Join(inheritableItems, ", "))
	}

	// If no rig name given, try RC file
	if name == "" {
		rig, _, err := findRC()
		if err != nil {
			return err
		}
		if rig == "" {
			return fmt.Errorf("usage: claude-rig inherit <items...|--all> [rig]")
		}
		name = rig
	}

	dir, err := rigDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("rig %q does not exist", name)
	}

	cfg := loadRigConfig(dir)

	for _, item := range items {
		if cfg.isInherited(item) {
			fmt.Printf("  %s — already inherited\n", item)
			continue
		}
		cfg.Inherit = append(cfg.Inherit, item)
		fmt.Printf("  %s — inherited\n", item)
	}

	if err := saveRigConfig(dir, cfg); err != nil {
		return fmt.Errorf("saving rig config: %w", err)
	}

	// Sync immediately so the symlinks are created
	if err := syncGlobalContents(dir); err != nil {
		return fmt.Errorf("syncing global contents: %w", err)
	}

	fmt.Printf("Rig %q: inheriting %s\n", name, strings.Join(cfg.Inherit, ", "))
	return nil
}

// cmdUninherit disables global inheritance for skills/agents/hooks/commands.
func cmdUninherit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig uninherit <items...|--all> [rig]\nInheritable: %s", strings.Join(inheritableItems, ", "))
	}

	var items []string
	var name string
	var all bool
	for _, a := range args {
		if a == "--all" {
			all = true
		} else if isInheritable(a) {
			items = append(items, a)
		} else if name == "" {
			name = a
		}
	}
	if all {
		items = inheritableItems
	}
	if len(items) == 0 {
		return fmt.Errorf("specify items to uninherit or --all.\nInheritable: %s", strings.Join(inheritableItems, ", "))
	}

	// If no rig name given, try RC file
	if name == "" {
		rig, _, err := findRC()
		if err != nil {
			return err
		}
		if rig == "" {
			return fmt.Errorf("usage: claude-rig uninherit <items...|--all> [rig]")
		}
		name = rig
	}

	dir, err := rigDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("rig %q does not exist", name)
	}

	home, err := globalClaudeHome()
	if err != nil {
		return err
	}

	cfg := loadRigConfig(dir)

	for _, item := range items {
		if !cfg.isInherited(item) {
			fmt.Printf("  %s — not inherited\n", item)
			continue
		}

		// Remove inherited symlinks from this directory
		itemDir := filepath.Join(dir, item)
		globalDir := filepath.Join(home, item)
		removeInheritedSymlinks(itemDir, globalDir)

		// Remove from inherit list
		for i, v := range cfg.Inherit {
			if v == item {
				cfg.Inherit = append(cfg.Inherit[:i], cfg.Inherit[i+1:]...)
				break
			}
		}
		fmt.Printf("  %s — uninherited\n", item)
	}

	if err := saveRigConfig(dir, cfg); err != nil {
		return fmt.Errorf("saving rig config: %w", err)
	}

	remaining := cfg.Inherit
	if len(remaining) == 0 {
		fmt.Printf("Rig %q: no inheritance\n", name)
	} else {
		fmt.Printf("Rig %q: inheriting %s\n", name, strings.Join(remaining, ", "))
	}
	return nil
}

// syncGlobalContents symlinks entries from ~/.claude/{skills,agents,hooks,commands}/
// into the rig's corresponding directories for inherited items.
// Only creates symlinks for entries that don't already exist locally in the rig.
// Also cleans up stale symlinks pointing to deleted global entries.
func syncGlobalContents(rigDir string) error {
	home, err := globalClaudeHome()
	if err != nil {
		return err
	}

	cfg := loadRigConfig(rigDir)

	for _, item := range inheritableItems {
		if !cfg.isInherited(item) {
			continue
		}

		globalDir := filepath.Join(home, item)
		localDir := filepath.Join(rigDir, item)

		// Clean up stale symlinks first
		removeStaleInheritedSymlinks(localDir, globalDir)

		// Read global entries
		entries, err := os.ReadDir(globalDir)
		if err != nil {
			continue // global dir doesn't exist or isn't readable
		}

		for _, e := range entries {
			name := e.Name()
			linkPath := filepath.Join(localDir, name)
			target := filepath.Join(globalDir, name)

			// Skip if something already exists locally
			if _, err := os.Lstat(linkPath); err == nil {
				continue
			}

			os.Symlink(target, linkPath)
		}
	}
	return nil
}

// removeInheritedSymlinks removes all symlinks in localDir that point into globalDir.
func removeInheritedSymlinks(localDir, globalDir string) {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		path := filepath.Join(localDir, e.Name())
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(path)
		if err != nil {
			continue
		}
		if strings.HasPrefix(target, globalDir+string(os.PathSeparator)) || target == globalDir {
			os.Remove(path)
		}
	}
}

// removeStaleInheritedSymlinks removes symlinks pointing to entries in globalDir
// that no longer exist.
func removeStaleInheritedSymlinks(localDir, globalDir string) {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		path := filepath.Join(localDir, e.Name())
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(path)
		if err != nil {
			continue
		}
		// Only clean up symlinks pointing into the global dir
		if !strings.HasPrefix(target, globalDir+string(os.PathSeparator)) {
			continue
		}
		// Remove if target no longer exists
		if _, err := os.Stat(target); os.IsNotExist(err) {
			os.Remove(path)
		}
	}
}

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

	// Check each rig
	root, _ := rigsRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	// Check for multiple rigs with remote control enabled
	var rcRigs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir, _ := rigDir(e.Name())
		data, err := os.ReadFile(filepath.Join(dir, ".claude.json"))
		if err != nil {
			continue
		}
		var cfg map[string]any
		if json.Unmarshal(data, &cfg) == nil {
			if rc, ok := cfg["remoteControlAtStartup"].(bool); ok && rc {
				rcRigs = append(rcRigs, e.Name())
			}
		}
	}
	if len(rcRigs) > 1 {
		fmt.Println("\nRemote Control:")
		warn("Multiple rigs have remoteControlAtStartup enabled: %s", strings.Join(rcRigs, ", "))
		fmt.Println("    Remote Control is fragile with multiple rigs sharing auth.")
		fmt.Println("    Enable it on one rig only to avoid connection cycling.")
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

// cmdInit sets up the rig system directory structure and shell integration.
func cmdInit() error {
	if err := checkSymlinkSupport(); err != nil {
		return err
	}

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

// cmdCreate creates a new rig directory with rig-specific items
// and symlinks to shared items in ~/.claude/.
func cmdCreate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig create <name> [--link-auth] [--isolate <items,...>] [--inherit-skills] [--inherit-agents] [--inherit-hooks] [--inherit-commands] [--inherit-all]")
	}

	var name string
	var linkAuth bool
	var isolateItems []string
	var inheritItems []string
	var inheritAll bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--link-auth":
			linkAuth = true
		case "--isolate":
			if i+1 >= len(args) {
				return fmt.Errorf("--isolate requires a comma-separated list of items")
			}
			i++
			isolateItems = strings.Split(args[i], ",")
		case "--inherit-all":
			inheritAll = true
		case "--inherit-skills":
			inheritItems = append(inheritItems, "skills")
		case "--inherit-agents":
			inheritItems = append(inheritItems, "agents")
		case "--inherit-hooks":
			inheritItems = append(inheritItems, "hooks")
		case "--inherit-commands":
			inheritItems = append(inheritItems, "commands")
		default:
			if name == "" {
				name = args[i]
			}
		}
	}
	if inheritAll {
		inheritItems = inheritableItems
	}
	if name == "" {
		return fmt.Errorf("usage: claude-rig create <name> [--link-auth] [--isolate <items,...>] [--inherit-all]")
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

	// Seed .claude.json with minimal fields from global config
	if err := seedClaudeJSON(dir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not seed .claude.json: %v\n", err)
	}

	// Write isolation config before syncing symlinks
	if len(isolateItems) > 0 {
		for _, item := range isolateItems {
			if !isIsolatable(item) {
				return fmt.Errorf("%q is not isolatable. Valid items: %s", item, strings.Join(isolatableItems, ", "))
			}
		}
		cfg := rigConfig{Isolate: isolateItems}
		if err := saveRigConfig(dir, cfg); err != nil {
			return fmt.Errorf("saving rig config: %w", err)
		}

		// Create local files/dirs for isolated items
		home, _ := claudeHome()
		for _, item := range isolateItems {
			localPath := filepath.Join(dir, item)
			srcPath := filepath.Join(home, item)
			srcInfo, srcErr := os.Stat(srcPath)
			if srcErr == nil && srcInfo.IsDir() {
				os.MkdirAll(localPath, 0755)
			} else if strings.Contains(item, ".") {
				os.WriteFile(localPath, []byte(""), 0644)
			} else {
				os.MkdirAll(localPath, 0755)
			}
		}
		fmt.Printf("Isolated: %s\n", strings.Join(isolateItems, ", "))
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

	// Set up inheritance if requested
	if len(inheritItems) > 0 {
		cfg := loadRigConfig(dir)
		cfg.Inherit = inheritItems
		if err := saveRigConfig(dir, cfg); err != nil {
			return fmt.Errorf("saving rig config: %w", err)
		}
		if err := syncGlobalContents(dir); err != nil {
			return fmt.Errorf("syncing inherited contents: %w", err)
		}
		fmt.Printf("Inheriting: %s\n", strings.Join(inheritItems, ", "))
	}

	fmt.Printf("Created rig %q at %s\n", name, dir)
	fmt.Printf("Launch with: claude-rig launch %s\n", name)
	return nil
}

// cmdClone duplicates a rig. Symlinks are recreated, real files are copied.
// Use "default" as source to clone from ~/.claude/.
func cmdClone(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: claude-rig clone <source|default> <dest> [--link-auth] [--inherit-all]")
	}

	var positional []string
	var linkAuth bool
	var inheritItems []string
	var inheritAll bool
	for _, a := range args {
		switch a {
		case "--link-auth":
			linkAuth = true
		case "--inherit-all":
			inheritAll = true
		case "--inherit-skills":
			inheritItems = append(inheritItems, "skills")
		case "--inherit-agents":
			inheritItems = append(inheritItems, "agents")
		case "--inherit-hooks":
			inheritItems = append(inheritItems, "hooks")
		case "--inherit-commands":
			inheritItems = append(inheritItems, "commands")
		default:
			positional = append(positional, a)
		}
	}
	if inheritAll {
		inheritItems = inheritableItems
	}
	if len(positional) < 2 {
		return fmt.Errorf("usage: claude-rig clone <source|default> <dest> [--link-auth] [--inherit-all]")
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

	// Set up inheritance if requested
	if len(inheritItems) > 0 {
		cfg := loadRigConfig(destDir)
		cfg.Inherit = inheritItems
		if err := saveRigConfig(destDir, cfg); err != nil {
			return fmt.Errorf("saving rig config: %w", err)
		}
		if err := syncGlobalContents(destDir); err != nil {
			return fmt.Errorf("syncing inherited contents: %w", err)
		}
		fmt.Printf("Inheriting: %s\n", strings.Join(inheritItems, ", "))
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

	// Seed .claude.json with onboarding state + MCP from global
	if err := seedClaudeJSON(destDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not seed .claude.json: %v\n", err)
	}

	// Copy MCP servers from global ~/.claude.json if present
	globalClaude := filepath.Join(userHome, ".claude.json")
	if data, err := os.ReadFile(globalClaude); err == nil {
		var global map[string]any
		if json.Unmarshal(data, &global) == nil {
			if mcpServers, ok := global["mcpServers"]; ok {
				destClaude := filepath.Join(destDir, ".claude.json")
				if rigData, err := os.ReadFile(destClaude); err == nil {
					var rigJSON map[string]any
					if json.Unmarshal(rigData, &rigJSON) == nil {
						rigJSON["mcpServers"] = mcpServers
						if out, err := json.MarshalIndent(rigJSON, "", "  "); err == nil {
							os.WriteFile(destClaude, append(out, '\n'), 0644)
						}
					}
				}
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

	// Resolve auth source directory
	var authHome string
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
		authHome = fromDir
	} else {
		authHome, err = claudeHome()
		if err != nil {
			return err
		}
	}

	for _, item := range authItems {
		target := filepath.Join(authHome, item)
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

		sessions := rigRunningSessions(dir)
		marker := "  "
		if len(sessions) > 0 {
			marker = "* "
		}

		auth := rigAuthStatus(dir, rigDirs)
		skills := countDirEntries(filepath.Join(dir, "skills"))
		plugins := countDirEntries(filepath.Join(dir, "plugins"))
		mcp := countMCPServers(filepath.Join(dir, ".claude.json"))
		isolated := len(loadRigConfig(dir).Isolate)

		info := fmt.Sprintf("auth: %-20s skills: %d  plugins: %d  mcp: %d",
			auth, skills, plugins, mcp)
		if isolated > 0 {
			info += fmt.Sprintf("  isolated: %d", isolated)
		}
		fmt.Printf("%s%-20s %s\n", marker, name, info)
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

	fmt.Printf("Deleted rig %q\n", name)
	return nil
}

// cmdRename renames a rig directory.
func cmdRename(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: claude-rig rename <old> <new>")
	}
	oldName, newName := args[0], args[1]

	if err := validateRigName(newName); err != nil {
		return err
	}

	oldDir, err := rigDir(oldName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(oldDir); os.IsNotExist(err) {
		return fmt.Errorf("rig %q does not exist", oldName)
	}

	newDir, err := rigDir(newName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(newDir); err == nil {
		return fmt.Errorf("rig %q already exists", newName)
	}

	if err := os.Rename(oldDir, newDir); err != nil {
		return fmt.Errorf("renaming rig: %w", err)
	}

	fmt.Printf("Renamed %q → %q\n", oldName, newName)
	fmt.Println("Note: update any .claude-rig files that reference the old name")
	return nil
}

// cmdSync refreshes shared symlinks and inherited contents for one or all rigs.
func cmdSync(args []string) error {
	root, err := rigsRoot()
	if err != nil {
		return err
	}

	var names []string
	if len(args) > 0 {
		// Sync specific rig
		name := args[0]
		dir, err := rigDir(name)
		if err != nil {
			return err
		}
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("rig %q does not exist", name)
		}
		names = append(names, name)
	} else {
		// Sync all rigs
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No rigs found.")
				return nil
			}
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				names = append(names, e.Name())
			}
		}
	}

	for _, name := range names {
		dir, _ := rigDir(name)
		if err := syncSharedSymlinks(dir); err != nil {
			fmt.Fprintf(os.Stderr, "  %s: symlink sync error: %v\n", name, err)
			continue
		}
		if err := syncGlobalContents(dir); err != nil {
			fmt.Fprintf(os.Stderr, "  %s: inheritance sync error: %v\n", name, err)
			continue
		}
		fmt.Printf("  %s — synced\n", name)
	}
	return nil
}

// cmdUpdate forwards to `claude update`.
func cmdUpdate(args []string) error {
	binary := claudeCodeBinary()
	binPath, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("claude binary not found: %w (set CLAUDE_BINARY to override)", err)
	}

	cmd := exec.Command(binPath, append([]string{"update"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
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

	// Sync inherited global contents (skills, agents, hooks, commands)
	if err := syncGlobalContents(dir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not sync inherited contents: %v\n", err)
	}

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
	env = removeEnv(env, "CLAUDECODE") // allow launching from within a Claude Code shell

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
	return execLaunch(binPath, execArgs, env)
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

// cmdUpdatePlugins refreshes marketplaces and updates all installed plugins across rigs.
func cmdUpdatePlugins(args []string) error {
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

	// Determine which rigs to update
	var rigNames []string
	if len(args) > 0 {
		for _, name := range args {
			dir := filepath.Join(root, name)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return fmt.Errorf("rig %q does not exist", name)
			}
			rigNames = append(rigNames, name)
		}
	} else {
		for _, e := range entries {
			if e.IsDir() {
				rigNames = append(rigNames, e.Name())
			}
		}
	}

	if len(rigNames) == 0 {
		fmt.Println("No rigs found.")
		return nil
	}

	claudeBin := claudeCodeBinary()

	type rigResult struct {
		name   string
		output string
		failed bool
	}

	fmt.Printf("Updating %d rigs: %s ...\n", len(rigNames), strings.Join(rigNames, ", "))

	results := make([]rigResult, len(rigNames))
	var wg sync.WaitGroup

	for i, name := range rigNames {
		wg.Add(1)
		go func(idx int, rigName string) {
			defer wg.Done()
			dir := filepath.Join(root, rigName)
			var buf strings.Builder
			var failed bool

			fmt.Fprintf(&buf, "\n── %s ──\n", rigName)

			// Refresh marketplaces
			buf.WriteString("  Updating marketplaces... ")
			if out, err := runClaudePlugin(claudeBin, dir, "marketplace", "update"); err != nil {
				fmt.Fprintf(&buf, "FAILED\n    %s\n", lastLine(out))
				failed = true
				results[idx] = rigResult{rigName, buf.String(), failed}
				return
			} else {
				buf.WriteString(lastLine(out) + "\n")
			}

			// Get installed plugins
			out, err := runClaudePlugin(claudeBin, dir, "list", "--json")
			if err != nil {
				fmt.Fprintf(&buf, "  Could not list plugins: %s\n", lastLine(out))
				failed = true
				results[idx] = rigResult{rigName, buf.String(), failed}
				return
			}

			var plugins []struct {
				ID      string `json:"id"`
				Version string `json:"version"`
			}
			if err := json.Unmarshal([]byte(out), &plugins); err != nil {
				fmt.Fprintf(&buf, "  Could not parse plugin list: %v\n", err)
				failed = true
				results[idx] = rigResult{rigName, buf.String(), failed}
				return
			}

			if len(plugins) == 0 {
				buf.WriteString("  No plugins installed\n")
				results[idx] = rigResult{rigName, buf.String(), false}
				return
			}

			// Update each plugin
			for _, p := range plugins {
				fmt.Fprintf(&buf, "  %-40s ", p.ID)
				if out, err := runClaudePlugin(claudeBin, dir, "update", p.ID); err != nil {
					fmt.Fprintf(&buf, "FAILED: %s\n", lastLine(out))
					failed = true
				} else {
					buf.WriteString(lastLine(out) + "\n")
				}
			}

			results[idx] = rigResult{rigName, buf.String(), failed}
		}(i, name)
	}

	wg.Wait()

	var hadErrors bool
	for _, r := range results {
		fmt.Print(r.output)
		if r.failed {
			hadErrors = true
		}
	}

	if hadErrors {
		fmt.Println("\nSome updates failed. Check the output above.")
	} else {
		fmt.Println("\nAll plugins up to date.")
	}
	return nil
}

// runClaudePlugin runs a `claude plugin` subcommand with CLAUDE_CONFIG_DIR set to the rig directory.
func runClaudePlugin(claudeBin, rigDir string, pluginArgs ...string) (string, error) {
	args := append([]string{"plugin"}, pluginArgs...)
	cmd := exec.Command(claudeBin, args...)

	// Build env: inherit current, set CLAUDE_CONFIG_DIR, unset CLAUDECODE (nesting guard)
	env := os.Environ()
	env = setEnv(env, "CLAUDE_CONFIG_DIR", rigDir)
	env = removeEnv(env, "CLAUDECODE")
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// lastLine returns the last non-empty line from s (result messages are typically last).
func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}

// --- helpers ---

func linkAuthFiles(rigDir string) error {
	home, err := claudeHome()
	if err != nil {
		return err
	}

	for _, item := range authItems {
		target := filepath.Join(home, item)
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

	return nil
}

// seedClaudeJSON creates a minimal .claude.json in the rig directory,
// seeded from the global ~/.claude.json to skip onboarding.
func seedClaudeJSON(rigDir string) error {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Read global .claude.json for seed fields
	globalPath := filepath.Join(userHome, ".claude.json")
	seed := map[string]any{
		"hasCompletedOnboarding":                true,
		"officialMarketplaceAutoInstallAttempted": true,
		"officialMarketplaceAutoInstalled":        true,
		"mcpServers":                              map[string]any{},
	}

	if data, err := os.ReadFile(globalPath); err == nil {
		var global map[string]any
		if json.Unmarshal(data, &global) == nil {
			for _, key := range []string{"oauthAccount", "userID", "lastOnboardingVersion"} {
				if v, ok := global[key]; ok {
					seed[key] = v
				}
			}
		}
	}

	data, err := json.MarshalIndent(seed, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(rigDir, ".claude.json"), append(data, '\n'), 0644)
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

	cfg := loadRigConfig(rigDir)

	for _, e := range entries {
		name := e.Name()

		// Skip rig-specific items, hidden files, and isolated items
		if isRigSpecific(name) || strings.HasPrefix(name, ".") || cfg.isIsolated(name) {
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

func removeEnv(env []string, key string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			return append(env[:i], env[i+1:]...)
		}
	}
	return env
}

func cmdStatus(args []string) error {
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

	// Build rigDirs map for auth resolution
	rigDirs := map[string]string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir, _ := rigDir(e.Name())
		rigDirs[dir] = e.Name()
	}

	// Single rig mode
	if len(args) > 0 {
		name := args[0]
		dir, err := rigDir(name)
		if err != nil {
			return err
		}
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("rig %q does not exist", name)
		}
		return printStatusDetail(name, dir, rigDirs)
	}

	// Overview table of all rigs
	found := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		found = true
		name := e.Name()
		dir, _ := rigDir(name)

		sessions := rigRunningSessions(dir)
		marker := "  "
		if len(sessions) > 0 {
			marker = "* "
		}

		auth := rigAuthStatus(dir, rigDirs)
		plugins := countDirEntries(filepath.Join(dir, "plugins"))
		mcp := countMCPServers(filepath.Join(dir, ".claude.json"))
		isolated := len(loadRigConfig(dir).Isolate)
		disk := formatBytes(rigDiskUsage(dir))
		lastUsed := rigLastUsed(dir)

		sessionStr := ""
		if len(sessions) > 0 {
			sessionStr = fmt.Sprintf("  running:%d", len(sessions))
		}

		lastStr := ""
		if !lastUsed.IsZero() {
			lastStr = "  last: " + formatTimeAgo(lastUsed)
		}

		fmt.Printf("%s%-18s auth: %-20s plugins: %d  mcp: %d  isolated: %d  disk: %-5s%s%s\n",
			marker, name, auth, plugins, mcp, isolated, disk, sessionStr, lastStr)
	}

	if !found {
		fmt.Println("No rigs found. Run: claude-rig create <name>")
	}
	return nil
}

func printStatusDetail(name, dir string, rigDirs map[string]string) error {
	auth := rigAuthStatus(dir, rigDirs)
	skills := countDirEntries(filepath.Join(dir, "skills"))
	plugins := countDirEntries(filepath.Join(dir, "plugins"))
	mcp := countMCPServers(filepath.Join(dir, ".claude.json"))
	cfg := loadRigConfig(dir)
	sessions := rigRunningSessions(dir)
	lastUsed := rigLastUsed(dir)
	realSize, symlinkSize := rigDiskUsageDetailed(dir)

	fmt.Printf("Rig: %s\n", name)
	fmt.Printf("  Auth:       %s\n", auth)
	fmt.Printf("  Skills:     %d\n", skills)
	fmt.Printf("  Plugins:    %d\n", plugins)
	fmt.Printf("  MCP:        %d\n", mcp)

	if len(cfg.Isolate) > 0 {
		fmt.Printf("  Isolated:   %s\n", strings.Join(cfg.Isolate, ", "))
	} else {
		fmt.Printf("  Isolated:   none\n")
	}

	totalSize := realSize + symlinkSize
	if symlinkSize > 0 {
		fmt.Printf("  Disk:       %s (real: %s, symlinked: %s)\n",
			formatBytes(totalSize), formatBytes(realSize), formatBytes(symlinkSize))
	} else {
		fmt.Printf("  Disk:       %s\n", formatBytes(totalSize))
	}

	if len(sessions) > 0 {
		pids := make([]string, len(sessions))
		for i, pid := range sessions {
			pids[i] = fmt.Sprintf("%d", pid)
		}
		fmt.Printf("  Sessions:   %d running (PID %s)\n", len(sessions), strings.Join(pids, ", "))
	} else {
		fmt.Printf("  Sessions:   none\n")
	}

	if !lastUsed.IsZero() {
		fmt.Printf("  Last used:  %s\n", formatTimeAgo(lastUsed))
	} else {
		fmt.Printf("  Last used:  never\n")
	}

	fmt.Printf("  Path:       %s\n", dir)
	return nil
}

// rigDiskUsage returns the total size of real (non-symlinked) files in the rig directory.
func rigDiskUsage(dir string) int64 {
	size, _ := rigDiskUsageDetailed(dir)
	return size
}

// rigDiskUsageDetailed returns (real file size, symlink target size) for the rig directory.
func rigDiskUsageDetailed(dir string) (int64, int64) {
	var realSize, symlinkSize int64
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		linfo, lerr := os.Lstat(path)
		if lerr != nil {
			return nil
		}
		if linfo.Mode()&os.ModeSymlink != 0 {
			// Follow symlink to get target size
			if tgt, terr := os.Stat(path); terr == nil {
				if tgt.IsDir() {
					// Walk the symlinked directory
					filepath.Walk(path, func(_ string, fi os.FileInfo, e error) error {
						if e != nil || fi.IsDir() {
							return nil
						}
						symlinkSize += fi.Size()
						return nil
					})
				} else {
					symlinkSize += tgt.Size()
				}
			}
			return filepath.SkipDir
		}
		if !info.IsDir() {
			realSize += info.Size()
		}
		return nil
	})
	return realSize, symlinkSize
}

// rigLastUsed returns the modification time of the rig directory.
func rigLastUsed(dir string) time.Time {
	info, err := os.Stat(dir)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%dM", b/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%dK", b/(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

type exportOpts struct {
	IncludeAuth bool
	IncludeData bool
}

func isAuthFile(name string) bool {
	for _, item := range authItems {
		if name == item {
			return true
		}
	}
	return false
}

func cmdExport(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig export <rig> [file] [--include-auth] [--include-data]")
	}

	var name, destFile string
	var opts exportOpts
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--include-auth":
			opts.IncludeAuth = true
		case "--include-data":
			opts.IncludeData = true
		default:
			if name == "" {
				name = args[i]
			} else if destFile == "" {
				destFile = args[i]
			}
		}
	}
	if name == "" {
		return fmt.Errorf("usage: claude-rig export <rig> [file] [--include-auth] [--include-data]")
	}

	dir, err := rigDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("rig %q does not exist", name)
	}

	if destFile == "" {
		destFile = name + ".tar.gz"
	}

	cfg := loadRigConfig(dir)

	if err := createTarGz(dir, destFile, opts, cfg); err != nil {
		return fmt.Errorf("creating archive: %w", err)
	}

	info, _ := os.Stat(destFile)
	fmt.Printf("Exported rig %q → %s (%s)\n", name, destFile, formatBytes(info.Size()))
	return nil
}

func createTarGz(dir, dest string, opts exportOpts, cfg rigConfig) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Get path relative to rig dir
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		// Top-level name for filtering
		topLevel := strings.SplitN(rel, string(filepath.Separator), 2)[0]

		// Skip symlinks — shared items are recreated by syncSharedSymlinks on import.
		// Exception: auth symlinks when --include-auth, so we can export linked auth.
		linfo, err := os.Lstat(path)
		if err != nil {
			return nil
		}
		if linfo.Mode()&os.ModeSymlink != 0 {
			if opts.IncludeAuth && isAuthFile(topLevel) {
				// Follow the symlink — use the real file info for the tar header
				info, err = os.Stat(path)
				if err != nil {
					return nil
				}
				if info.IsDir() {
					// Auth directory (e.g. statsig/) — walk it manually
					return addDirToTar(tw, path, topLevel)
				}
				// Auth file — fall through to write it
				linfo = info
			} else {
				return nil
			}
		}

		// Auth files: skip unless --include-auth
		if isAuthFile(topLevel) && !opts.IncludeAuth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Isolated data: items in rig.json isolate list that aren't rig-specific
		if cfg.isIsolated(topLevel) && !isRigSpecific(topLevel) && !opts.IncludeData {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Write tar header
		header, err := tar.FileInfoHeader(linfo, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// Write file content
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})
}

// addDirToTar walks a symlinked directory and adds its contents to the tar.
// prefix is the name to use in the archive (e.g. "statsig").
func addDirToTar(tw *tar.Writer, realPath, prefix string) error {
	target, err := filepath.EvalSymlinks(realPath)
	if err != nil {
		return nil
	}
	return filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(target, path)
		if err != nil {
			return nil
		}
		archiveName := prefix
		if rel != "." {
			archiveName = filepath.Join(prefix, rel)
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = archiveName
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(tw, f)
			return err
		}
		return nil
	})
}

func cmdImport(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: claude-rig import <file> <name> [--link-auth]")
	}

	var srcFile, name string
	var linkAuth bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--link-auth":
			linkAuth = true
		default:
			if srcFile == "" {
				srcFile = args[i]
			} else if name == "" {
				name = args[i]
			}
		}
	}
	if srcFile == "" || name == "" {
		return fmt.Errorf("usage: claude-rig import <file> <name> [--link-auth]")
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

	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		return fmt.Errorf("archive not found: %s", srcFile)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating rig directory: %w", err)
	}

	count, err := extractTarGz(srcFile, dir)
	if err != nil {
		os.RemoveAll(dir)
		return fmt.Errorf("extracting archive: %w", err)
	}

	// Seed .claude.json if the archive didn't include one
	if _, err := os.Stat(filepath.Join(dir, ".claude.json")); os.IsNotExist(err) {
		if err := seedClaudeJSON(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not seed .claude.json: %v\n", err)
		}
	}

	// Create shared symlinks
	if err := syncSharedSymlinks(dir); err != nil {
		return fmt.Errorf("creating shared symlinks: %w", err)
	}

	// Optionally link auth
	if linkAuth {
		if err := linkAuthFiles(dir); err != nil {
			return fmt.Errorf("linking auth files: %w", err)
		}
		fmt.Println("Linked auth from existing Claude config")
	}

	fmt.Printf("Imported %d items into rig %q at %s\n", count, name, dir)
	fmt.Printf("Launch with: claude-rig launch %s\n", name)
	return nil
}

func extractTarGz(src, destDir string) (int, error) {
	f, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	count := 0

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}

		// Sanitize: reject paths that escape the dest dir
		clean := filepath.Clean(header.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			continue
		}

		target := filepath.Join(destDir, clean)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return count, err
			}
		case tar.TypeReg:
			// Ensure parent dir exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return count, err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return count, err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return count, err
			}
			out.Close()
			count++
		}
	}
	return count, nil
}

func cmdDiff(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: claude-rig diff <rig1> <rig2>")
	}
	name1, name2 := args[0], args[1]

	dir1, err := rigDir(name1)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir1); err != nil {
		return fmt.Errorf("rig %q does not exist", name1)
	}
	dir2, err := rigDir(name2)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir2); err != nil {
		return fmt.Errorf("rig %q does not exist", name2)
	}

	// Build rigDirs map for auth status resolution
	root, _ := rigsRoot()
	rigDirs := map[string]string{}
	if entries, err := os.ReadDir(root); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				rigDirs[filepath.Join(root, e.Name())] = e.Name()
			}
		}
	}

	// Auth
	auth1 := rigAuthStatus(dir1, rigDirs)
	auth2 := rigAuthStatus(dir2, rigDirs)
	if auth1 == auth2 {
		fmt.Printf("  Auth:       same (%s)\n", auth1)
	} else {
		fmt.Printf("  Auth:       %s vs %s\n", auth1, auth2)
	}

	// Settings
	s1 := filepath.Join(dir1, "settings.json")
	s2 := filepath.Join(dir2, "settings.json")
	diffKeys := settingsKeyDiff(s1, s2)
	if len(diffKeys) == 0 {
		fmt.Println("  Settings:   same")
	} else {
		fmt.Printf("  Settings:   %d differences (%s)\n", len(diffKeys), strings.Join(diffKeys, ", "))
	}

	// Dir-based categories
	type dirCat struct {
		label string
		sub   string
	}
	dirCats := []dirCat{
		{"Plugins", "plugins"},
		{"Skills", "skills"},
		{"Agents", "agents"},
		{"Commands", "commands"},
		{"Hooks", "hooks"},
	}
	for _, cat := range dirCats {
		e1 := dirEntryNames(filepath.Join(dir1, cat.sub))
		e2 := dirEntryNames(filepath.Join(dir2, cat.sub))
		printSetDiff(cat.label, name1, name2, e1, e2)
	}

	// MCP servers
	mcp1 := mcpServerNames(filepath.Join(dir1, ".claude.json"))
	mcp2 := mcpServerNames(filepath.Join(dir2, ".claude.json"))
	printSetDiff("MCP", name1, name2, mcp1, mcp2)

	// Isolation
	cfg1 := loadRigConfig(dir1)
	cfg2 := loadRigConfig(dir2)
	iso1 := cfg1.Isolate
	iso2 := cfg2.Isolate
	if len(iso1) == 0 && len(iso2) == 0 {
		fmt.Println("  Isolation:  same (none)")
	} else {
		s1 := "none"
		s2 := "none"
		if len(iso1) > 0 {
			s1 = strings.Join(iso1, ", ")
		}
		if len(iso2) > 0 {
			s2 = strings.Join(iso2, ", ")
		}
		if s1 == s2 {
			fmt.Printf("  Isolation:  same (%s)\n", s1)
		} else {
			fmt.Printf("  Isolation:  %s=%s, %s=%s\n", name1, s1, name2, s2)
		}
	}

	return nil
}

func printSetDiff(label, name1, name2 string, a, b []string) {
	onlyA, onlyB := setDiff(a, b)
	pad := strings.Repeat(" ", 10-len(label))
	if len(onlyA) == 0 && len(onlyB) == 0 {
		fmt.Printf("  %s:%s same (%d)\n", label, pad, len(a))
		return
	}
	parts := []string{fmt.Sprintf("%s has %d, %s has %d", name1, len(a), name2, len(b))}
	if len(onlyA) > 0 {
		parts = append(parts, fmt.Sprintf("only %s: %s", name1, strings.Join(onlyA, ", ")))
	}
	if len(onlyB) > 0 {
		parts = append(parts, fmt.Sprintf("only %s: %s", name2, strings.Join(onlyB, ", ")))
	}
	fmt.Printf("  %s:%s %s\n", label, pad, strings.Join(parts, " | "))
}

func dirEntryNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func mcpServerNames(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	names := make([]string, 0, len(cfg.MCPServers))
	for k := range cfg.MCPServers {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func settingsKeyDiff(path1, path2 string) []string {
	read := func(p string) map[string]json.RawMessage {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		var m map[string]json.RawMessage
		json.Unmarshal(data, &m)
		return m
	}
	m1, m2 := read(path1), read(path2)
	if m1 == nil && m2 == nil {
		return nil
	}
	// Collect all keys
	keys := map[string]bool{}
	for k := range m1 {
		keys[k] = true
	}
	for k := range m2 {
		keys[k] = true
	}
	var diff []string
	for k := range keys {
		v1, ok1 := m1[k]
		v2, ok2 := m2[k]
		if !ok1 || !ok2 || string(v1) != string(v2) {
			diff = append(diff, k)
		}
	}
	sort.Strings(diff)
	return diff
}

func setDiff(a, b []string) (onlyA, onlyB []string) {
	setA := map[string]bool{}
	setB := map[string]bool{}
	for _, s := range a {
		setA[s] = true
	}
	for _, s := range b {
		setB[s] = true
	}
	for _, s := range a {
		if !setB[s] {
			onlyA = append(onlyA, s)
		}
	}
	for _, s := range b {
		if !setA[s] {
			onlyB = append(onlyB, s)
		}
	}
	return
}
