package config

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// config append: new array (key does not exist)
// ---------------------------------------------------------------------------

func TestAppend_NewArray(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "append", "identity.tags", "foo"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config append failed: %v", err)
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
	if len(tags) != 1 || tags[0] != "foo" {
		t.Errorf("identity.tags = %v, want [foo]", tags)
	}
}

// ---------------------------------------------------------------------------
// config append: existing array
// ---------------------------------------------------------------------------

func TestAppend_ExistingArray(t *testing.T) {
	env := newTestEnv(t)

	// Seed an existing array.
	env.RootCmd.SetArgs([]string{"config", "set", "identity.tags", `["a","b"]`})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set seed failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "append", "identity.tags", "c"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config append failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	identity := m["identity"].(map[string]any)
	tags := identity["tags"].([]any)
	if len(tags) != 3 || tags[0] != "a" || tags[1] != "b" || tags[2] != "c" {
		t.Errorf("identity.tags = %v, want [a b c]", tags)
	}
}

// ---------------------------------------------------------------------------
// config append: nil/null value at path creates new array
// ---------------------------------------------------------------------------

func TestAppend_NilValueCreatesArray(t *testing.T) {
	env := newTestEnv(t)

	// Set the key to null first.
	env.RootCmd.SetArgs([]string{"config", "set", "identity.tags", "null"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set null failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "append", "identity.tags", "first"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config append failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	identity := m["identity"].(map[string]any)
	tags := identity["tags"].([]any)
	if len(tags) != 1 || tags[0] != "first" {
		t.Errorf("identity.tags = %v, want [first]", tags)
	}
}

// ---------------------------------------------------------------------------
// config append: non-array value is an error
// ---------------------------------------------------------------------------

func TestAppend_NonArrayError(t *testing.T) {
	env := newTestEnv(t)

	// Set a string value first.
	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "debug"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set seed failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "append", "logs.level", "extra"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when appending to non-array")
	}
}

// ---------------------------------------------------------------------------
// config append: JSON-parsed values
// ---------------------------------------------------------------------------

func TestAppend_JSONNumber(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "append", "nums", "42"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config append failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	nums := m["nums"].([]any)
	if len(nums) != 1 || nums[0] != float64(42) {
		t.Errorf("nums = %v, want [42]", nums)
	}
}

func TestAppend_JSONObject(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "append", "steps", `{"name":"lint"}`})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config append failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	steps := m["steps"].([]any)
	if len(steps) != 1 {
		t.Fatalf("steps has %d elements, want 1", len(steps))
	}
	obj, ok := steps[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", steps[0])
	}
	if obj["name"] != "lint" {
		t.Errorf("steps[0].name = %v, want %q", obj["name"], "lint")
	}
}

// ---------------------------------------------------------------------------
// --tier flag
// ---------------------------------------------------------------------------

func TestAppend_TierCustom(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "append", "identity.tags", "x", "--tier", "custom"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config append --tier custom failed: %v", err)
	}

	m := readCustomOverlay(t, env)
	identity := m["identity"].(map[string]any)
	tags := identity["tags"].([]any)
	if len(tags) != 1 || tags[0] != "x" {
		t.Errorf("identity.tags = %v, want [x]", tags)
	}
}

func TestAppend_TierInvalid(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "append", "identity.tags", "x", "--tier", "base"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for --tier base")
	}
}

// ---------------------------------------------------------------------------
// Argument validation
// ---------------------------------------------------------------------------

func TestAppend_TooFewArgs(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "append", "identity.tags"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too few arguments")
	}
}

func TestAppend_ExtraArgs_Ignored(t *testing.T) {
	env := newTestEnv(t)

	// Extra args beyond the required key+value are silently ignored.
	env.RootCmd.SetArgs([]string{"config", "append", "identity.tags", "a", "b"})
	err := env.RootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error with extra args: %v", err)
	}
}

// ---------------------------------------------------------------------------
// JSON output
// ---------------------------------------------------------------------------

func TestAppend_JSONOutput(t *testing.T) {
	env := newTestEnv(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "append", "identity.tags", "foo", "--json"})
	err := env.RootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("config append --json failed: %v", err)
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
	if envelope.Action != "config_append" {
		t.Errorf("action = %q, want %q", envelope.Action, "config_append")
	}
	data, ok := envelope.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data to be a map, got %T", envelope.Data)
	}
	if data["key"] != "identity.tags" {
		t.Errorf("data.key = %v, want %q", data["key"], "identity.tags")
	}
	if data["tier"] != "local" {
		t.Errorf("data.tier = %v, want %q", data["tier"], "local")
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func TestRegister_HasAppendSubcommand(t *testing.T) {
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
		if sub.Name() == "append" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("append subcommand not registered under config")
	}
}
