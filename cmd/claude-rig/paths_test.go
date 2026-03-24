package main

import "testing"

func TestIsRigSpecific(t *testing.T) {
	for _, name := range []string{"settings.json", "skills", "plugins", "agents", "hooks", "CLAUDE.md"} {
		if !isRigSpecific(name) {
			t.Errorf("isRigSpecific(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"history.jsonl", "conversations", "commands", ".credentials.json", "unknown", ""} {
		if isRigSpecific(name) {
			t.Errorf("isRigSpecific(%q) = true, want false", name)
		}
	}
}

func TestIsIsolatable(t *testing.T) {
	for _, name := range []string{"history.jsonl", "conversations", "projects", "cache", "sessions", "channels", "backups", "ide", "commands"} {
		if !isIsolatable(name) {
			t.Errorf("isIsolatable(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"settings.json", "skills", "unknown"} {
		if isIsolatable(name) {
			t.Errorf("isIsolatable(%q) = true, want false", name)
		}
	}
}

func TestIsInheritable(t *testing.T) {
	for _, name := range []string{"skills", "agents", "commands", "hooks"} {
		if !isInheritable(name) {
			t.Errorf("isInheritable(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"plugins", "settings.json", "unknown"} {
		if isInheritable(name) {
			t.Errorf("isInheritable(%q) = true, want false", name)
		}
	}
}
