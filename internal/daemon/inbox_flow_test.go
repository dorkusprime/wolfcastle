package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// TestRunIntakeStage_ProcessesNewItems verifies that the intake stage
// reads new inbox items, invokes the model, and marks them as "filed".
func TestRunIntakeStage_ProcessesNewItems(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	mockModel := config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", `cat > /dev/null; echo "WOLFCASTLE_INTAKE_COMPLETE"`},
	}
	d.Config.Models["mid"] = mockModel
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"intake":  {Model: "mid", PromptFile: "stages/intake.md"},
		"execute": {Model: "echo", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"intake", "execute"}

	writePromptFile(t, d.WolfcastleDir, "stages/intake.md")

	// Write an inbox with a "new" item
	projDir := d.Store.Dir()
	inboxData := state.InboxFile{
		Items: []state.InboxItem{
			{Text: "build a feature", Status: "new", Timestamp: clock.New().Now().Format("2006-01-02T15:04:05Z")},
		},
	}
	data, _ := json.MarshalIndent(inboxData, "", "  ")
	_ = os.WriteFile(filepath.Join(projDir, "inbox.json"), data, 0644)

	// Run the intake stage directly
	stage := config.PipelineStage{Model: "mid", PromptFile: "stages/intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Fatalf("intake stage error: %v", err)
	}

	// Check that inbox items were processed (status changed to "filed")
	updatedInbox, err := state.LoadInbox(filepath.Join(projDir, "inbox.json"))
	if err != nil {
		t.Fatalf("loading inbox after processing: %v", err)
	}
	if len(updatedInbox.Items) == 0 {
		t.Fatal("inbox should still have items")
	}
	if updatedInbox.Items[0].Status != "filed" {
		t.Errorf("inbox item should be 'filed' after intake stage, got %q", updatedInbox.Items[0].Status)
	}
}

// TestRunIntakeStage_SkipsWhenEmpty verifies that the intake stage
// is a no-op when the inbox has no new items.
func TestRunIntakeStage_SkipsWhenEmpty(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "stages/intake.md")

	// No inbox file exists
	stage := config.PipelineStage{Model: "echo", PromptFile: "stages/intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRunIntakeStage_SkipsWhenAllFiled verifies that already-filed
// items don't trigger re-processing.
func TestRunIntakeStage_SkipsWhenAllFiled(t *testing.T) {
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "stages/intake.md")

	projDir := d.Store.Dir()
	inboxData := state.InboxFile{
		Items: []state.InboxItem{
			{Text: "old idea", Status: "filed", Timestamp: clock.New().Now().Format("2006-01-02T15:04:05Z")},
		},
	}
	data, _ := json.MarshalIndent(inboxData, "", "  ")
	_ = os.WriteFile(filepath.Join(projDir, "inbox.json"), data, 0644)

	stage := config.PipelineStage{Model: "echo", PromptFile: "stages/intake.md"}
	if err := d.runIntakeStage(context.Background(), stage); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRunInboxWithFsnotify_ReactsToFileChange verifies that the fsnotify
// watcher detects inbox.json writes and processes new items.
func TestRunInboxWithFsnotify_ReactsToFileChange(t *testing.T) {
	d := testDaemon(t)
	d.Config.Models["mid"] = config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", `cat > /dev/null; echo "done"`},
	}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"intake":  {Model: "mid", PromptFile: "stages/intake.md"},
		"execute": {Model: "echo", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"intake", "execute"}
	writePromptFile(t, d.WolfcastleDir, "stages/intake.md")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use the full runInboxLoop which tries fsnotify first
	go d.runInboxLoop(ctx)

	// Give the watcher a moment to initialize
	time.Sleep(100 * time.Millisecond)

	// Write an inbox item; fsnotify should detect the write
	projDir := d.Store.Dir()
	inboxPath := filepath.Join(projDir, "inbox.json")
	inboxData := state.InboxFile{
		Items: []state.InboxItem{
			{Text: "fsnotify test", Status: "new", Timestamp: clock.New().Now().Format("2006-01-02T15:04:05Z")},
		},
	}
	data, _ := json.MarshalIndent(inboxData, "", "  ")
	_ = os.WriteFile(inboxPath, data, 0644)

	// Wait for processing
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for fsnotify to process inbox")
		default:
		}

		updated, err := state.LoadInbox(inboxPath)
		if err == nil && len(updated.Items) > 0 && updated.Items[0].Status == "filed" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
}

// TestRunInboxLoop_ProcessesItemFromGoroutine verifies the parallel
// inbox goroutine picks up new items and processes them.
func TestRunInboxLoop_ProcessesItemFromGoroutine(t *testing.T) {
	d := testDaemon(t)
	d.Config.Daemon.InboxPollIntervalSeconds = 1
	d.Config.Models["mid"] = config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", `cat > /dev/null; echo "done"`},
	}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"intake":  {Model: "mid", PromptFile: "stages/intake.md"},
		"execute": {Model: "echo", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"intake", "execute"}
	writePromptFile(t, d.WolfcastleDir, "stages/intake.md")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the inbox loop in a goroutine
	go d.runInboxLoop(ctx)

	// Add an inbox item from this goroutine
	projDir := d.Store.Dir()
	inboxPath := filepath.Join(projDir, "inbox.json")
	inboxData := state.InboxFile{
		Items: []state.InboxItem{
			{Text: "async idea", Status: "new", Timestamp: clock.New().Now().Format("2006-01-02T15:04:05Z")},
		},
	}
	data, _ := json.MarshalIndent(inboxData, "", "  ")
	_ = os.WriteFile(inboxPath, data, 0644)

	// Wait for the goroutine to process it
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for inbox item to be processed")
		default:
		}

		updated, err := state.LoadInbox(inboxPath)
		if err == nil && len(updated.Items) > 0 && updated.Items[0].Status == "filed" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
}
