package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

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

// cmdLinkAuth links shared auth files into an existing profile.
func cmdLinkAuth(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: claude-rig link-auth <name>")
	}
	name := args[0]

	dir, err := profileDir(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", name)
	}

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

	fmt.Printf("Profile %q now uses shared auth\n", name)
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

// cmdList shows all profiles and marks the active one.
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

	found := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		found = true
		marker := "  "
		if e.Name() == active {
			marker = "* "
		}
		fmt.Printf("%s%s\n", marker, e.Name())
	}

	if !found {
		fmt.Println("No profiles found. Run: claude-rig create <name>")
	}
	return nil
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
