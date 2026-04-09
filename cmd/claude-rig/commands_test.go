package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// setupTestHome creates a temp HOME with minimal ~/.claude/ and ~/.claude-rig/rigs/ structure.
// Returns the home dir. CLAUDE_CONFIG_DIR is cleared so path helpers use HOME.
func setupTestHome(t *testing.T) string {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	os.MkdirAll(filepath.Join(home, ".claude"), 0755)
	os.MkdirAll(filepath.Join(home, ".claude-rig", "rigs"), 0755)
	return home
}

// createRigDir creates a rig directory with the standard rig-specific subdirs.
func createRigDir(t *testing.T, home, name string) string {
	t.Helper()
	dir := filepath.Join(home, ".claude-rig", "rigs", name)
	os.MkdirAll(dir, 0755)
	for _, item := range rigSpecificItems {
		if item == "settings.json" || item == "CLAUDE.md" {
			continue // files, not dirs
		}
		os.MkdirAll(filepath.Join(dir, item), 0755)
	}
	return dir
}

// --- Tier 1: Pure functions ---

func TestValidateRigName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"myrig", false},
		{"test-rig", false},
		{"rig123", false},
		{"", true},
		{"has/slash", true},
		{"has\\back", true},
		{"has.dot", true},
		{"has space", true},
		{"-leading-dash", true},
	}
	for _, tt := range tests {
		err := validateRigName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateRigName(%q) err=%v, wantErr=%v", tt.name, err, tt.wantErr)
		}
	}
}

func TestSetEnv(t *testing.T) {
	env := []string{"FOO=bar", "PATH=/usr/bin"}

	// Add new
	env = setEnv(env, "NEW", "val")
	if env[len(env)-1] != "NEW=val" {
		t.Errorf("setEnv add: got %v", env)
	}

	// Overwrite existing
	env = setEnv(env, "FOO", "baz")
	found := false
	for _, e := range env {
		if e == "FOO=baz" {
			found = true
		}
		if e == "FOO=bar" {
			t.Error("setEnv overwrite: old value still present")
		}
	}
	if !found {
		t.Error("setEnv overwrite: new value not found")
	}
}

func TestRemoveEnv(t *testing.T) {
	env := []string{"FOO=bar", "PATH=/usr/bin", "HOME=/home/x"}

	env = removeEnv(env, "PATH")
	for _, e := range env {
		if e == "PATH=/usr/bin" {
			t.Error("removeEnv: PATH still present")
		}
	}
	if len(env) != 2 {
		t.Errorf("removeEnv: expected len 2, got %d", len(env))
	}

	// Remove missing key — no panic, same length
	env = removeEnv(env, "MISSING")
	if len(env) != 2 {
		t.Errorf("removeEnv missing: expected len 2, got %d", len(env))
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0B"},
		{500, "500B"},
		{1024, "1K"},
		{1536, "1K"}, // 1.5K truncated to int
		{1048576, "1M"},
		{1572864, "1M"}, // 1.5M truncated
		{1073741824, "1.0G"},
		{1610612736, "1.5G"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatTimeAgo(t *testing.T) {
	now := time.Now()
	tests := []struct {
		offset time.Duration
		want   string
	}{
		{10 * time.Second, "just now"},
		{1 * time.Minute, "1m ago"},
		{5 * time.Minute, "5m ago"},
		{1 * time.Hour, "1h ago"},
		{3 * time.Hour, "3h ago"},
		{24 * time.Hour, "1d ago"},
		{72 * time.Hour, "3d ago"},
	}
	for _, tt := range tests {
		got := formatTimeAgo(now.Add(-tt.offset))
		if got != tt.want {
			t.Errorf("formatTimeAgo(-%v) = %q, want %q", tt.offset, got, tt.want)
		}
	}
}

func TestSetDiff(t *testing.T) {
	tests := []struct {
		name           string
		a, b           []string
		wantA, wantB   []string
	}{
		{"identical", []string{"a", "b"}, []string{"a", "b"}, nil, nil},
		{"disjoint", []string{"a"}, []string{"b"}, []string{"a"}, []string{"b"}},
		{"overlap", []string{"a", "b", "c"}, []string{"b", "c", "d"}, []string{"a"}, []string{"d"}},
		{"empty_both", nil, nil, nil, nil},
		{"empty_a", nil, []string{"x"}, nil, []string{"x"}},
		{"empty_b", []string{"x"}, nil, []string{"x"}, nil},
	}
	for _, tt := range tests {
		onlyA, onlyB := setDiff(tt.a, tt.b)
		if !sliceEqual(onlyA, tt.wantA) || !sliceEqual(onlyB, tt.wantB) {
			t.Errorf("setDiff[%s]: got (%v, %v), want (%v, %v)", tt.name, onlyA, onlyB, tt.wantA, tt.wantB)
		}
	}
}

func TestLastLine(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"single", "single"},
		{"first\nsecond\nthird", "third"},
		{"trailing\n\n", "trailing"},
		{"", ""},
		{"  spaced  ", "spaced"},
	}
	for _, tt := range tests {
		got := lastLine(tt.input)
		if got != tt.want {
			t.Errorf("lastLine(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRigConfigMethods(t *testing.T) {
	cfg := rigConfig{
		Isolate: []string{"conversations", "history.jsonl"},
		Inherit: []string{"skills", "hooks"},
	}

	if !cfg.isIsolated("conversations") {
		t.Error("isIsolated should find conversations")
	}
	if cfg.isIsolated("projects") {
		t.Error("isIsolated should not find projects")
	}
	if !cfg.isInherited("skills") {
		t.Error("isInherited should find skills")
	}
	if cfg.isInherited("agents") {
		t.Error("isInherited should not find agents")
	}

	// Empty config
	empty := rigConfig{}
	if empty.isIsolated("anything") {
		t.Error("empty config should not isolate anything")
	}
	if empty.isInherited("anything") {
		t.Error("empty config should not inherit anything")
	}
}

// --- Tier 2: Filesystem with temp dirs ---

func TestLoadSaveRigConfig(t *testing.T) {
	home := setupTestHome(t)
	dir := createRigDir(t, home, "testrig")

	// Missing file returns empty config
	cfg := loadRigConfig(dir)
	if len(cfg.Isolate) != 0 || len(cfg.Inherit) != 0 {
		t.Error("loadRigConfig missing file should return empty config")
	}

	// Round-trip
	cfg = rigConfig{
		Isolate: []string{"conversations"},
		Inherit: []string{"skills", "hooks"},
	}
	if err := saveRigConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	loaded := loadRigConfig(dir)
	if !sliceEqual(loaded.Isolate, cfg.Isolate) || !sliceEqual(loaded.Inherit, cfg.Inherit) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", loaded, cfg)
	}
}

func TestSyncSharedSymlinks(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "testrig")

	// Put some shared items in ~/.claude/
	os.MkdirAll(filepath.Join(claudeDir, "conversations"), 0755)
	os.WriteFile(filepath.Join(claudeDir, "history.jsonl"), []byte("{}"), 0644)

	// Also put a rig-specific item that should NOT be symlinked
	os.MkdirAll(filepath.Join(claudeDir, "skills"), 0755)

	// And a hidden file that should be skipped
	os.WriteFile(filepath.Join(claudeDir, ".hidden"), []byte("x"), 0644)

	if err := syncSharedSymlinks(rigDir); err != nil {
		t.Fatal(err)
	}

	// conversations and history.jsonl should be symlinked
	for _, name := range []string{"conversations", "history.jsonl"} {
		link := filepath.Join(rigDir, name)
		info, err := os.Lstat(link)
		if err != nil {
			t.Errorf("expected symlink for %s, got error: %v", name, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s should be a symlink", name)
		}
	}

	// skills (rig-specific) should NOT have a new symlink
	info, err := os.Lstat(filepath.Join(rigDir, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("skills should not be symlinked (rig-specific, already exists)")
	}

	// .hidden should not be symlinked
	if _, err := os.Lstat(filepath.Join(rigDir, ".hidden")); err == nil {
		t.Error(".hidden should not be symlinked")
	}
}

func TestSyncSharedSymlinksSkipsIsolated(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "testrig")

	os.MkdirAll(filepath.Join(claudeDir, "conversations"), 0755)
	os.MkdirAll(filepath.Join(claudeDir, "cache"), 0755)

	// Isolate conversations
	saveRigConfig(rigDir, rigConfig{Isolate: []string{"conversations"}})
	os.MkdirAll(filepath.Join(rigDir, "conversations"), 0755)

	if err := syncSharedSymlinks(rigDir); err != nil {
		t.Fatal(err)
	}

	// conversations should remain a real dir (not symlinked)
	info, _ := os.Lstat(filepath.Join(rigDir, "conversations"))
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("isolated conversations should not be symlinked")
	}

	// cache should be symlinked
	info, err := os.Lstat(filepath.Join(rigDir, "cache"))
	if err != nil {
		t.Fatal("cache should exist")
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("cache should be symlinked")
	}
}

func TestSyncGlobalContents(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "testrig")

	// Set up global skills
	globalSkills := filepath.Join(claudeDir, "skills")
	os.MkdirAll(globalSkills, 0755)
	os.WriteFile(filepath.Join(globalSkills, "global-skill.md"), []byte("# skill"), 0644)

	// Enable inheritance for skills
	saveRigConfig(rigDir, rigConfig{Inherit: []string{"skills"}})

	if err := syncGlobalContents(rigDir); err != nil {
		t.Fatal(err)
	}

	// Check global-skill.md is symlinked into rig's skills/
	link := filepath.Join(rigDir, "skills", "global-skill.md")
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal("global-skill.md should be symlinked into rig")
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("global-skill.md should be a symlink")
	}
	target, _ := os.Readlink(link)
	if target != filepath.Join(globalSkills, "global-skill.md") {
		t.Errorf("symlink target = %q, want %q", target, filepath.Join(globalSkills, "global-skill.md"))
	}
}

func TestSyncGlobalContentsSkipsLocalOverrides(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "testrig")

	globalSkills := filepath.Join(claudeDir, "skills")
	os.MkdirAll(globalSkills, 0755)
	os.WriteFile(filepath.Join(globalSkills, "override.md"), []byte("global"), 0644)

	// Create a local override in the rig
	os.WriteFile(filepath.Join(rigDir, "skills", "override.md"), []byte("local"), 0644)

	saveRigConfig(rigDir, rigConfig{Inherit: []string{"skills"}})
	syncGlobalContents(rigDir)

	// Local file should still be a real file, not overwritten
	info, _ := os.Lstat(filepath.Join(rigDir, "skills", "override.md"))
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("local override should not be replaced with symlink")
	}
	data, _ := os.ReadFile(filepath.Join(rigDir, "skills", "override.md"))
	if string(data) != "local" {
		t.Error("local file content should be preserved")
	}
}

func TestRemoveInheritedSymlinks(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "testrig")

	globalSkills := filepath.Join(claudeDir, "skills")
	os.MkdirAll(globalSkills, 0755)
	os.WriteFile(filepath.Join(globalSkills, "global.md"), []byte("g"), 0644)

	localSkills := filepath.Join(rigDir, "skills")

	// Create a global symlink and a local file
	os.Symlink(filepath.Join(globalSkills, "global.md"), filepath.Join(localSkills, "global.md"))
	os.WriteFile(filepath.Join(localSkills, "local.md"), []byte("l"), 0644)

	removeInheritedSymlinks(localSkills, globalSkills)

	// Global symlink should be removed
	if _, err := os.Lstat(filepath.Join(localSkills, "global.md")); err == nil {
		t.Error("global symlink should be removed")
	}
	// Local file should be preserved
	if _, err := os.Lstat(filepath.Join(localSkills, "local.md")); err != nil {
		t.Error("local file should be preserved")
	}
}

func TestRemoveStaleInheritedSymlinks(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "testrig")

	globalSkills := filepath.Join(claudeDir, "skills")
	os.MkdirAll(globalSkills, 0755)

	localSkills := filepath.Join(rigDir, "skills")

	// Create a symlink to a file that exists
	os.WriteFile(filepath.Join(globalSkills, "valid.md"), []byte("ok"), 0644)
	os.Symlink(filepath.Join(globalSkills, "valid.md"), filepath.Join(localSkills, "valid.md"))

	// Create a symlink to a file that does NOT exist (stale)
	os.Symlink(filepath.Join(globalSkills, "gone.md"), filepath.Join(localSkills, "gone.md"))

	removeStaleInheritedSymlinks(localSkills, globalSkills)

	// Valid symlink preserved
	if _, err := os.Lstat(filepath.Join(localSkills, "valid.md")); err != nil {
		t.Error("valid symlink should be preserved")
	}
	// Stale symlink removed
	if _, err := os.Lstat(filepath.Join(localSkills, "gone.md")); err == nil {
		t.Error("stale symlink should be removed")
	}
}

func TestDirEntryNames(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte(""), 0644)
	os.MkdirAll(filepath.Join(dir, "c"), 0755)

	names := dirEntryNames(dir)
	sort.Strings(names)
	want := []string{"a.txt", "b.txt", "c"}
	if !sliceEqual(names, want) {
		t.Errorf("dirEntryNames = %v, want %v", names, want)
	}

	// Missing dir returns nil
	if got := dirEntryNames("/nonexistent/path"); got != nil {
		t.Errorf("dirEntryNames missing dir = %v, want nil", got)
	}
}

// --- Tier 3: Integration workflows ---

func TestCreateInheritSyncUninherit(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "workflow")

	// Set up global skills and agents
	for _, item := range []string{"skills", "agents"} {
		d := filepath.Join(claudeDir, item)
		os.MkdirAll(d, 0755)
		os.WriteFile(filepath.Join(d, "global-"+item+".md"), []byte("content"), 0644)
	}

	// Inherit all
	cfg := rigConfig{Inherit: []string{"skills", "agents", "commands", "hooks"}}
	saveRigConfig(rigDir, cfg)
	syncGlobalContents(rigDir)

	// Verify global skills/agents symlinked
	for _, item := range []string{"skills", "agents"} {
		link := filepath.Join(rigDir, item, "global-"+item+".md")
		info, err := os.Lstat(link)
		if err != nil {
			t.Fatalf("expected symlink for global-%s.md: %v", item, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("global-%s.md should be a symlink", item)
		}
	}

	// Uninherit skills
	cfg.Inherit = []string{"agents", "commands", "hooks"}
	saveRigConfig(rigDir, cfg)
	removeInheritedSymlinks(
		filepath.Join(rigDir, "skills"),
		filepath.Join(claudeDir, "skills"),
	)

	// Skills global symlink removed
	if _, err := os.Lstat(filepath.Join(rigDir, "skills", "global-skills.md")); err == nil {
		t.Error("global-skills.md should be removed after uninherit")
	}
	// Agents still inherited
	if _, err := os.Lstat(filepath.Join(rigDir, "agents", "global-agents.md")); err != nil {
		t.Error("global-agents.md should still exist")
	}
}

func TestLaunchSyncPicksUpNewGlobalSkill(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "synctest")

	globalSkills := filepath.Join(claudeDir, "skills")
	os.MkdirAll(globalSkills, 0755)
	os.WriteFile(filepath.Join(globalSkills, "existing.md"), []byte("e"), 0644)

	cfg := rigConfig{Inherit: []string{"skills"}}
	saveRigConfig(rigDir, cfg)
	syncGlobalContents(rigDir)

	// Verify existing skill is synced
	if _, err := os.Lstat(filepath.Join(rigDir, "skills", "existing.md")); err != nil {
		t.Fatal("existing.md should be synced")
	}

	// Add a new global skill
	os.WriteFile(filepath.Join(globalSkills, "new-skill.md"), []byte("n"), 0644)

	// Re-sync (simulates launch sync)
	syncGlobalContents(rigDir)

	// New skill should now appear
	info, err := os.Lstat(filepath.Join(rigDir, "skills", "new-skill.md"))
	if err != nil {
		t.Fatal("new-skill.md should be synced after re-sync")
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("new-skill.md should be a symlink")
	}
}

func TestStaleSymlinkCleanup(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "staletest")

	globalSkills := filepath.Join(claudeDir, "skills")
	os.MkdirAll(globalSkills, 0755)
	os.WriteFile(filepath.Join(globalSkills, "temp.md"), []byte("t"), 0644)

	cfg := rigConfig{Inherit: []string{"skills"}}
	saveRigConfig(rigDir, cfg)
	syncGlobalContents(rigDir)

	// Verify synced
	if _, err := os.Lstat(filepath.Join(rigDir, "skills", "temp.md")); err != nil {
		t.Fatal("temp.md should be synced")
	}

	// Remove the global source
	os.Remove(filepath.Join(globalSkills, "temp.md"))

	// Re-sync — stale cleanup happens inside syncGlobalContents
	syncGlobalContents(rigDir)

	// Stale symlink should be cleaned up
	if _, err := os.Lstat(filepath.Join(rigDir, "skills", "temp.md")); err == nil {
		t.Error("stale temp.md symlink should be removed after source deleted")
	}
}

// --- helpers ---

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- Default settings tests ---

func TestParseDotPath(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"theme", []string{"theme"}},
		{"env.FOO", []string{"env", "FOO"}},
		{"a.b.c", []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		got := parseDotPath(tt.input)
		if !sliceEqual(got, tt.want) {
			t.Errorf("parseDotPath(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSetGetNestedValue(t *testing.T) {
	m := make(map[string]any)

	// Set a deeply nested value
	setNestedValue(m, []string{"a", "b", "c"}, "deep")
	val, ok := getNestedValue(m, []string{"a", "b", "c"})
	if !ok || val != "deep" {
		t.Errorf("getNestedValue after set: got %v, %v", val, ok)
	}

	// Set a top-level value
	setNestedValue(m, []string{"top"}, 42.0)
	val, ok = getNestedValue(m, []string{"top"})
	if !ok || val != 42.0 {
		t.Errorf("top-level get: got %v, %v", val, ok)
	}

	// Overwrite non-map with map path
	setNestedValue(m, []string{"top", "nested"}, "v")
	val, ok = getNestedValue(m, []string{"top", "nested"})
	if !ok || val != "v" {
		t.Errorf("overwrite non-map: got %v, %v", val, ok)
	}

	// Get missing key
	_, ok = getNestedValue(m, []string{"missing"})
	if ok {
		t.Error("expected false for missing key")
	}
}

func TestDeleteNestedValue(t *testing.T) {
	m := map[string]any{
		"a": map[string]any{
			"b": "val",
			"c": "other",
		},
		"top": "x",
	}

	// Delete leaf, parent stays because it has other keys
	if !deleteNestedValue(m, []string{"a", "b"}) {
		t.Error("expected true for existing key")
	}
	if _, ok := getNestedValue(m, []string{"a", "b"}); ok {
		t.Error("key should be deleted")
	}
	if _, ok := getNestedValue(m, []string{"a", "c"}); !ok {
		t.Error("sibling should remain")
	}

	// Delete last child — parent should be cleaned up
	deleteNestedValue(m, []string{"a", "c"})
	if _, ok := m["a"]; ok {
		t.Error("empty parent map should be cleaned up")
	}

	// Delete top-level
	if !deleteNestedValue(m, []string{"top"}) {
		t.Error("expected true for top-level key")
	}

	// Delete missing
	if deleteNestedValue(m, []string{"nope"}) {
		t.Error("expected false for missing key")
	}
}

func TestFlattenKeys(t *testing.T) {
	m := map[string]any{
		"theme": "dark",
		"env": map[string]any{
			"FOO": "bar",
			"BAZ": "qux",
		},
		"list": []any{1, 2, 3},
	}
	got := flattenKeys(m, "")
	want := []string{"env.BAZ", "env.FOO", "list", "theme"}
	if !sliceEqual(got, want) {
		t.Errorf("flattenKeys = %v, want %v", got, want)
	}
}

func TestSyncDefaultSettingsApplies(t *testing.T) {
	home := setupTestHome(t)
	dir := createRigDir(t, home, "test")
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "rig.json"), []byte("{}\n"), 0644)

	// Write defaults
	defaults := map[string]any{"theme": "dark", "env": map[string]any{"FOO": "bar"}}
	saveDefaultSettings(defaults)

	applied, err := syncDefaultSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 2 {
		t.Errorf("expected 2 applied, got %d: %v", len(applied), applied)
	}

	// Verify settings.json
	settings, err := readJSONFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if settings["theme"] != "dark" {
		t.Errorf("theme = %v, want dark", settings["theme"])
	}
	env, ok := settings["env"].(map[string]any)
	if !ok || env["FOO"] != "bar" {
		t.Errorf("env.FOO = %v, want bar", settings["env"])
	}
}

func TestSyncDefaultSettingsSkipsOverrides(t *testing.T) {
	home := setupTestHome(t)
	dir := createRigDir(t, home, "test")
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"theme":"light"}`+"\n"), 0644)

	cfg := rigConfig{SettingsOverrides: []string{"theme"}}
	saveRigConfig(dir, cfg)

	defaults := map[string]any{"theme": "dark", "other": true}
	saveDefaultSettings(defaults)

	applied, err := syncDefaultSettings(dir)
	if err != nil {
		t.Fatal(err)
	}

	// "theme" should be skipped, "other" applied
	for _, a := range applied {
		if a == "theme" {
			t.Error("theme should have been skipped (overridden)")
		}
	}

	settings, _ := readJSONFile(filepath.Join(dir, "settings.json"))
	if settings["theme"] != "light" {
		t.Errorf("theme = %v, want light (override)", settings["theme"])
	}
	if settings["other"] != true {
		t.Errorf("other = %v, want true", settings["other"])
	}
}

func TestSyncDefaultSettingsSkipsIsolated(t *testing.T) {
	home := setupTestHome(t)
	dir := createRigDir(t, home, "test")
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}\n"), 0644)

	cfg := rigConfig{Isolate: []string{"settings"}}
	saveRigConfig(dir, cfg)

	defaults := map[string]any{"theme": "dark"}
	saveDefaultSettings(defaults)

	applied, err := syncDefaultSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 0 {
		t.Errorf("expected 0 applied for isolated settings, got %d", len(applied))
	}

	// settings.json should be unchanged
	settings, _ := readJSONFile(filepath.Join(dir, "settings.json"))
	if _, ok := settings["theme"]; ok {
		t.Error("theme should not be set in isolated rig")
	}
}

func TestSyncDefaultSettingsNestedMerge(t *testing.T) {
	home := setupTestHome(t)
	dir := createRigDir(t, home, "test")
	// Rig has existing env.OTHER
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"env":{"OTHER":"keep"}}`+"\n"), 0644)
	os.WriteFile(filepath.Join(dir, "rig.json"), []byte("{}\n"), 0644)

	defaults := map[string]any{"env": map[string]any{"FOO": "bar"}}
	saveDefaultSettings(defaults)

	_, err := syncDefaultSettings(dir)
	if err != nil {
		t.Fatal(err)
	}

	settings, _ := readJSONFile(filepath.Join(dir, "settings.json"))
	env, ok := settings["env"].(map[string]any)
	if !ok {
		t.Fatal("env should be a map")
	}
	if env["FOO"] != "bar" {
		t.Errorf("env.FOO = %v, want bar", env["FOO"])
	}
	if env["OTHER"] != "keep" {
		t.Errorf("env.OTHER = %v, want keep (should not be clobbered)", env["OTHER"])
	}
}

func TestRemoveDefaultSettingFromRig(t *testing.T) {
	home := setupTestHome(t)
	dir := createRigDir(t, home, "test")
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"theme":"dark","other":"x"}`+"\n"), 0644)
	os.WriteFile(filepath.Join(dir, "rig.json"), []byte("{}\n"), 0644)

	if err := removeDefaultSettingFromRig(dir, "theme"); err != nil {
		t.Fatal(err)
	}

	settings, _ := readJSONFile(filepath.Join(dir, "settings.json"))
	if _, ok := settings["theme"]; ok {
		t.Error("theme should be removed")
	}
	if settings["other"] != "x" {
		t.Error("other should remain")
	}
}

func TestRemoveDefaultSettingSkipsOverride(t *testing.T) {
	home := setupTestHome(t)
	dir := createRigDir(t, home, "test")
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"theme":"light"}`+"\n"), 0644)

	cfg := rigConfig{SettingsOverrides: []string{"theme"}}
	saveRigConfig(dir, cfg)

	if err := removeDefaultSettingFromRig(dir, "theme"); err != nil {
		t.Fatal(err)
	}

	settings, _ := readJSONFile(filepath.Join(dir, "settings.json"))
	if settings["theme"] != "light" {
		t.Errorf("theme = %v, want light (should be preserved by override)", settings["theme"])
	}
}

func TestParseJSONValue(t *testing.T) {
	tests := []struct {
		input string
		want  any
	}{
		{`true`, true},
		{`false`, false},
		{`42`, 42.0},
		{`"hello"`, "hello"},
		{`plain string`, "plain string"},
		{`[1,2,3]`, []any{1.0, 2.0, 3.0}},
	}
	for _, tt := range tests {
		got := parseJSONValue(tt.input)
		switch want := tt.want.(type) {
		case []any:
			gotSlice, ok := got.([]any)
			if !ok || len(gotSlice) != len(want) {
				t.Errorf("parseJSONValue(%q) = %v, want %v", tt.input, got, tt.want)
			}
		default:
			if got != tt.want {
				t.Errorf("parseJSONValue(%q) = %v, want %v", tt.input, got, tt.want)
			}
		}
	}
}

// --- Blueprint tests ---

// createTestRig sets up a rig with plugins, MCP, settings, skills, and isolation for blueprint testing.
func createTestRig(t *testing.T, home, name string) string {
	t.Helper()
	dir := createRigDir(t, home, name)

	// rig.json with isolation and inherit
	cfg := rigConfig{
		Isolate: []string{"conversations", "sessions"},
		Inherit: []string{"skills"},
		PluginMCP: map[string]string{
			"plugin-server": "test-plugin@marketplace",
		},
	}
	saveRigConfig(dir, cfg)

	// settings.json
	settings := map[string]any{"theme": "dark", "env": map[string]any{"GO111MODULE": "on"}}
	writeJSONFile(filepath.Join(dir, "settings.json"), settings)

	// .claude.json with MCP servers (manual + plugin-provided)
	claudeJSON := map[string]any{
		"mcpServers": map[string]any{
			"gopls":         map[string]any{"command": "gopls-mcp", "args": []any{"--stdio"}},
			"plugin-server": map[string]any{"command": "plugin-cmd"},
		},
	}
	writeJSONFile(filepath.Join(dir, ".claude.json"), claudeJSON)

	// CLAUDE.md
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Test Rig\nGo development rig\n"), 0644)

	// Real skill file
	os.WriteFile(filepath.Join(dir, "skills", "review.md"), []byte("# Code Review Skill\n"), 0644)

	// Inherited symlink in skills (should be skipped by blueprint create)
	globalSkills := filepath.Join(home, ".claude", "skills")
	os.MkdirAll(globalSkills, 0755)
	os.WriteFile(filepath.Join(globalSkills, "global-skill.md"), []byte("global"), 0644)
	os.Symlink(filepath.Join(globalSkills, "global-skill.md"), filepath.Join(dir, "skills", "global-skill.md"))

	// Real hook file
	os.WriteFile(filepath.Join(dir, "hooks", "pre-commit.sh"), []byte("#!/bin/bash\necho ok\n"), 0755)

	// default-args
	os.WriteFile(filepath.Join(dir, "default-args"), []byte("--dangerously-skip-permissions\n"), 0644)

	// installed_plugins.json
	pluginsDir := filepath.Join(dir, "plugins")
	manifest := pluginManifest{
		Version: 2,
		Plugins: map[string][]pluginEntry{
			"npm:@anthropic/github": {{Scope: "global", InstallPath: "/tmp/fake", Version: "1.0.0"}},
			"npm:edimuj/my-plugin":  {{Scope: "global", InstallPath: "/tmp/fake2", Version: "0.5.0"}},
		},
	}
	writePluginManifest(filepath.Join(pluginsDir, "installed_plugins.json"), manifest)

	return dir
}

func TestBlueprintCreateFromRig(t *testing.T) {
	home := setupTestHome(t)
	createTestRig(t, home, "testrig")

	rigPath, _ := rigDir("testrig")
	t.Setenv("CLAUDE_CONFIG_DIR", rigPath)

	err := cmdBlueprintCreate([]string{"test-bp"})
	if err != nil {
		t.Fatal(err)
	}

	bpDir, _ := blueprintDir("test-bp")
	bp, err := loadBlueprint(bpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Check name
	if bp.Name != "test-bp" {
		t.Errorf("name = %q, want %q", bp.Name, "test-bp")
	}

	// Check plugins captured
	sort.Strings(bp.Plugins)
	if len(bp.Plugins) != 2 {
		t.Errorf("plugins = %v, want 2", bp.Plugins)
	}

	// Check MCP — should only have gopls, not plugin-server
	if len(bp.MCPServers) != 1 {
		t.Errorf("mcp_servers = %v, want 1 (gopls only)", bp.MCPServers)
	}
	if _, ok := bp.MCPServers["gopls"]; !ok {
		t.Errorf("mcp_servers missing gopls")
	}
	if _, ok := bp.MCPServers["plugin-server"]; ok {
		t.Errorf("mcp_servers should not contain plugin-provided server")
	}

	// Check settings
	if bp.Settings["theme"] != "dark" {
		t.Errorf("settings.theme = %v, want dark", bp.Settings["theme"])
	}

	// Check isolation
	if len(bp.Isolation) != 2 {
		t.Errorf("isolation = %v, want 2", bp.Isolation)
	}

	// Check inherit
	if len(bp.Inherit) != 1 || bp.Inherit[0] != "skills" {
		t.Errorf("inherit = %v, want [skills]", bp.Inherit)
	}

	// Check args
	if bp.Args != "--dangerously-skip-permissions" {
		t.Errorf("args = %q, want --dangerously-skip-permissions", bp.Args)
	}

	// Check CLAUDE.md was copied
	data, err := os.ReadFile(filepath.Join(bpDir, "CLAUDE.md"))
	if err != nil {
		t.Errorf("CLAUDE.md not copied: %v", err)
	} else if string(data) != "# Test Rig\nGo development rig\n" {
		t.Errorf("CLAUDE.md content wrong: %q", string(data))
	}

	// Check skills — only real file, not symlink
	entries, _ := os.ReadDir(filepath.Join(bpDir, "skills"))
	if len(entries) != 1 {
		t.Errorf("skills has %d files, want 1 (review.md only)", len(entries))
	} else if entries[0].Name() != "review.md" {
		t.Errorf("skills[0] = %q, want review.md", entries[0].Name())
	}

	// Check hooks
	entries, _ = os.ReadDir(filepath.Join(bpDir, "hooks"))
	if len(entries) != 1 {
		t.Errorf("hooks has %d files, want 1", len(entries))
	}
}

func TestBlueprintApplyCreatesRig(t *testing.T) {
	home := setupTestHome(t)

	// Create a blueprint directory manually
	bpDir := filepath.Join(home, "test-blueprint")
	os.MkdirAll(bpDir, 0755)
	os.MkdirAll(filepath.Join(bpDir, "skills"), 0755)

	bp := blueprint{
		Name:      "applied-rig",
		Settings:  map[string]any{"theme": "light", "verbose": true},
		Isolation: []string{"conversations", "sessions", "history.jsonl"},
		Inherit:   []string{"skills"},
		Args:      "--verbose",
		MCPServers: map[string]any{
			"test-mcp": map[string]any{"command": "test-cmd"},
		},
	}
	data, _ := json.MarshalIndent(bp, "", "  ")
	os.WriteFile(filepath.Join(bpDir, "blueprint.json"), data, 0644)
	os.WriteFile(filepath.Join(bpDir, "CLAUDE.md"), []byte("# Applied\n"), 0644)
	os.WriteFile(filepath.Join(bpDir, "skills", "test-skill.md"), []byte("# Skill\n"), 0644)

	err := cmdBlueprintApply([]string{bpDir, "--skip-plugins"})
	if err != nil {
		t.Fatal(err)
	}

	// Verify rig was created
	dir, _ := rigDir("applied-rig")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("rig directory not created")
	}

	// Check rig.json isolation
	cfg := loadRigConfig(dir)
	if len(cfg.Isolate) != 3 {
		t.Errorf("isolate = %v, want 3 items", cfg.Isolate)
	}
	if len(cfg.Inherit) != 1 || cfg.Inherit[0] != "skills" {
		t.Errorf("inherit = %v, want [skills]", cfg.Inherit)
	}

	// Check settings.json
	settings, err := readJSONFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if settings["theme"] != "light" {
		t.Errorf("settings.theme = %v, want light", settings["theme"])
	}
	if settings["verbose"] != true {
		t.Errorf("settings.verbose = %v, want true", settings["verbose"])
	}

	// Check .claude.json for MCP servers
	claudeData, err := readJSONFile(filepath.Join(dir, ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	servers, _ := claudeData["mcpServers"].(map[string]any)
	if _, ok := servers["test-mcp"]; !ok {
		t.Errorf("MCP server test-mcp not found in .claude.json")
	}

	// Check CLAUDE.md
	mdData, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil || string(mdData) != "# Applied\n" {
		t.Errorf("CLAUDE.md not applied correctly")
	}

	// Check skills
	entries, _ := os.ReadDir(filepath.Join(dir, "skills"))
	found := false
	for _, e := range entries {
		if e.Name() == "test-skill.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("test-skill.md not found in rig skills")
	}

	// Check args
	argsData, err := os.ReadFile(filepath.Join(dir, "default-args"))
	if err != nil || string(argsData) != "--verbose\n" {
		t.Errorf("default-args = %q, want --verbose", string(argsData))
	}
}

func TestBlueprintRoundTrip(t *testing.T) {
	home := setupTestHome(t)
	createTestRig(t, home, "source-rig")

	rigPath, _ := rigDir("source-rig")
	t.Setenv("CLAUDE_CONFIG_DIR", rigPath)

	// Create blueprint from rig
	if err := cmdBlueprintCreate([]string{"roundtrip-bp"}); err != nil {
		t.Fatal(err)
	}

	// Apply blueprint to create new rig
	bpDir, _ := blueprintDir("roundtrip-bp")
	if err := cmdBlueprintApply([]string{bpDir, "--as", "dest-rig", "--skip-plugins"}); err != nil {
		t.Fatal(err)
	}

	destDir, _ := rigDir("dest-rig")

	// Compare settings
	srcSettings, _ := readJSONFile(filepath.Join(rigPath, "settings.json"))
	dstSettings, _ := readJSONFile(filepath.Join(destDir, "settings.json"))
	if srcSettings["theme"] != dstSettings["theme"] {
		t.Errorf("theme mismatch: src=%v dst=%v", srcSettings["theme"], dstSettings["theme"])
	}

	// Compare isolation
	srcCfg := loadRigConfig(rigPath)
	dstCfg := loadRigConfig(destDir)
	if len(srcCfg.Isolate) != len(dstCfg.Isolate) {
		t.Errorf("isolation count mismatch: src=%d dst=%d", len(srcCfg.Isolate), len(dstCfg.Isolate))
	}

	// Compare inherit
	if len(srcCfg.Inherit) != len(dstCfg.Inherit) {
		t.Errorf("inherit count mismatch: src=%d dst=%d", len(srcCfg.Inherit), len(dstCfg.Inherit))
	}

	// Compare CLAUDE.md
	srcMD, _ := os.ReadFile(filepath.Join(rigPath, "CLAUDE.md"))
	dstMD, _ := os.ReadFile(filepath.Join(destDir, "CLAUDE.md"))
	if string(srcMD) != string(dstMD) {
		t.Errorf("CLAUDE.md mismatch")
	}

	// Compare skills (only real files)
	srcSkills, _ := os.ReadDir(filepath.Join(rigPath, "skills"))
	dstSkills, _ := os.ReadDir(filepath.Join(destDir, "skills"))
	// Filter out symlinks from source for comparison
	realSrc := 0
	for _, e := range srcSkills {
		info, _ := os.Lstat(filepath.Join(rigPath, "skills", e.Name()))
		if info != nil && info.Mode()&os.ModeSymlink == 0 {
			realSrc++
		}
	}
	// Dest may have inherited symlinks too, count non-symlinks
	realDst := 0
	for _, e := range dstSkills {
		info, _ := os.Lstat(filepath.Join(destDir, "skills", e.Name()))
		if info != nil && info.Mode()&os.ModeSymlink == 0 {
			realDst++
		}
	}
	if realSrc != realDst {
		t.Errorf("real skill files mismatch: src=%d dst=%d", realSrc, realDst)
	}
}

func TestBlueprintSkipsSymlinks(t *testing.T) {
	home := setupTestHome(t)
	dir := createRigDir(t, home, "symlink-test")
	os.WriteFile(filepath.Join(dir, "rig.json"), []byte("{}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}\n"), 0644)

	// Create a real file and a symlink in skills
	os.WriteFile(filepath.Join(dir, "skills", "real.md"), []byte("real"), 0644)
	globalSkills := filepath.Join(home, ".claude", "skills")
	os.MkdirAll(globalSkills, 0755)
	os.WriteFile(filepath.Join(globalSkills, "inherited.md"), []byte("inherited"), 0644)
	os.Symlink(filepath.Join(globalSkills, "inherited.md"), filepath.Join(dir, "skills", "inherited.md"))

	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	err := cmdBlueprintCreate([]string{"skip-test"})
	if err != nil {
		t.Fatal(err)
	}

	bpDir, _ := blueprintDir("skip-test")
	entries, _ := os.ReadDir(filepath.Join(bpDir, "skills"))
	if len(entries) != 1 {
		t.Errorf("blueprint skills has %d files, want 1 (real only)", len(entries))
	}
	if len(entries) > 0 && entries[0].Name() != "real.md" {
		t.Errorf("skills[0] = %q, want real.md", entries[0].Name())
	}
}

func TestBlueprintApplyWithAsName(t *testing.T) {
	home := setupTestHome(t)

	bpDir := filepath.Join(home, "bp")
	os.MkdirAll(bpDir, 0755)
	bp := blueprint{Name: "original-name"}
	data, _ := json.MarshalIndent(bp, "", "  ")
	os.WriteFile(filepath.Join(bpDir, "blueprint.json"), data, 0644)

	err := cmdBlueprintApply([]string{bpDir, "--as", "custom-name", "--skip-plugins"})
	if err != nil {
		t.Fatal(err)
	}

	dir, _ := rigDir("custom-name")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("rig 'custom-name' not created")
	}
	// Original name rig should NOT exist
	dir2, _ := rigDir("original-name")
	if _, err := os.Stat(dir2); !os.IsNotExist(err) {
		t.Fatal("rig 'original-name' should not exist when --as is used")
	}
}

func TestBlueprintList(t *testing.T) {
	setupTestHome(t)

	// Create two blueprints
	root, _ := blueprintsRoot()
	os.MkdirAll(root, 0755)
	for _, name := range []string{"bp-alpha", "bp-beta"} {
		dir := filepath.Join(root, name)
		os.MkdirAll(dir, 0755)
		bp := blueprint{Name: name, Description: "Test " + name}
		saveBlueprint(dir, bp)
	}

	// Should not panic
	err := cmdBlueprintList()
	if err != nil {
		t.Fatal(err)
	}
}

func TestResolveBlueprintLocal(t *testing.T) {
	home := setupTestHome(t)

	// 1. Direct directory
	dirBP := filepath.Join(home, "direct-bp")
	os.MkdirAll(dirBP, 0755)
	os.WriteFile(filepath.Join(dirBP, "blueprint.json"), []byte(`{"name":"direct"}`), 0644)

	resolved, cleanup, err := resolveBlueprint(dirBP)
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	if resolved != dirBP {
		t.Errorf("resolved = %q, want %q", resolved, dirBP)
	}

	// 2. Library lookup
	root, _ := blueprintsRoot()
	os.MkdirAll(root, 0755)
	libDir := filepath.Join(root, "my-lib-bp")
	os.MkdirAll(libDir, 0755)
	os.WriteFile(filepath.Join(libDir, "blueprint.json"), []byte(`{"name":"my-lib-bp"}`), 0644)

	resolved, cleanup, err = resolveBlueprint("my-lib-bp")
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	if resolved != libDir {
		t.Errorf("library resolved = %q, want %q", resolved, libDir)
	}

	// 3. Non-existent should fail
	_, cleanup, err = resolveBlueprint("nonexistent-bp-xyz")
	if err == nil {
		cleanup()
		t.Fatal("expected error for nonexistent blueprint")
	}
}

func TestBlueprintApplyDefaultIsolation(t *testing.T) {
	home := setupTestHome(t)

	// Blueprint without isolation specified — should get defaults
	bpDir := filepath.Join(home, "no-iso-bp")
	os.MkdirAll(bpDir, 0755)
	bp := blueprint{Name: "no-iso-rig"}
	data, _ := json.MarshalIndent(bp, "", "  ")
	os.WriteFile(filepath.Join(bpDir, "blueprint.json"), data, 0644)

	err := cmdBlueprintApply([]string{bpDir, "--skip-plugins"})
	if err != nil {
		t.Fatal(err)
	}

	dir, _ := rigDir("no-iso-rig")
	cfg := loadRigConfig(dir)
	if len(cfg.Isolate) != len(defaultIsolatedItems) {
		t.Errorf("isolation = %d items, want %d (defaults)", len(cfg.Isolate), len(defaultIsolatedItems))
	}
}

func TestCopyRealFiles(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	os.MkdirAll(src, 0755)

	// Real file
	os.WriteFile(filepath.Join(src, "real.txt"), []byte("content"), 0644)

	// Symlink
	target := filepath.Join(tmp, "target.txt")
	os.WriteFile(target, []byte("linked"), 0644)
	os.Symlink(target, filepath.Join(src, "link.txt"))

	err := copyRealFiles(src, dst)
	if err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dst)
	if len(entries) != 1 {
		t.Errorf("copied %d files, want 1", len(entries))
	}
	if len(entries) > 0 && entries[0].Name() != "real.txt" {
		t.Errorf("copied file = %q, want real.txt", entries[0].Name())
	}

	// Source dir doesn't exist — no error
	err = copyRealFiles(filepath.Join(tmp, "nonexistent"), filepath.Join(tmp, "dst2"))
	if err != nil {
		t.Errorf("expected no error for missing src dir, got %v", err)
	}
}

func TestLoadSaveBlueprint(t *testing.T) {
	tmp := t.TempDir()

	bp := blueprint{
		Name:        "test",
		Description: "Test blueprint",
		Version:     "1",
		Plugins:     []string{"npm:test/plugin"},
		MCPServers:  map[string]any{"srv": map[string]any{"command": "cmd"}},
		Settings:    map[string]any{"key": "value"},
		Isolation:   []string{"conversations"},
		Inherit:     []string{"skills"},
		Args:        "--verbose",
	}

	err := saveBlueprint(tmp, bp)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := loadBlueprint(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Name != bp.Name {
		t.Errorf("name = %q, want %q", loaded.Name, bp.Name)
	}
	if loaded.Description != bp.Description {
		t.Errorf("description = %q, want %q", loaded.Description, bp.Description)
	}
	if len(loaded.Plugins) != 1 || loaded.Plugins[0] != "npm:test/plugin" {
		t.Errorf("plugins = %v", loaded.Plugins)
	}
	if loaded.Args != "--verbose" {
		t.Errorf("args = %q, want --verbose", loaded.Args)
	}
}

func TestBlueprintCreateFromFlag(t *testing.T) {
	home := setupTestHome(t)
	createTestRig(t, home, "from-rig")

	// Don't set CLAUDE_CONFIG_DIR — use --from instead
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	err := cmdBlueprintCreate([]string{"from-test", "--from", "from-rig"})
	if err != nil {
		t.Fatal(err)
	}

	bpDir, _ := blueprintDir("from-test")
	bp, err := loadBlueprint(bpDir)
	if err != nil {
		t.Fatal(err)
	}
	if bp.Name != "from-test" {
		t.Errorf("name = %q, want from-test", bp.Name)
	}
	if len(bp.Plugins) != 2 {
		t.Errorf("plugins = %v, want 2", bp.Plugins)
	}
}

func TestSyncPluginsDetectsVersionDrift(t *testing.T) {
	home := setupTestHome(t)
	rigDir := createRigDir(t, home, "testrig")

	// Set up source plugins dir with a plugin at v1
	sourceDir := filepath.Join(home, ".claude", "plugins")
	v1Cache := filepath.Join(sourceDir, "cache", "market", "myplugin", "v1.0.0")
	os.MkdirAll(v1Cache, 0755)
	os.WriteFile(filepath.Join(v1Cache, "plugin.json"), []byte(`{}`), 0644)

	srcManifest := pluginManifest{
		Version: 2,
		Plugins: map[string][]pluginEntry{
			"myplugin@market": {{
				Scope:        "user",
				InstallPath:  v1Cache,
				Version:      "1.0.0",
				GitCommitSha: "aaa111",
			}},
		},
	}
	writePluginManifest(filepath.Join(sourceDir, "installed_plugins.json"), srcManifest)

	// First sync — plugin gets added
	synced, err := syncPlugins(rigDir, sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(synced) != 1 || synced[0] != "myplugin@market" {
		t.Fatalf("first sync: want [myplugin@market], got %v", synced)
	}

	// Verify v1 symlink exists in target
	tgtV1 := filepath.Join(rigDir, "plugins", "cache", "market", "myplugin", "v1.0.0")
	if _, err := os.Lstat(tgtV1); err != nil {
		t.Fatalf("v1 symlink missing after first sync: %v", err)
	}

	// Source updates to v2
	v2Cache := filepath.Join(sourceDir, "cache", "market", "myplugin", "v2.0.0")
	os.MkdirAll(v2Cache, 0755)
	os.WriteFile(filepath.Join(v2Cache, "plugin.json"), []byte(`{}`), 0644)

	srcManifest.Plugins["myplugin@market"] = []pluginEntry{{
		Scope:        "user",
		InstallPath:  v2Cache,
		Version:      "2.0.0",
		GitCommitSha: "bbb222",
	}}
	writePluginManifest(filepath.Join(sourceDir, "installed_plugins.json"), srcManifest)

	// Second sync — should detect version drift and update
	synced2, _ := syncPlugins(rigDir, sourceDir)
	// No NEW plugins synced (already registered), but manifest should be updated
	if len(synced2) != 0 {
		t.Errorf("second sync: want no new synced, got %v", synced2)
	}

	// Target manifest should now have v2
	tgtManifest, err := readPluginManifest(filepath.Join(rigDir, "plugins", "installed_plugins.json"))
	if err != nil {
		t.Fatal(err)
	}
	entries := tgtManifest.Plugins["myplugin@market"]
	if len(entries) == 0 {
		t.Fatal("plugin missing from target manifest")
	}
	if entries[0].Version != "2.0.0" {
		t.Errorf("version = %q, want %q", entries[0].Version, "2.0.0")
	}
	if entries[0].GitCommitSha != "bbb222" {
		t.Errorf("sha = %q, want %q", entries[0].GitCommitSha, "bbb222")
	}

	// v2 symlink should exist
	tgtV2 := filepath.Join(rigDir, "plugins", "cache", "market", "myplugin", "v2.0.0")
	if _, err := os.Lstat(tgtV2); err != nil {
		t.Errorf("v2 symlink missing: %v", err)
	}

	// v1 symlink should be cleaned up
	if _, err := os.Lstat(tgtV1); err == nil {
		t.Error("v1 symlink should have been removed after version update")
	}
}
