package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/logging"
)

// ---------------------------------------------------------------------------
// status.go: LoadRootIndex error
// ---------------------------------------------------------------------------

func TestStatusCmd_LoadRootIndexError(t *testing.T) {
	env := newStatusTestEnv(t)

	// Corrupt the root index
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), []byte("corrupt json{{"), 0644)

	env.RootCmd.SetArgs([]string{"status"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when root index is corrupt")
	}
}

// ---------------------------------------------------------------------------
// status.go: namespace with broken state.json
// ---------------------------------------------------------------------------

func TestShowAllStatus_NamespaceWithBrokenStateJSON(t *testing.T) {
	env := newStatusTestEnv(t)

	// Create a second namespace with broken state.json
	brokenDir := filepath.Join(env.WolfcastleDir, "system", "projects", "broken-ns")
	_ = os.MkdirAll(brokenDir, 0755)
	_ = os.WriteFile(filepath.Join(brokenDir, "state.json"), []byte("not valid json"), 0644)

	// showAllStatus should skip the broken namespace gracefully
	err := showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus should skip broken namespaces: %v", err)
	}
}

// ---------------------------------------------------------------------------
// status.go: no summaries in showAllStatus
// ---------------------------------------------------------------------------

func TestShowAllStatus_NoSummaries(t *testing.T) {
	env := newTestEnv(t)

	// projects/ dir exists but has no namespace subdirectories at all
	// Remove the default namespace dir
	_ = os.RemoveAll(filepath.Join(env.WolfcastleDir, "system", "projects", "test-dev"))

	// No namespaces should result in "No engineer namespaces found"
	err := showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus with no namespaces failed: %v", err)
	}
}

func TestShowAllStatus_NoSummaries_JSON(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	_ = os.RemoveAll(filepath.Join(env.WolfcastleDir, "system", "projects", "test-dev"))

	err := showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus JSON with no namespaces failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// follow.go: new iteration header on file change
// The follow command contains an infinite loop, so we test the sub-functions
// directly rather than running the full command.
// ---------------------------------------------------------------------------

func TestShowHistoricalLines_ThenNewFile(t *testing.T) {
	tmp := t.TempDir()

	// Create first log file
	logFile1 := filepath.Join(tmp, "001-first.jsonl")
	_ = os.WriteFile(logFile1, []byte(`{"type":"assistant","text":"first"}`+"\n"), 0644)

	// Show historical lines from first file
	clearOffset(logFile1)
	showHistoricalLines(logFile1, 10, logging.LevelDebug)

	// Create second log file (simulates new iteration)
	logFile2 := filepath.Join(tmp, "002-second.jsonl")
	_ = os.WriteFile(logFile2, []byte(`{"type":"assistant","text":"second"}`+"\n"), 0644)

	// Tail the second file from the beginning
	clearOffset(logFile2)
	err := tailFileStreaming(logFile2, logging.LevelDebug)
	if err != nil {
		t.Fatalf("tailFileStreaming second file failed: %v", err)
	}

	offset := getOffset(logFile2)
	if offset == 0 {
		t.Error("expected offset to advance after reading second file")
	}
}
