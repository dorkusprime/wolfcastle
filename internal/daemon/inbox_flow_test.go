package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// TestProcessInboxIfNeeded_CreatesTasksBeforeNavigation verifies that
// inbox items are processed (expand + file) before navigation, so
// that new tasks appear in the tree for the daemon to find.
func TestProcessInboxIfNeeded_CreatesTasksBeforeNavigation(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	d.Config.Daemon.MaxIterations = 3
	_ = d.Logger.StartIteration()

	// Add expand and file stages to the pipeline
	mockModel := config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", `cat > /dev/null; printf '## Build a Feature\n\n**Scope:** Build the feature\n\n**Suggested Tasks:**\n1. Implement it\n'`},
	}
	d.Config.Models["fast"] = mockModel
	d.Config.Models["mid"] = mockModel
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "expand", Model: "fast", PromptFile: "expand.md"},
		{Name: "file", Model: "mid", PromptFile: "file.md"},
		{Name: "execute", Model: "echo", PromptFile: "execute.md"},
	}

	writePromptFile(t, d.WolfcastleDir, "expand.md")
	writePromptFile(t, d.WolfcastleDir, "file.md")

	// Write an inbox with a "new" item
	projDir := d.Resolver.ProjectsDir()
	inboxData := state.InboxFile{
		Items: []state.InboxItem{
			{Text: "build a feature", Status: "new", Timestamp: clock.New().Now().Format("2006-01-02T15:04:05Z")},
		},
	}
	data, _ := json.MarshalIndent(inboxData, "", "  ")
	_ = os.WriteFile(filepath.Join(projDir, "inbox.json"), data, 0644)

	// Run processInboxIfNeeded
	d.processInboxIfNeeded(context.Background())

	// Check that inbox items were processed (status changed from "new")
	updatedInbox, err := state.LoadInbox(filepath.Join(projDir, "inbox.json"))
	if err != nil {
		t.Fatalf("loading inbox after processing: %v", err)
	}
	if len(updatedInbox.Items) == 0 {
		t.Fatal("inbox should still have items")
	}
	if updatedInbox.Items[0].Status == "new" {
		t.Error("inbox item should no longer be 'new' after expand stage")
	}
}

// TestProcessInboxIfNeeded_SkipsWhenEmpty verifies that processInboxIfNeeded
// is a no-op when the inbox is empty or doesn't exist.
func TestProcessInboxIfNeeded_SkipsWhenEmpty(t *testing.T) {
	d := testDaemon(t)

	// No inbox file exists
	d.processInboxIfNeeded(context.Background())
	// Should not panic or error
}

// TestProcessInboxIfNeeded_SkipsWhenAllFiled verifies that already-filed
// items don't trigger re-processing.
func TestProcessInboxIfNeeded_SkipsWhenAllFiled(t *testing.T) {
	d := testDaemon(t)

	projDir := d.Resolver.ProjectsDir()
	inboxData := state.InboxFile{
		Items: []state.InboxItem{
			{Text: "old idea", Status: "filed", Timestamp: clock.New().Now().Format("2006-01-02T15:04:05Z")},
		},
	}
	data, _ := json.MarshalIndent(inboxData, "", "  ")
	_ = os.WriteFile(filepath.Join(projDir, "inbox.json"), data, 0644)

	d.processInboxIfNeeded(context.Background())
	// Should not invoke any models (no new or expanded items)
}

func testDaemonWithLogger(t *testing.T) *Daemon {
	t.Helper()
	d := testDaemon(t)
	logDir := filepath.Join(d.WolfcastleDir, "logs")
	_ = os.MkdirAll(logDir, 0755)
	logger, err := logging.NewLogger(logDir)
	if err != nil {
		t.Fatal(err)
	}
	d.Logger = logger
	return d
}

// Ensure testDaemon is available (it's in daemon_test.go)
var _ = func() *Daemon {
	return &Daemon{
		Config:   &config.Config{},
		Resolver: &tree.Resolver{},
		Clock:    clock.New(),
	}
}
