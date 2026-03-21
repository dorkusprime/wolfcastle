package config

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// config unset basics
// ---------------------------------------------------------------------------

func TestUnset_SetsKeyToNil(t *testing.T) {
	env := newTestEnv(t)

	// Set a key first, then unset it.
	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "debug"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "unset", "logs.level"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config unset failed: %v", err)
	}

	// DeletePath sets the value to nil (null-deletion semantics for DeepMerge).
	m := readLocalOverlay(t, env)
	logs, ok := m["logs"].(map[string]any)
	if !ok {
		t.Fatalf("expected logs to be a map, got %T", m["logs"])
	}
	val, exists := logs["level"]
	if !exists {
		t.Fatal("logs.level key should still exist (set to nil)")
	}
	if val != nil {
		t.Errorf("logs.level = %v, want nil", val)
	}
}

func TestUnset_NonexistentKeySucceeds(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "unset", "does.not.exist"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config unset on nonexistent key should succeed, got: %v", err)
	}
}

func TestUnset_TopLevelKey(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "simpleflag", "yes"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "unset", "simpleflag"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config unset failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	val, exists := m["simpleflag"]
	if !exists {
		t.Fatal("simpleflag key should still exist (set to nil)")
	}
	if val != nil {
		t.Errorf("simpleflag = %v, want nil", val)
	}
}

// ---------------------------------------------------------------------------
// --tier flag
// ---------------------------------------------------------------------------

func TestUnset_TierCustom(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "warn", "--tier", "custom"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set --tier custom failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "unset", "logs.level", "--tier", "custom"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config unset --tier custom failed: %v", err)
	}

	m := readCustomOverlay(t, env)
	logs, ok := m["logs"].(map[string]any)
	if !ok {
		t.Fatalf("expected logs to be a map, got %T", m["logs"])
	}
	val, exists := logs["level"]
	if !exists {
		t.Fatal("logs.level key should still exist in custom tier (set to nil)")
	}
	if val != nil {
		t.Errorf("logs.level = %v, want nil", val)
	}
}

func TestUnset_TierDefaultsToLocal(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "set", "logs.level", "trace"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "unset", "logs.level"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config unset failed: %v", err)
	}

	m := readLocalOverlay(t, env)
	logs, ok := m["logs"].(map[string]any)
	if !ok {
		t.Fatalf("expected logs to be a map, got %T", m["logs"])
	}
	val, exists := logs["level"]
	if !exists {
		t.Fatal("logs.level key should still exist (set to nil)")
	}
	if val != nil {
		t.Errorf("logs.level = %v, want nil", val)
	}
}

func TestUnset_TierInvalid(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "unset", "logs.level", "--tier", "base"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for --tier base")
	}
}

func TestUnset_TierBogus(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "unset", "logs.level", "--tier", "bogus"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for --tier bogus")
	}
}

// ---------------------------------------------------------------------------
// Argument validation
// ---------------------------------------------------------------------------

func TestUnset_NoArgs(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "unset"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing arguments")
	}
}

func TestUnset_ExtraArgs_Ignored(t *testing.T) {
	env := newTestEnv(t)

	// Extra args beyond the required key are silently ignored (no ExactArgs).
	env.RootCmd.SetArgs([]string{"config", "unset", "logs.level", "extra"})
	err := env.RootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error with extra args: %v", err)
	}
}

// ---------------------------------------------------------------------------
// JSON output
// ---------------------------------------------------------------------------

func TestUnset_JSONOutput(t *testing.T) {
	env := newTestEnv(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "unset", "logs.level", "--json"})
	err := env.RootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("config unset --json failed: %v", err)
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
	if envelope.Action != "config_unset" {
		t.Errorf("action = %q, want %q", envelope.Action, "config_unset")
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

func TestRegister_HasUnsetSubcommand(t *testing.T) {
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
		if sub.Name() == "unset" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("unset subcommand not registered under config")
	}
}
