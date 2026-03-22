package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaults_GitCommitFieldsDefaultTrue(t *testing.T) {
	t.Parallel()
	cfg := Defaults()

	if !cfg.Git.CommitOnSuccess {
		t.Error("expected commit_on_success to default to true")
	}
	if !cfg.Git.CommitOnFailure {
		t.Error("expected commit_on_failure to default to true")
	}
	if !cfg.Git.CommitState {
		t.Error("expected commit_state to default to true")
	}
}

func TestDefaults_SkipHooksDefaultFalse(t *testing.T) {
	t.Parallel()
	cfg := Defaults()

	if cfg.Git.SkipHooks {
		t.Error("expected skip_hooks to default to false")
	}
}

func TestGitConfig_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := GitConfig{
		AutoCommit:          true,
		CommitOnSuccess:     true,
		CommitOnFailure:     false,
		CommitState:         true,
		CommitMessageFormat: "{action}: {summary}",
		VerifyBranch:        true,
		SkipHooks:           true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded GitConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", decoded, original)
	}
}

func TestGitConfig_SkipHooksDeserializesFromJSON(t *testing.T) {
	t.Parallel()

	input := `{"skip_hooks": true}`
	var gc GitConfig
	if err := json.Unmarshal([]byte(input), &gc); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !gc.SkipHooks {
		t.Error("expected skip_hooks to be true after deserializing from JSON")
	}
}

func TestGitConfig_NewFieldsDeserializeFromJSON(t *testing.T) {
	t.Parallel()

	input := `{
		"auto_commit": true,
		"commit_on_success": false,
		"commit_on_failure": true,
		"commit_state": false,
		"skip_hooks": true
	}`
	var gc GitConfig
	if err := json.Unmarshal([]byte(input), &gc); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if gc.AutoCommit != true {
		t.Error("auto_commit should be true")
	}
	if gc.CommitOnSuccess != false {
		t.Error("commit_on_success should be false")
	}
	if gc.CommitOnFailure != true {
		t.Error("commit_on_failure should be true")
	}
	if gc.CommitState != false {
		t.Error("commit_state should be false")
	}
	if gc.SkipHooks != true {
		t.Error("skip_hooks should be true")
	}
}

func TestLoad_AutoCommitFalse_ParsesFineGrainedFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(dir, "system", "custom"), 0755)
	configJSON := `{
		"git": {
			"auto_commit": false,
			"commit_on_success": false,
			"commit_on_failure": false,
			"commit_state": false
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "system", "custom", "config.json"), []byte(configJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Git.AutoCommit {
		t.Error("expected auto_commit to be false")
	}
	if cfg.Git.CommitOnSuccess {
		t.Error("expected commit_on_success to be false from override")
	}
	if cfg.Git.CommitOnFailure {
		t.Error("expected commit_on_failure to be false from override")
	}
	if cfg.Git.CommitState {
		t.Error("expected commit_state to be false from override")
	}
}

func TestValidateStructure_CatchesNoOpGitCommitConfig(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Git.AutoCommit = true
	cfg.Git.CommitOnSuccess = false
	cfg.Git.CommitOnFailure = false

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error when auto_commit is true but both commit_on_success and commit_on_failure are false")
	}
	if !strings.Contains(err.Error(), "commit_on_success") || !strings.Contains(err.Error(), "commit_on_failure") {
		t.Errorf("expected error to mention commit fields, got: %v", err)
	}
}

func TestValidateStructure_AcceptsAutoCommitFalseWithBothCommitsFalse(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Git.AutoCommit = false
	cfg.Git.CommitOnSuccess = false
	cfg.Git.CommitOnFailure = false

	if err := ValidateStructure(cfg); err != nil {
		t.Errorf("auto_commit=false should skip the no-op check, got: %v", err)
	}
}

func TestGitConfig_JSONFieldNames(t *testing.T) {
	t.Parallel()

	gc := GitConfig{
		CommitOnSuccess: true,
		CommitOnFailure: true,
		CommitState:     true,
		SkipHooks:       true,
	}
	data, err := json.Marshal(gc)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	raw := string(data)

	for _, field := range []string{"commit_on_success", "commit_on_failure", "commit_state", "skip_hooks"} {
		if !strings.Contains(raw, field) {
			t.Errorf("expected JSON to contain field %q, got: %s", field, raw)
		}
	}
}
