package config

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// config remove: remove string from array
// ---------------------------------------------------------------------------

func TestRemove_StringFromArray(t *testing.T) {
	env := newTestEnv(t)

	// Seed an array with three elements.
	env.RootCmd.SetArgs([]string{"config", "set", "identity.tags", `["a","b","c"]`})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set seed failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "remove", "identity.tags", "b"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config remove failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	identity := m["identity"].(map[string]any)
	tags := identity["tags"].([]any)
	if len(tags) != 2 || tags[0] != "a" || tags[1] != "c" {
		t.Errorf("identity.tags = %v, want [a c]", tags)
	}
}

// ---------------------------------------------------------------------------
// config remove: remove number from array (JSON equality)
// ---------------------------------------------------------------------------

func TestRemove_NumberFromArray(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "nums", `[1,2,3]`})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set seed failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "remove", "nums", "2"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config remove failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	nums := m["nums"].([]any)
	if len(nums) != 2 || nums[0] != float64(1) || nums[1] != float64(3) {
		t.Errorf("nums = %v, want [1 3]", nums)
	}
}

// ---------------------------------------------------------------------------
// config remove: remove object from array (JSON equality)
// ---------------------------------------------------------------------------

func TestRemove_ObjectFromArray(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "steps", `[{"name":"lint"},{"name":"build"}]`})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set seed failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "remove", "steps", `{"name":"lint"}`})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config remove failed: %v", err)
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
	if obj["name"] != "build" {
		t.Errorf("steps[0].name = %v, want %q", obj["name"], "build")
	}
}

// ---------------------------------------------------------------------------
// config remove: removes only the first matching element
// ---------------------------------------------------------------------------

func TestRemove_FirstMatchOnly(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "identity.tags", `["x","y","x"]`})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set seed failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "remove", "identity.tags", "x"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config remove failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	identity := m["identity"].(map[string]any)
	tags := identity["tags"].([]any)
	if len(tags) != 2 || tags[0] != "y" || tags[1] != "x" {
		t.Errorf("identity.tags = %v, want [y x]", tags)
	}
}

// ---------------------------------------------------------------------------
// config remove: value not found in array
// ---------------------------------------------------------------------------

func TestRemove_NotFoundError(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "identity.tags", `["a","b"]`})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set seed failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "remove", "identity.tags", "z"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when removing non-existent value")
	}
}

// ---------------------------------------------------------------------------
// config remove: non-array value is an error
// ---------------------------------------------------------------------------

func TestRemove_NonArrayError(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "debug"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set seed failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "remove", "logs.level", "debug"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when removing from non-array")
	}
}

// ---------------------------------------------------------------------------
// config remove: key does not exist
// ---------------------------------------------------------------------------

func TestRemove_MissingKeyError(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "remove", "nonexistent.key", "val"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when key does not exist")
	}
}

// ---------------------------------------------------------------------------
// --tier flag
// ---------------------------------------------------------------------------

func TestRemove_TierCustom(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "identity.tags", `["a","b"]`, "--tier", "custom"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set seed failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "remove", "identity.tags", "a", "--tier", "custom"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config remove --tier custom failed: %v", err)
	}

	m := readCustomOverlay(t, env)
	identity := m["identity"].(map[string]any)
	tags := identity["tags"].([]any)
	if len(tags) != 1 || tags[0] != "b" {
		t.Errorf("identity.tags = %v, want [b]", tags)
	}
}

func TestRemove_TierInvalid(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "remove", "identity.tags", "x", "--tier", "base"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for --tier base")
	}
}

// ---------------------------------------------------------------------------
// Argument validation
// ---------------------------------------------------------------------------

func TestRemove_TooFewArgs(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "remove", "identity.tags"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too few arguments")
	}
}

func TestRemove_TooManyArgs(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "remove", "identity.tags", "a", "b"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many arguments")
	}
}

// ---------------------------------------------------------------------------
// JSON output
// ---------------------------------------------------------------------------

func TestRemove_JSONOutput(t *testing.T) {
	env := newTestEnv(t)

	// Seed the array.
	env.RootCmd.SetArgs([]string{"config", "set", "identity.tags", `["foo","bar"]`})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set seed failed: %v", err)
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "remove", "identity.tags", "foo", "--json"})
	err := env.RootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("config remove --json failed: %v", err)
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
	if envelope.Action != "config_remove" {
		t.Errorf("action = %q, want %q", envelope.Action, "config_remove")
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

func TestRegister_HasRemoveSubcommand(t *testing.T) {
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
		if sub.Name() == "remove" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("remove subcommand not registered under config")
	}
}
