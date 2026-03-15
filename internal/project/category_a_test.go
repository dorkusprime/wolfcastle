package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// scaffold.go — ReScaffold: invalid JSON in local/config.json
// ═══════════════════════════════════════════════════════════════════════════

func TestReScaffold_InvalidJSONInLocalConfig(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")

	// First scaffold normally to set up the directory structure
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Now corrupt local/config.json with invalid JSON
	localPath := filepath.Join(dir, "local", "config.json")
	if err := os.WriteFile(localPath, []byte("NOT VALID JSON{{{"), 0644); err != nil {
		t.Fatal(err)
	}

	err := ReScaffold(dir)
	if err == nil {
		t.Fatal("expected error when local/config.json contains invalid JSON")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("expected 'not valid JSON' in error, got: %v", err)
	}
}
