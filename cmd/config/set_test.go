package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// config set basics
// ---------------------------------------------------------------------------

func TestSet_SimpleString(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "debug"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	logs, ok := m["logs"].(map[string]any)
	if !ok {
		t.Fatalf("expected logs to be a map, got %T", m["logs"])
	}
	if logs["level"] != "debug" {
		t.Errorf("logs.level = %v, want %q", logs["level"], "debug")
	}
}

func TestSet_JSONNumber(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "pipeline.timeout", "30"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	pipeline, ok := m["pipeline"].(map[string]any)
	if !ok {
		t.Fatalf("expected pipeline to be a map, got %T", m["pipeline"])
	}
	if pipeline["timeout"] != float64(30) {
		t.Errorf("pipeline.timeout = %v, want 30", pipeline["timeout"])
	}
}

func TestSet_JSONBoolean(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "pipeline.enabled", "true"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	pipeline, ok := m["pipeline"].(map[string]any)
	if !ok {
		t.Fatalf("expected pipeline to be a map, got %T", m["pipeline"])
	}
	if pipeline["enabled"] != true {
		t.Errorf("pipeline.enabled = %v, want true", pipeline["enabled"])
	}
}

func TestSet_JSONNull(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "null"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	logs, ok := m["logs"].(map[string]any)
	if !ok {
		t.Fatalf("expected logs to be a map, got %T", m["logs"])
	}
	if logs["level"] != nil {
		t.Errorf("logs.level = %v, want nil", logs["level"])
	}
}

func TestSet_JSONArray(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "identity.tags", `["a","b"]`})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	identity, ok := m["identity"].(map[string]any)
	if !ok {
		t.Fatalf("expected identity to be a map, got %T", m["identity"])
	}
	tags, ok := identity["tags"].([]any)
	if !ok {
		t.Fatalf("expected tags to be a slice, got %T", identity["tags"])
	}
	if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Errorf("identity.tags = %v, want [a b]", tags)
	}
}

func TestSet_JSONObject(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "identity.meta", `{"k":"v"}`})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	identity, ok := m["identity"].(map[string]any)
	if !ok {
		t.Fatalf("expected identity to be a map, got %T", m["identity"])
	}
	meta, ok := identity["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta to be a map, got %T", identity["meta"])
	}
	if meta["k"] != "v" {
		t.Errorf("identity.meta.k = %v, want %q", meta["k"], "v")
	}
}

func TestSet_BareStringFallback(t *testing.T) {
	env := newTestEnv(t)

	// "hello world" is not valid JSON, so it should be stored as a string.
	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "hello world"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	logs, ok := m["logs"].(map[string]any)
	if !ok {
		t.Fatalf("expected logs to be a map, got %T", m["logs"])
	}
	if logs["level"] != "hello world" {
		t.Errorf("logs.level = %v, want %q", logs["level"], "hello world")
	}
}

// ---------------------------------------------------------------------------
// config set: top-level scalar (single segment key)
// ---------------------------------------------------------------------------

func TestSet_TopLevelScalar(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "version", "42"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set top-level key failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	if m["version"] != float64(42) {
		t.Errorf("version = %v, want 42", m["version"])
	}
}

// ---------------------------------------------------------------------------
// config set: deeply nested map-keyed structure (auto-vivification)
// ---------------------------------------------------------------------------

func TestSet_DeeplyNestedKey(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "models.fast.command", "claude"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set deeply nested key failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	models, ok := m["models"].(map[string]any)
	if !ok {
		t.Fatalf("expected models to be a map, got %T", m["models"])
	}
	fast, ok := models["fast"].(map[string]any)
	if !ok {
		t.Fatalf("expected models.fast to be a map, got %T", models["fast"])
	}
	if fast["command"] != "claude" {
		t.Errorf("models.fast.command = %v, want %q", fast["command"], "claude")
	}
}

// ---------------------------------------------------------------------------
// config set: validation failure triggers rollback
// ---------------------------------------------------------------------------

func TestSet_ValidationFailureRollback(t *testing.T) {
	env := newTestEnv(t)

	// Snapshot the local overlay before the invalid mutation.
	overlayPath := filepath.Join(env.WolfcastleDir, "system", "local", "config.json")
	before, err := os.ReadFile(overlayPath)
	if err != nil {
		t.Fatalf("reading local overlay before mutation: %v", err)
	}

	// Setting daemon.poll_interval_seconds to 0 violates "must be > 0".
	env.RootCmd.SetArgs([]string{"config", "set", "daemon.poll_interval_seconds", "0"})
	err = env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected validation error for daemon.poll_interval_seconds = 0")
	}

	// The local overlay should be restored to its original content.
	after, readErr := os.ReadFile(overlayPath)
	if readErr != nil {
		t.Fatalf("reading local overlay after rollback: %v", readErr)
	}
	if string(after) != string(before) {
		t.Errorf("local overlay was not rolled back:\nbefore: %s\nafter:  %s", before, after)
	}
}

// ---------------------------------------------------------------------------
// config set: invalid key formats
// ---------------------------------------------------------------------------

func TestSet_EmptySegmentKey(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "logs..level", "debug"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for key with empty segment")
	}
}

func TestSet_TrailingDotKey(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "logs.", "debug"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for key with trailing dot")
	}
}

// ---------------------------------------------------------------------------
// --tier flag
// ---------------------------------------------------------------------------

func TestSet_TierCustom(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "warn", "--tier", "custom"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set --tier custom failed: %v", err)
	}

	m := readCustomOverlay(t, env)
	logs, ok := m["logs"].(map[string]any)
	if !ok {
		t.Fatalf("expected logs to be a map, got %T", m["logs"])
	}
	if logs["level"] != "warn" {
		t.Errorf("logs.level = %v, want %q", logs["level"], "warn")
	}
}

func TestSet_TierDefaultsToLocal(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "trace"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	logs, ok := m["logs"].(map[string]any)
	if !ok {
		t.Fatalf("expected logs to be a map, got %T", m["logs"])
	}
	if logs["level"] != "trace" {
		t.Errorf("logs.level = %v, want %q", logs["level"], "trace")
	}
}

func TestSet_TierInvalid(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "debug", "--tier", "base"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for --tier base")
	}
}

func TestSet_TierBogus(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "debug", "--tier", "bogus"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for --tier bogus")
	}
}

// ---------------------------------------------------------------------------
// Argument validation
// ---------------------------------------------------------------------------

func TestSet_TooFewArgs(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "logs.level"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too few arguments")
	}
}

func TestSet_TooManyArgs(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "debug", "extra"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many arguments")
	}
}

// ---------------------------------------------------------------------------
// JSON output
// ---------------------------------------------------------------------------

func TestSet_JSONOutput(t *testing.T) {
	env := newTestEnv(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "debug", "--json"})
	err := env.RootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("config set --json failed: %v", err)
	}

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	_ = r.Close()

	var envelope output.Response
	if err := json.Unmarshal(buf[:n], &envelope); err != nil {
		t.Fatalf("parsing JSON envelope: %v\nraw: %s", err, string(buf[:n]))
	}
	if !envelope.OK {
		t.Error("expected ok=true in envelope")
	}
	if envelope.Action != "config_set" {
		t.Errorf("action = %q, want %q", envelope.Action, "config_set")
	}
	data, ok := envelope.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data to be a map, got %T", envelope.Data)
	}
	if data["key"] != "logs.level" {
		t.Errorf("data.key = %v, want %q", data["key"], "logs.level")
	}
	if data["tier"] != "local" {
		t.Errorf("data.tier = %v, want %q", data["tier"], "local")
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func TestRegister_HasSetSubcommand(t *testing.T) {
	env := newTestEnv(t)

	var configCmd *cobra.Command
	for _, cmd := range env.RootCmd.Commands() {
		if cmd.Use == "config" {
			configCmd = cmd
			break
		}
	}
	if configCmd == nil {
		t.Fatal("config command not found")
	}

	found := false
	for _, sub := range configCmd.Commands() {
		if sub.Name() == "set" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("set subcommand not registered under config")
	}
}

// ---------------------------------------------------------------------------
// parseValue
// ---------------------------------------------------------------------------

func TestParseValue(t *testing.T) {
	tests := []struct {
		input string
		want  any
	}{
		{"42", float64(42)},
		{"3.14", float64(3.14)},
		{"true", true},
		{"false", false},
		{"null", nil},
		{`"quoted"`, "quoted"},
		{`[1,2,3]`, []any{float64(1), float64(2), float64(3)}},
		{`{"a":"b"}`, map[string]any{"a": "b"}},
		{"bare string", "bare string"},
		{"hello", "hello"},
	}

	for _, tt := range tests {
		got := parseValue(tt.input)
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(tt.want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("parseValue(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func readLocalOverlay(t *testing.T, env *testEnv) map[string]any {
	t.Helper()
	return readOverlay(t, env, "local")
}

func readCustomOverlay(t *testing.T, env *testEnv) map[string]any {
	t.Helper()
	return readOverlay(t, env, "custom")
}

func readOverlay(t *testing.T, env *testEnv, tier string) map[string]any {
	t.Helper()
	path := filepath.Join(env.WolfcastleDir, "system", tier, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s overlay: %v", tier, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parsing %s overlay: %v", tier, err)
	}
	return m
}
