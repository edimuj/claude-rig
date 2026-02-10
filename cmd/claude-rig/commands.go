package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// cmdDoctor checks the health of the profile system.
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

	// Check active profile
	active := getActiveProfile()
	if active != "" {
		dir, _ := profileDir(active)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			warn("Active profile %q does not exist", active)
		} else {
			ok("Active profile: %s", active)
		}
	}

	// Check each profile
	root, _ := profilesRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		dir, _ := profileDir(name)

		fmt.Printf("\nProfile %q:\n", name)

		// Check profile-specific items exist
		for _, item := range profileSpecificItems {
			path := filepath.Join(dir, item)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				warn("Missing: %s", item)
			}
		}

		// Walk all entries and check for broken symlinks
		dirEntries, err := os.ReadDir(dir)
		if err != nil {
			warn("Cannot read profile directory: %v", err)
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

// cmdInit sets up the profile system directory structure.
func cmdInit() error {
	root, err := profilesRoot()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(root, 0755); err != nil {
		return fmt.Errorf("creating profiles directory: %w", err)
	}

	rig, _ := rigHome()
	fmt.Printf("Initialized profile system at %s\n", rig)
	fmt.Println("Next: claude-rig create <name>")
	return nil
}

// cmdCreate creates a new profile directory with profile-specific items
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

	if err := validateProfileName(name); err != nil {
		return err
	}

	dir, err := profileDir(name)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("profile %q already exists", name)
	}

	root, err := profilesRoot()
	if err != nil {
		return err
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return fmt.Errorf("profile system not initialized — run: claude-rig init")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating profile directory: %w", err)
	}

	// Create profile-specific directories
	for _, item := range profileSpecificItems {
		path := filepath.Join(dir, item)
		if strings.HasSuffix(item, ".json") {
			// Create empty JSON files
			if err := os.WriteFile(path, []byte("{}\n"), 0644); err != nil {
				return fmt.Errorf("creating %s: %w", item, err)
			}
		} else {
			// Create directories
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

	fmt.Printf("Created profile %q at %s\n", name, dir)
	fmt.Printf("Launch with: claude-rig launch %s\n", name)
	return nil
}

// cmdClone duplicates a profile. Symlinks are recreated, real files are copied.
func cmdClone(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: claude-rig clone <source> <dest> [--link-auth]")
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
		return fmt.Errorf("usage: claude-rig clone <source> <dest> [--link-auth]")
	}
	srcName, destName := positional[0], positional[1]

	if err := validateProfileName(destName); err != nil {
		return err
	}

	srcDir, err := profileDir(srcName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", srcName)
	}

	destDir, err := profileDir(destName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("profile %q already exists", destName)
	}

	if err := cloneDir(srcDir, destDir); err != nil {
		os.RemoveAll(destDir) // clean up partial clone
		return fmt.Errorf("cloning profile: %w", err)
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

// cmdLinkAuth links auth files into a profile from ~/.claude/ or another profile.
func cmdLinkAuth(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig link-auth <name> [--from <profile>]")
	}

	var name, fromProfile string
	for i := 0; i < len(args); i++ {
		if args[i] == "--from" && i+1 < len(args) {
			fromProfile = args[i+1]
			i++
		} else if name == "" {
			name = args[i]
		}
	}
	if name == "" {
		return fmt.Errorf("usage: claude-rig link-auth <name> [--from <profile>]")
	}

	dir, err := profileDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", name)
	}

	// Resolve auth source directories
	var authHome, authUserHome string
	if fromProfile != "" {
		if fromProfile == name {
			return fmt.Errorf("cannot link auth from a profile to itself")
		}
		fromDir, err := profileDir(fromProfile)
		if err != nil {
			return err
		}
		if _, err := os.Stat(fromDir); os.IsNotExist(err) {
			return fmt.Errorf("source profile %q does not exist", fromProfile)
		}
		// All auth items live inside the profile dir
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
		// .claude.json lives in ~ when linking from default, but inside profile dir when linking from profile
		sourceDir := authHome
		if item == ".claude.json" && fromProfile == "" {
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
			fmt.Printf("Profile %q already has %s. Replace with shared auth? [y/N] ", name, item)
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

	if fromProfile != "" {
		fmt.Printf("Profile %q now uses auth from %q\n", name, fromProfile)
	} else {
		fmt.Printf("Profile %q now uses shared auth\n", name)
	}
	return nil
}

// cmdUnlinkAuth removes shared auth symlinks from a profile.
func cmdUnlinkAuth(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig unlink-auth <name>")
	}
	name := args[0]

	dir, err := profileDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", name)
	}

	removed := 0
	for _, item := range authItems {
		linkPath := filepath.Join(dir, item)
		info, err := os.Lstat(linkPath)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue // not a symlink, leave it alone
		}
		os.Remove(linkPath)
		fmt.Printf("Removed %s\n", item)
		removed++
	}

	if removed == 0 {
		fmt.Printf("Profile %q has no shared auth links\n", name)
	} else {
		fmt.Printf("Profile %q will use its own auth on next launch\n", name)
	}
	return nil
}

// cmdList shows all profiles with auth status and item counts.
func cmdList() error {
	root, err := profilesRoot()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No profiles found. Run: claude-rig init")
			return nil
		}
		return err
	}

	active := getActiveProfile()

	// First pass: collect names to resolve auth targets to profile names
	profileDirs := map[string]string{} // dir path → profile name
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir, _ := profileDir(e.Name())
		profileDirs[dir] = e.Name()
	}

	found := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		found = true
		name := e.Name()
		dir, _ := profileDir(name)

		marker := "  "
		if name == active {
			marker = "* "
		}

		auth := profileAuthStatus(dir, profileDirs)
		skills := countDirEntries(filepath.Join(dir, "skills"))
		plugins := countDirEntries(filepath.Join(dir, "plugins"))
		mcp := countMCPServers(filepath.Join(dir, "mcp.json"))

		fmt.Printf("%s%-20s auth: %-20s skills: %d  plugins: %d  mcp: %d\n",
			marker, name, auth, skills, plugins, mcp)
	}

	if !found {
		fmt.Println("No profiles found. Run: claude-rig create <name>")
	}
	return nil
}

// profileAuthStatus returns the auth status for a profile.
func profileAuthStatus(dir string, profileDirs map[string]string) string {
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
	if name, ok := profileDirs[targetDir]; ok {
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

// cmdUse sets the active profile (for shell alias workflows).
func cmdUse(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig use <name>")
	}
	name := args[0]

	dir, err := profileDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", name)
	}

	file, err := activeProfileFile()
	if err != nil {
		return err
	}
	if err := os.WriteFile(file, []byte(name), 0644); err != nil {
		return fmt.Errorf("writing active profile: %w", err)
	}

	fmt.Printf("Active profile: %s\n", name)
	fmt.Printf("Launch with: CLAUDE_CONFIG_DIR=%s claude\n", dir)
	return nil
}

// cmdCurrent shows the active profile.
func cmdCurrent() error {
	active := getActiveProfile()
	if active == "" {
		fmt.Println("No active profile set. Use: claude-rig use <name>")
	} else {
		fmt.Println(active)
	}
	return nil
}

// cmdDelete removes a profile directory.
func cmdDelete(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig delete <name>")
	}
	name := args[0]

	dir, err := profileDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", name)
	}

	fmt.Printf("Delete profile %q and all its settings/skills/plugins? [y/N] ", name)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled.")
		return nil
	}

	// Only remove real files, not symlink targets
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing profile: %w", err)
	}

	// Clear active if this was it
	if getActiveProfile() == name {
		file, _ := activeProfileFile()
		os.Remove(file)
	}

	fmt.Printf("Deleted profile %q\n", name)
	return nil
}

// cmdLaunch starts Claude Code with the specified profile's config dir.
func cmdLaunch(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig launch <name> [claude-args...]")
	}
	name := args[0]
	extraArgs := args[1:]

	dir, err := profileDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", name)
	}

	// Refresh shared symlinks in case new files appeared in ~/.claude/
	if err := syncSharedSymlinks(dir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not sync shared symlinks: %v\n", err)
	}

	// Set active profile marker
	file, _ := activeProfileFile()
	os.WriteFile(file, []byte(name), 0644)

	binary := claudeCodeBinary()
	binPath, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("claude binary not found: %w (set CLAUDE_BINARY to override)", err)
	}

	// Replace this process with claude, passing the config dir via env
	env := os.Environ()
	env = setEnv(env, "CLAUDE_CONFIG_DIR", dir)

	execArgs := append([]string{binary}, extraArgs...)
	return syscall.Exec(binPath, execArgs, env)
}

// --- helpers ---

func linkAuthFiles(profileDir string) error {
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
		linkPath := filepath.Join(profileDir, item)
		if _, err := os.Lstat(linkPath); err == nil {
			continue
		}
		if err := os.Symlink(target, linkPath); err != nil {
			return fmt.Errorf("symlinking %s: %w", item, err)
		}
	}
	return nil
}

func syncSharedSymlinks(profileDir string) error {
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

		// Skip profile-specific items and hidden files
		if isProfileSpecific(name) || strings.HasPrefix(name, ".") {
			continue
		}

		linkPath := filepath.Join(profileDir, name)
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

func getActiveProfile() string {
	file, err := activeProfileFile()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func validateProfileName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	if strings.ContainsAny(name, "/\\. ") {
		return fmt.Errorf("profile name cannot contain slashes, dots, or spaces")
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("profile name cannot start with a dash")
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
