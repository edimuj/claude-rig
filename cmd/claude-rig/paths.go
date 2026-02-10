package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// profileSpecificItems are directories/files that are unique per profile.
// Everything else in ~/.claude/ is shared via symlinks.
var profileSpecificItems = []string{
	"settings.json",
	"skills",
	"plugins",
	"agents",
	"commands",
	"hooks",
	"mcp.json",
}

// authItems are files/directories needed to skip onboarding and reuse existing auth.
var authItems = []string{
	".credentials.json",
	"statsig",
}

// sharedItems are explicitly symlinked from the canonical ~/.claude/ into each profile.
// This list is discovered dynamically at profile creation time — anything in ~/.claude/
// that is NOT profile-specific gets symlinked.

func claudeHome() (string, error) {
	if env := os.Getenv("CLAUDE_CONFIG_DIR"); env != "" {
		// If user already overrides config dir, respect that as the canonical home
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

func rigHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude-rig"), nil
}

func profilesRoot() (string, error) {
	rig, err := rigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(rig, "profiles"), nil
}

func profileDir(name string) (string, error) {
	root, err := profilesRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}

func activeProfileFile() (string, error) {
	rig, err := rigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(rig, ".active"), nil
}

func isProfileSpecific(name string) bool {
	for _, item := range profileSpecificItems {
		if name == item {
			return true
		}
	}
	return false
}

func claudeCodeBinary() string {
	if bin := os.Getenv("CLAUDE_BINARY"); bin != "" {
		return bin
	}
	if runtime.GOOS == "windows" {
		return "claude.exe"
	}
	return "claude"
}
