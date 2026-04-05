package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// newLogCmd: flag registration and RunE path coverage
// ═══════════════════════════════════════════════════════════════════════════

func TestLogCmd_NoLogFiles(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)

	// No log files present. Should return an error (exit 1).
	env.RootCmd.SetArgs([]string{"log"})
	if err := env.RootCmd.Execute(); err == nil {
		t.Fatal("expected error when no log files exist")
	}
}

func TestLogCmd_SummaryReplay(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"stage_start","stage":"execute","node":"test/node","task":"task-0001","timestamp":"2026-03-21T18:00:00Z"}`+"\n"+
			`{"type":"stage_complete","stage":"execute","node":"test/node","task":"task-0001","exit_code":0,"timestamp":"2026-03-21T18:01:22Z"}`+"\n"), 0644)

	env.RootCmd.SetArgs([]string{"log"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("log summary replay failed: %v", err)
	}
}

func TestLogCmd_ThoughtsMode(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"assistant","text":"I'll start by reading the file...","timestamp":"2026-03-21T18:00:01Z"}`+"\n"), 0644)

	env.RootCmd.SetArgs([]string{"log", "--thoughts"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("log --thoughts failed: %v", err)
	}
}

func TestLogCmd_InterleavedMode(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"stage_start","stage":"execute","node":"test/node","task":"task-0001","timestamp":"2026-03-21T18:00:00Z"}`+"\n"+
			`{"type":"assistant","text":"Working on it...","timestamp":"2026-03-21T18:00:01Z"}`+"\n"+
			`{"type":"stage_complete","stage":"execute","node":"test/node","task":"task-0001","exit_code":0,"timestamp":"2026-03-21T18:01:22Z"}`+"\n"), 0644)

	env.RootCmd.SetArgs([]string{"log", "--interleaved"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("log --interleaved failed: %v", err)
	}
}

func TestLogCmd_JSONMode(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","scope":"test","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	env.RootCmd.SetArgs([]string{"log", "--json"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("log --json failed: %v", err)
	}
}

func TestLogCmd_SessionFlag(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)

	// Two sessions: first starts at iteration 1, second starts at iteration 1 with later timestamp.
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260320T10-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","scope":"old","timestamp":"2026-03-20T10:00:00Z"}`+"\n"), 0644)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","scope":"new","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	// --session 1 should replay the older session.
	env.RootCmd.SetArgs([]string{"log", "--session", "1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("log --session 1 failed: %v", err)
	}
}

func TestLogCmd_FollowAlias_Removed(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","scope":"test","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	// The "follow" alias was removed in v0.5.0; the command should fail.
	env.RootCmd.SetArgs([]string{"follow"})
	if err := env.RootCmd.Execute(); err == nil {
		t.Fatal("expected error: follow alias should no longer be recognized")
	}
}

func TestLogCmd_FollowFlag(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","scope":"test","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	done := make(chan error, 1)
	go func() {
		env.RootCmd.SetArgs([]string{"log", "--follow"})
		done <- env.RootCmd.Execute()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("log --follow failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		// Expected: still streaming.
	}
}

func TestLogCmd_FollowThoughts(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"assistant","text":"thinking...","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	done := make(chan error, 1)
	go func() {
		env.RootCmd.SetArgs([]string{"log", "--follow", "--thoughts"})
		done <- env.RootCmd.Execute()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("log --follow --thoughts failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		// Expected: still streaming.
	}
}

func TestLogCmd_FollowInterleaved(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"stage_start","stage":"plan","node":"n","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	done := make(chan error, 1)
	go func() {
		env.RootCmd.SetArgs([]string{"log", "-f", "-i"})
		done <- env.RootCmd.Execute()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("log -f -i failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		// Expected: still streaming.
	}
}

func TestLogCmd_FollowJSON(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","scope":"test","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	done := make(chan error, 1)
	go func() {
		env.RootCmd.SetArgs([]string{"log", "--follow", "--json"})
		done <- env.RootCmd.Execute()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("log --follow --json failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		// Expected: still streaming.
	}
}
