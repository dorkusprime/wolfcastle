package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// captureStdout redirects os.Stdout to a pipe for the duration of fn,
// then returns whatever was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = origStdout

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestTryRecoverRootIndex_NonexistentFile(t *testing.T) {
	_, err := tryRecoverRootIndex("/no/such/path/state.json", false)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "reading root index for recovery") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestTryRecoverRootIndex_RecoveryFailure(t *testing.T) {
	// Data that is not JSON and cannot be salvaged by any recovery strategy:
	// doesn't start with { or [, so tryStripTrailing and tryCloseTruncated
	// both bail immediately.
	tmp := t.TempDir()
	indexPath := filepath.Join(tmp, "state.json")
	if err := os.WriteFile(indexPath, []byte("not json at all"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := tryRecoverRootIndex(indexPath, false)
	if err == nil {
		t.Fatal("expected error for unrecoverable data")
	}
	if !strings.Contains(err.Error(), "recovery failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestTryRecoverRootIndex_SuccessNoFix(t *testing.T) {
	// Write a valid index with a BOM prefix so recovery has something to report.
	idx := state.NewRootIndex()
	idx.Nodes["alpha"] = state.IndexEntry{
		Name:    "Alpha",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "alpha",
	}
	data, err := json.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	// Prepend UTF-8 BOM so sanitizeJSON produces an Applied step.
	bom := []byte{0xEF, 0xBB, 0xBF}
	corrupted := append(bom, data...)

	tmp := t.TempDir()
	indexPath := filepath.Join(tmp, "state.json")
	if err := os.WriteFile(indexPath, corrupted, 0644); err != nil {
		t.Fatal(err)
	}

	var recovered *state.RootIndex
	output := captureStdout(t, func() {
		var recoverErr error
		recovered, recoverErr = tryRecoverRootIndex(indexPath, false)
		if recoverErr != nil {
			t.Fatalf("unexpected error: %v", recoverErr)
		}
	})

	if recovered == nil {
		t.Fatal("expected non-nil recovered index")
	}
	if _, ok := recovered.Nodes["alpha"]; !ok {
		t.Error("expected node 'alpha' in recovered index")
	}
	if !strings.Contains(output, "Recovered root index from malformed JSON") {
		t.Error("expected recovery header in output")
	}
	if !strings.Contains(output, "stripped UTF-8 BOM") {
		t.Error("expected BOM step in output")
	}
	if !strings.Contains(output, "--fix") {
		t.Error("expected --fix suggestion when fix=false")
	}
	if strings.Contains(output, "FIXED") {
		t.Error("should not see FIXED message when fix=false")
	}
}

func TestTryRecoverRootIndex_SuccessWithFix(t *testing.T) {
	idx := state.NewRootIndex()
	idx.Nodes["beta"] = state.IndexEntry{
		Name:    "Beta",
		Type:    state.NodeLeaf,
		State:   state.StatusComplete,
		Address: "beta",
	}
	data, err := json.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	// Append trailing garbage so recovery strips it.
	corrupted := append(data, []byte(`}}}}extra`)...)

	tmp := t.TempDir()
	indexPath := filepath.Join(tmp, "state.json")
	if err := os.WriteFile(indexPath, corrupted, 0644); err != nil {
		t.Fatal(err)
	}

	var recovered *state.RootIndex
	output := captureStdout(t, func() {
		var recoverErr error
		recovered, recoverErr = tryRecoverRootIndex(indexPath, true)
		if recoverErr != nil {
			t.Fatalf("unexpected error: %v", recoverErr)
		}
	})

	if recovered == nil {
		t.Fatal("expected non-nil recovered index")
	}
	if _, ok := recovered.Nodes["beta"]; !ok {
		t.Error("expected node 'beta' in recovered index")
	}
	if !strings.Contains(output, "FIXED") {
		t.Error("expected FIXED message when fix=true")
	}
	if strings.Contains(output, "--fix") {
		t.Error("should not see --fix suggestion when fix=true")
	}

	// Verify the file was actually written back.
	reloaded, err := state.LoadRootIndex(indexPath)
	if err != nil {
		t.Fatalf("failed to reload fixed index: %v", err)
	}
	if _, ok := reloaded.Nodes["beta"]; !ok {
		t.Error("expected node 'beta' in reloaded index")
	}
}

func TestTryRecoverRootIndex_EmptyAppliedAndLost(t *testing.T) {
	// A perfectly valid index file: recovery succeeds with no steps applied
	// and no data lost (sanitizeJSON finds nothing to fix, direct unmarshal works).
	idx := state.NewRootIndex()
	data, err := json.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	indexPath := filepath.Join(tmp, "state.json")
	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	var recovered *state.RootIndex
	output := captureStdout(t, func() {
		var recoverErr error
		recovered, recoverErr = tryRecoverRootIndex(indexPath, false)
		if recoverErr != nil {
			t.Fatalf("unexpected error: %v", recoverErr)
		}
	})

	if recovered == nil {
		t.Fatal("expected non-nil recovered index")
	}
	if !strings.Contains(output, "Recovered root index from malformed JSON") {
		t.Error("expected recovery header in output")
	}
	// With clean JSON, there should be no Applied steps or LOST lines printed
	// beyond the header and the --fix suggestion.
	if strings.Contains(output, "LOST:") {
		t.Error("expected no LOST lines for clean recovery")
	}
}
