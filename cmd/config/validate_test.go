package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// splitValidationError (unit)
// ---------------------------------------------------------------------------

func TestSplitValidationError_Formatted(t *testing.T) {
	err := errors.New("config validation failed:\n  - first problem\n  - second problem")
	got := splitValidationError(err)
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(got), got)
	}
	if got[0] != "first problem" {
		t.Errorf("line[0] = %q, want %q", got[0], "first problem")
	}
	if got[1] != "second problem" {
		t.Errorf("line[1] = %q, want %q", got[1], "second problem")
	}
}

func TestSplitValidationError_SingleLine(t *testing.T) {
	err := errors.New("something went wrong")
	got := splitValidationError(err)
	if len(got) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(got), got)
	}
	if got[0] != "something went wrong" {
		t.Errorf("line[0] = %q", got[0])
	}
}

func TestSplitValidationError_EmptyLines(t *testing.T) {
	err := errors.New("config validation failed:\n  - one\n\n  - two\n")
	got := splitValidationError(err)
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(got), got)
	}
}

// ---------------------------------------------------------------------------
// parseValidationErrors (unit)
// ---------------------------------------------------------------------------

func TestParseValidationErrors_AppendsToReport(t *testing.T) {
	report := &validationReport{
		Issues: []validationIssue{{Severity: "warning", Category: "test", Description: "existing"}},
	}
	err := errors.New("config validation failed:\n  - bad field\n  - another")
	parseValidationErrors(report, "structure", err)
	if len(report.Issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(report.Issues))
	}
	for _, issue := range report.Issues[1:] {
		if issue.Severity != "error" {
			t.Errorf("parsed issue severity = %q, want %q", issue.Severity, "error")
		}
		if issue.Category != "structure" {
			t.Errorf("parsed issue category = %q, want %q", issue.Category, "structure")
		}
	}
}

// ---------------------------------------------------------------------------
// Valid config: no issues
// ---------------------------------------------------------------------------

func TestValidate_ValidConfig_ExitZero(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"config", "validate"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("expected success for valid config, got: %v", err)
	}
}

func TestValidate_ValidConfig_HumanOutput(t *testing.T) {
	env := newTestEnv(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "validate"})
	err := env.RootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	_ = r.Close()

	out := string(buf[:n])
	if !strings.Contains(out, "0 errors") {
		t.Errorf("expected '0 errors' in output, got: %s", out)
	}
	if !strings.Contains(out, "0 warnings") {
		t.Errorf("expected '0 warnings' in output, got: %s", out)
	}
}

func TestValidate_ValidConfig_JSON(t *testing.T) {
	env := newTestEnv(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "validate", "--json"})
	err := env.RootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	_ = r.Close()

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Issues       []validationIssue `json:"issues"`
			ErrorCount   int               `json:"error_count"`
			WarningCount int               `json:"warning_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf[:n], &envelope); err != nil {
		t.Fatalf("parsing JSON: %v\nraw: %s", err, string(buf[:n]))
	}
	if !envelope.OK {
		t.Error("expected ok=true")
	}
	if len(envelope.Data.Issues) != 0 {
		t.Errorf("expected empty issues, got %d", len(envelope.Data.Issues))
	}
	if envelope.Data.ErrorCount != 0 {
		t.Errorf("error_count = %d, want 0", envelope.Data.ErrorCount)
	}
	if envelope.Data.WarningCount != 0 {
		t.Errorf("warning_count = %d, want 0", envelope.Data.WarningCount)
	}
}

// ---------------------------------------------------------------------------
// Structural errors (caught by Load)
// ---------------------------------------------------------------------------

func TestValidate_StructuralError_EmptyPipeline(t *testing.T) {
	env := newTestEnv(t)

	// Overwrite base config with an empty pipeline to trigger structural validation
	// failure during Load(). This requires rewriting the base config entirely.
	env.writeTierConfig(t, "base", map[string]any{
		"version": float64(1),
		"pipeline": map[string]any{
			"stages":      map[string]any{},
			"stage_order": []any{},
		},
	})

	env.RootCmd.SetArgs([]string{"config", "validate"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for structurally invalid config")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected 'loading config' in error, got: %v", err)
	}
}

func TestValidate_StructuralError_InvalidStageRef(t *testing.T) {
	env := newTestEnv(t)

	// Override stage_order to reference a stage that doesn't exist.
	env.writeTierConfig(t, "local", map[string]any{
		"identity": map[string]any{"user": "test", "machine": "machine"},
		"pipeline": map[string]any{
			"stage_order": []any{"intake", "execute", "nonexistent"},
		},
	})

	env.RootCmd.SetArgs([]string{"config", "validate"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid stage reference")
	}
}

// ---------------------------------------------------------------------------
// Warnings (non-fatal, exit 0)
// ---------------------------------------------------------------------------

func TestValidate_WarningsOnly_ExitZero(t *testing.T) {
	env := newTestEnv(t)

	// Add an unknown field to local config to trigger a warning.
	env.writeTierConfig(t, "local", map[string]any{
		"identity":      map[string]any{"user": "test", "machine": "machine"},
		"bogus_setting": "should warn",
	})

	env.RootCmd.SetArgs([]string{"config", "validate"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("warnings should not cause failure, got: %v", err)
	}
}

func TestValidate_WarningsOnly_HumanOutput(t *testing.T) {
	env := newTestEnv(t)

	env.writeTierConfig(t, "local", map[string]any{
		"identity":      map[string]any{"user": "test", "machine": "machine"},
		"bogus_setting": "should warn",
	})

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "validate"})
	err := env.RootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	_ = r.Close()

	out := string(buf[:n])
	if !strings.Contains(out, "warning") {
		t.Errorf("expected 'warning' in output, got: %s", out)
	}
	if !strings.Contains(out, "0 errors") {
		t.Errorf("expected '0 errors' in output, got: %s", out)
	}
}

func TestValidate_WarningsOnly_JSON(t *testing.T) {
	env := newTestEnv(t)

	env.writeTierConfig(t, "local", map[string]any{
		"identity":      map[string]any{"user": "test", "machine": "machine"},
		"bogus_setting": "should warn",
	})

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "validate", "--json"})
	err := env.RootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	_ = r.Close()

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Issues       []validationIssue `json:"issues"`
			ErrorCount   int               `json:"error_count"`
			WarningCount int               `json:"warning_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf[:n], &envelope); err != nil {
		t.Fatalf("parsing JSON: %v\nraw: %s", err, string(buf[:n]))
	}
	if !envelope.OK {
		t.Error("expected ok=true for warnings-only")
	}
	if envelope.Data.ErrorCount != 0 {
		t.Errorf("error_count = %d, want 0", envelope.Data.ErrorCount)
	}
	if envelope.Data.WarningCount == 0 {
		t.Error("expected at least 1 warning")
	}
	hasWarning := false
	for _, issue := range envelope.Data.Issues {
		if issue.Severity == "warning" {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Error("expected a warning-severity issue")
	}
}

// ---------------------------------------------------------------------------
// --full flag (identity and cross-reference checks)
// ---------------------------------------------------------------------------

func TestValidate_Full_ValidConfig(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"config", "validate", "--full"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("--full with valid config should succeed: %v", err)
	}
}

func TestValidate_Full_MissingIdentity_Human(t *testing.T) {
	env := newTestEnv(t)

	// Remove identity so Validate (full) flags it.
	env.writeTierConfig(t, "local", map[string]any{})

	oldOut := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	oldErr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	env.RootCmd.SetArgs([]string{"config", "validate", "--full"})
	execErr := env.RootCmd.Execute()

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr

	if execErr == nil {
		t.Fatal("expected error for missing identity with --full")
	}
	if !strings.Contains(execErr.Error(), "validation failed") {
		t.Errorf("expected 'validation failed' in error, got: %v", execErr)
	}

	var buf [8192]byte
	n, _ := rErr.Read(buf[:])
	_ = rErr.Close()
	_ = rOut.Close()

	stderr := string(buf[:n])
	if !strings.Contains(stderr, "error") || !strings.Contains(stderr, "identity") {
		t.Errorf("expected stderr to mention identity error, got: %s", stderr)
	}
}

func TestValidate_Full_MissingIdentity_JSON(t *testing.T) {
	env := newTestEnv(t)

	env.writeTierConfig(t, "local", map[string]any{})

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "validate", "--full", "--json"})
	execErr := env.RootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	if execErr == nil {
		t.Fatal("expected error for missing identity with --full --json")
	}

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	_ = r.Close()

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Issues       []validationIssue `json:"issues"`
			ErrorCount   int               `json:"error_count"`
			WarningCount int               `json:"warning_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf[:n], &envelope); err != nil {
		t.Fatalf("parsing JSON: %v\nraw: %s", err, string(buf[:n]))
	}
	// The JSON envelope always uses output.Ok, so ok=true even with errors.
	// Errors are signaled via error_count > 0 and the cobra return error.
	if !envelope.OK {
		t.Error("expected ok=true in JSON envelope")
	}
	if envelope.Data.ErrorCount == 0 {
		t.Error("expected at least 1 error")
	}
	foundIdentity := false
	for _, issue := range envelope.Data.Issues {
		if issue.Severity == "error" && strings.Contains(issue.Description, "identity") {
			foundIdentity = true
		}
	}
	if !foundIdentity {
		t.Error("expected an error about identity in issues")
	}
}

func TestValidate_Full_CategoryIsValidation(t *testing.T) {
	env := newTestEnv(t)

	// Remove identity so --full produces errors with "validation" category.
	env.writeTierConfig(t, "local", map[string]any{})

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "validate", "--full", "--json"})
	_ = env.RootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	_ = r.Close()

	var envelope struct {
		Data struct {
			Issues []validationIssue `json:"issues"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf[:n], &envelope); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}
	for _, issue := range envelope.Data.Issues {
		if issue.Severity == "error" && issue.Category != "validation" {
			t.Errorf("expected category %q for --full errors, got %q", "validation", issue.Category)
		}
	}
}

// ---------------------------------------------------------------------------
// Missing / unreadable config
// ---------------------------------------------------------------------------

func TestValidate_MalformedJSON(t *testing.T) {
	env := newTestEnv(t)

	path := filepath.Join(env.WolfcastleDir, "system", "local", "config.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("writing malformed config: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "validate"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected 'loading config' in error, got: %v", err)
	}
}

func TestValidate_UnreadableConfig(t *testing.T) {
	env := newTestEnv(t)

	// Replace config.json with a directory to force a read error.
	path := filepath.Join(env.WolfcastleDir, "system", "local", "config.json")
	_ = os.Remove(path)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("creating directory in place of config: %v", err)
	}

	env.RootCmd.SetArgs([]string{"config", "validate"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unreadable config")
	}
}

func TestValidate_AllTiersMissing(t *testing.T) {
	env := newTestEnv(t)
	env.removeTierConfig(t, "base")
	env.removeTierConfig(t, "custom")
	env.removeTierConfig(t, "local")

	// With all tiers missing, defaults are used. Defaults pass ValidateStructure,
	// so the command should succeed (without --full, since identity is nil in defaults).
	env.RootCmd.SetArgs([]string{"config", "validate"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("expected success with defaults only, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate subcommand is registered
// ---------------------------------------------------------------------------

func TestRegister_HasValidateSubcommand(t *testing.T) {
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
		if sub.Name() == "validate" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("validate subcommand not registered under config")
	}
}

func TestValidate_HasFullFlag(t *testing.T) {
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

	var validateCmd *cobra.Command
	for _, sub := range configCmd.Commands() {
		if sub.Name() == "validate" {
			validateCmd = sub
			break
		}
	}
	if validateCmd == nil {
		t.Fatal("validate subcommand not found")
	}

	flag := validateCmd.Flags().Lookup("full")
	if flag == nil {
		t.Fatal("--full flag not registered on validate command")
	}
}

// ---------------------------------------------------------------------------
// Mixed warnings and errors (--full)
// ---------------------------------------------------------------------------

func TestValidate_Full_WarningsAndErrors_JSON(t *testing.T) {
	env := newTestEnv(t)

	// Unknown field triggers a warning, missing identity triggers an error.
	env.writeTierConfig(t, "local", map[string]any{
		"totally_unknown": true,
	})

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "validate", "--full", "--json"})
	_ = env.RootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	_ = r.Close()

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Issues       []validationIssue `json:"issues"`
			ErrorCount   int               `json:"error_count"`
			WarningCount int               `json:"warning_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf[:n], &envelope); err != nil {
		t.Fatalf("parsing JSON: %v\nraw: %s", err, string(buf[:n]))
	}
	if !envelope.OK {
		t.Error("expected ok=true in JSON envelope")
	}
	if envelope.Data.ErrorCount == 0 {
		t.Error("expected at least 1 error")
	}
	if envelope.Data.WarningCount == 0 {
		t.Error("expected at least 1 warning")
	}
	if len(envelope.Data.Issues) != envelope.Data.ErrorCount+envelope.Data.WarningCount {
		t.Errorf("issue count mismatch: %d issues vs %d errors + %d warnings",
			len(envelope.Data.Issues), envelope.Data.ErrorCount, envelope.Data.WarningCount)
	}
}

// ---------------------------------------------------------------------------
// Without --full, structural re-check finds nothing (Load already validated)
// ---------------------------------------------------------------------------

func TestValidate_NoFull_StructureCategory(t *testing.T) {
	env := newTestEnv(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "validate", "--json"})
	err := env.RootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	_ = r.Close()

	var envelope struct {
		Data struct {
			Issues     []validationIssue `json:"issues"`
			ErrorCount int               `json:"error_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf[:n], &envelope); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}
	// Since Load() already ran ValidateStructure successfully, re-running it
	// should find zero errors.
	if envelope.Data.ErrorCount != 0 {
		t.Errorf("expected 0 errors from structural recheck, got %d", envelope.Data.ErrorCount)
	}
}

// ---------------------------------------------------------------------------
// JSON envelope action field
// ---------------------------------------------------------------------------

func TestValidate_JSON_ActionField(t *testing.T) {
	env := newTestEnv(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"config", "validate", "--json"})
	err := env.RootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	_ = r.Close()

	var envelope struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(buf[:n], &envelope); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}
	if envelope.Action != "config_validate" {
		t.Errorf("action = %q, want %q", envelope.Action, "config_validate")
	}
}
