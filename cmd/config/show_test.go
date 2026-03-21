package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
	"github.com/spf13/cobra"
)

type testEnv struct {
	WolfcastleDir string
	App           *cmdutil.App
	RootCmd       *cobra.Command
	env           *testutil.Environment
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	env := testutil.NewEnvironment(t)
	af := env.ToAppFields()

	testApp := &cmdutil.App{
		Config:   af.Config,
		Identity: af.Identity,
		State:    af.State,
		Prompts:  af.Prompts,
		Classes:  af.Classes,
		Daemon:   af.Daemon,
		Git:      af.Git,
		Clock:    clock.New(),
	}

	rootCmd := &cobra.Command{Use: "wolfcastle"}
	rootCmd.PersistentFlags().BoolVar(&testApp.JSON, "json", false, "Output in JSON format")
	rootCmd.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "Lifecycle:"},
		&cobra.Group{ID: "work", Title: "Work Management:"},
		&cobra.Group{ID: "audit", Title: "Auditing:"},
		&cobra.Group{ID: "docs", Title: "Documentation:"},
		&cobra.Group{ID: "diagnostics", Title: "Diagnostics:"},
		&cobra.Group{ID: "integration", Title: "Integration:"},
	)
	Register(testApp, rootCmd)

	return &testEnv{
		WolfcastleDir: env.Root,
		App:           testApp,
		RootCmd:       rootCmd,
		env:           env,
	}
}

// writeTierConfig writes a JSON config file to the specified tier directory.
func (e *testEnv) writeTierConfig(t *testing.T, tier string, data map[string]any) {
	t.Helper()
	path := filepath.Join(e.WolfcastleDir, "system", tier, "config.json")
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("marshaling %s config: %v", tier, err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("writing %s config: %v", tier, err)
	}
}

// removeTierConfig removes a tier's config.json to simulate a missing file.
func (e *testEnv) removeTierConfig(t *testing.T, tier string) {
	t.Helper()
	path := filepath.Join(e.WolfcastleDir, "system", tier, "config.json")
	_ = os.Remove(path)
}

// ---------------------------------------------------------------------------
// Register
// ---------------------------------------------------------------------------

func TestRegister_AddsConfigCommand(t *testing.T) {
	env := newTestEnv(t)

	// Verify "config" is a child of root
	found := false
	for _, cmd := range env.RootCmd.Commands() {
		if cmd.Use == "config" {
			found = true
			if cmd.GroupID != "diagnostics" {
				t.Errorf("config command group = %q, want %q", cmd.GroupID, "diagnostics")
			}
			break
		}
	}
	if !found {
		t.Fatal("config command not registered on root")
	}
}

func TestRegister_HasShowSubcommand(t *testing.T) {
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
		if sub.Name() == "show" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("show subcommand not registered under config")
	}
}

// ---------------------------------------------------------------------------
// Default mode (merged config)
// ---------------------------------------------------------------------------

func TestShow_DefaultMode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "show"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show failed: %v", err)
	}
}

func TestShow_DefaultMode_JSON(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "show", "--json"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show --json failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// --tier mode
// ---------------------------------------------------------------------------

func TestShow_TierBase(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "show", "--tier", "base"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show --tier base failed: %v", err)
	}
}

func TestShow_TierLocal(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "show", "--tier", "local"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show --tier local failed: %v", err)
	}
}

func TestShow_TierCustom(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "show", "--tier", "custom"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show --tier custom failed: %v", err)
	}
}

func TestShow_TierInvalid(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "show", "--tier", "bogus"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid tier name")
	}
}

func TestShow_TierMissingFile(t *testing.T) {
	env := newTestEnv(t)
	env.removeTierConfig(t, "custom")

	env.RootCmd.SetArgs([]string{"config", "show", "--tier", "custom"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show --tier custom (missing file) should succeed: %v", err)
	}
}

func TestShow_TierMalformedJSON(t *testing.T) {
	env := newTestEnv(t)

	// Write garbage to a tier file
	path := filepath.Join(env.WolfcastleDir, "system", "custom", "config.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("writing malformed config: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "show", "--tier", "custom"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for malformed JSON tier file")
	}
}

func TestShow_TierJSON(t *testing.T) {
	env := newTestEnv(t)

	env.writeTierConfig(t, "local", map[string]any{
		"identity": map[string]any{
			"user":    "wild",
			"machine": "test-box",
		},
	})

	env.RootCmd.SetArgs([]string{"config", "show", "--tier", "local", "--json"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show --tier local --json failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// --raw mode
// ---------------------------------------------------------------------------

func TestShow_RawMode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "show", "--raw"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show --raw failed: %v", err)
	}
}

func TestShow_RawMode_JSON(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "show", "--raw", "--json"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show --raw --json failed: %v", err)
	}
}

func TestShow_RawModeWithTier(t *testing.T) {
	env := newTestEnv(t)

	// --raw --tier should behave identically to --tier alone
	env.RootCmd.SetArgs([]string{"config", "show", "--raw", "--tier", "base"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show --raw --tier base failed: %v", err)
	}
}

func TestShow_RawMode_MalformedTier(t *testing.T) {
	env := newTestEnv(t)

	path := filepath.Join(env.WolfcastleDir, "system", "custom", "config.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("writing malformed config: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "show", "--raw"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when --raw encounters malformed tier file")
	}
}

func TestShow_RawMode_MissingAllTiers(t *testing.T) {
	env := newTestEnv(t)
	env.removeTierConfig(t, "base")
	env.removeTierConfig(t, "custom")
	env.removeTierConfig(t, "local")

	env.RootCmd.SetArgs([]string{"config", "show", "--raw"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show --raw with no tier files should succeed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// isValidTier
// ---------------------------------------------------------------------------

func TestIsValidTier(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"base", true},
		{"custom", true},
		{"local", true},
		{"", false},
		{"global", false},
		{"BASE", false},
	}

	for _, tt := range tests {
		if got := isValidTier(tt.name); got != tt.valid {
			t.Errorf("isValidTier(%q) = %v, want %v", tt.name, got, tt.valid)
		}
	}
}

// ---------------------------------------------------------------------------
// readTierFile
// ---------------------------------------------------------------------------

func TestReadTierFile_Present(t *testing.T) {
	env := newTestEnv(t)

	m, err := readTierFile(env.WolfcastleDir, "base")
	if err != nil {
		t.Fatalf("readTierFile(base) failed: %v", err)
	}
	if _, ok := m["version"]; !ok {
		t.Error("expected base tier to contain a 'version' key")
	}
}

func TestReadTierFile_Missing(t *testing.T) {
	env := newTestEnv(t)
	env.removeTierConfig(t, "custom")

	m, err := readTierFile(env.WolfcastleDir, "custom")
	if err != nil {
		t.Fatalf("readTierFile(missing) should not error: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map for missing tier, got %d keys", len(m))
	}
}

func TestReadTierFile_Malformed(t *testing.T) {
	env := newTestEnv(t)

	path := filepath.Join(env.WolfcastleDir, "system", "local", "config.json")
	if err := os.WriteFile(path, []byte("{{bad}}"), 0o644); err != nil {
		t.Fatalf("writing malformed file: %v", err)
	}

	_, err := readTierFile(env.WolfcastleDir, "local")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

// ---------------------------------------------------------------------------
// mergeRawTiers
// ---------------------------------------------------------------------------

func TestMergeRawTiers_OverlayOrder(t *testing.T) {
	env := newTestEnv(t)

	// base has version=1, local overrides to version=99
	env.writeTierConfig(t, "base", map[string]any{"version": float64(1)})
	env.writeTierConfig(t, "local", map[string]any{"version": float64(99)})
	env.removeTierConfig(t, "custom")

	m, err := mergeRawTiers(env.WolfcastleDir)
	if err != nil {
		t.Fatalf("mergeRawTiers failed: %v", err)
	}
	if v, ok := m["version"]; !ok || v != float64(99) {
		t.Errorf("expected version=99 from local override, got %v", m["version"])
	}
}

// ---------------------------------------------------------------------------
// Section filtering
// ---------------------------------------------------------------------------

func TestShow_SectionDefault(t *testing.T) {
	env := newTestEnv(t)

	// "models" is a section present in the default merged config.
	env.RootCmd.SetArgs([]string{"config", "show", "models"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show models failed: %v", err)
	}
}

func TestShow_SectionWithTier(t *testing.T) {
	env := newTestEnv(t)

	env.writeTierConfig(t, "local", map[string]any{
		"identity": map[string]any{"user": "wild"},
		"logs":     map[string]any{"level": "debug"},
	})

	env.RootCmd.SetArgs([]string{"config", "show", "identity", "--tier", "local"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show identity --tier local failed: %v", err)
	}
}

func TestShow_SectionWithRaw(t *testing.T) {
	env := newTestEnv(t)

	env.writeTierConfig(t, "base", map[string]any{
		"version": float64(1),
		"logs":    map[string]any{"level": "info"},
	})

	env.RootCmd.SetArgs([]string{"config", "show", "logs", "--raw"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("config show logs --raw failed: %v", err)
	}
}

func TestShow_SectionInvalid(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "show", "nonexistent"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown section")
	}
	if !strings.Contains(err.Error(), "unknown section") {
		t.Errorf("error should mention unknown section, got: %v", err)
	}
	if !strings.Contains(err.Error(), "valid sections") {
		t.Errorf("error should list valid sections, got: %v", err)
	}
}

func TestShow_SectionJSON(t *testing.T) {
	env := newTestEnv(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "show", "models", "--json"})
	err := env.RootCmd.Execute()

	if closeErr := w.Close(); closeErr != nil {
		t.Errorf("closing pipe writer: %v", closeErr)
	}
	os.Stdout = old

	if err != nil {
		t.Fatalf("config show models --json failed: %v", err)
	}

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	if closeErr := r.Close(); closeErr != nil {
		t.Errorf("closing pipe reader: %v", closeErr)
	}

	var envelope output.Response
	if err := json.Unmarshal(buf[:n], &envelope); err != nil {
		t.Fatalf("parsing JSON envelope: %v\nraw: %s", err, string(buf[:n]))
	}
	if !envelope.OK {
		t.Error("expected ok=true in envelope")
	}
	if envelope.Data == nil {
		t.Error("expected non-nil data for section in JSON mode")
	}
}

func TestShow_TooManyArgs(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "show", "models", "extra"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many arguments")
	}
}

// ---------------------------------------------------------------------------
// extractSection
// ---------------------------------------------------------------------------

func TestExtractSection_FromMap(t *testing.T) {
	m := map[string]any{
		"alpha": "one",
		"beta":  map[string]any{"nested": true},
	}
	val, err := extractSection(m, "beta")
	if err != nil {
		t.Fatalf("extractSection(map, beta) failed: %v", err)
	}
	nested, ok := val.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", val)
	}
	if nested["nested"] != true {
		t.Errorf("expected nested=true, got %v", nested["nested"])
	}
}

func TestExtractSection_FromStruct(t *testing.T) {
	type sample struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	val, err := extractSection(&sample{Name: "test", Count: 42}, "count")
	if err != nil {
		t.Fatalf("extractSection(struct, count) failed: %v", err)
	}
	if val != float64(42) {
		t.Errorf("expected 42, got %v", val)
	}
}

func TestExtractSection_MissingKey(t *testing.T) {
	m := map[string]any{"alpha": 1, "beta": 2}
	_, err := extractSection(m, "gamma")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if !strings.Contains(err.Error(), "alpha") || !strings.Contains(err.Error(), "beta") {
		t.Errorf("error should list valid keys, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// marshalPretty
// ---------------------------------------------------------------------------

func TestMarshalPretty_NoHTMLEscape(t *testing.T) {
	data := map[string]any{"url": "http://example.com?foo=1&bar=2"}
	s, err := marshalPretty(data)
	if err != nil {
		t.Fatalf("marshalPretty failed: %v", err)
	}
	if got := s; got == "" {
		t.Fatal("expected non-empty output")
	}
	// Verify ampersand is NOT escaped
	var parsed map[string]any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["url"] != "http://example.com?foo=1&bar=2" {
		t.Errorf("url round-trip failed: %v", parsed["url"])
	}
}

// ---------------------------------------------------------------------------
// Full envelope structure
// ---------------------------------------------------------------------------

func TestShow_JSONEnvelopeStructure(t *testing.T) {
	env := newTestEnv(t)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "show", "--tier", "local", "--json"})
	err := env.RootCmd.Execute()

	if err := w.Close(); err != nil {
		t.Errorf("closing pipe writer: %v", err)
	}
	os.Stdout = old

	if err != nil {
		t.Fatalf("config show --tier local --json failed: %v", err)
	}

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	if err := r.Close(); err != nil {
		t.Errorf("closing pipe reader: %v", err)
	}

	var envelope output.Response
	if err := json.Unmarshal(buf[:n], &envelope); err != nil {
		t.Fatalf("parsing JSON envelope: %v\nraw: %s", err, string(buf[:n]))
	}
	if !envelope.OK {
		t.Error("expected ok=true in envelope")
	}
	if envelope.Action != "config_show" {
		t.Errorf("action = %q, want %q", envelope.Action, "config_show")
	}
	if envelope.Data == nil {
		t.Error("expected non-nil data in envelope")
	}
}
