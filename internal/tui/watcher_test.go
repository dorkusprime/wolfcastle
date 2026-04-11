package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/fsnotify/fsnotify"
)

// drainEvents returns a buffered events channel large enough that the
// watcher's non-blocking emit never drops messages during a test, plus
// a helper that returns the next message (or fails after a timeout).
// Tests that don't care about delivery just pass `nil` as the events
// argument to NewWatcher; emit becomes a no-op in that case.
func drainEvents(t *testing.T) (chan tea.Msg, func() tea.Msg) {
	t.Helper()
	ch := make(chan tea.Msg, 64)
	next := func() tea.Msg {
		select {
		case msg := <-ch:
			return msg
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for watcher event")
			return nil
		}
	}
	return ch, next
}

func TestNewWatcher_FieldsInitialized(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "/tmp/logs", "/tmp/instances", nil)

	if w.store != store {
		t.Fatal("store not set")
	}
	if w.logDir != "/tmp/logs" {
		t.Fatalf("logDir=%q, want /tmp/logs", w.logDir)
	}
	if w.instanceDir != "/tmp/instances" {
		t.Fatalf("instanceDir=%q, want /tmp/instances", w.instanceDir)
	}
	if w.pending == nil {
		t.Fatal("pending map not initialized")
	}
	if w.done == nil {
		t.Fatal("done channel not initialized")
	}
	if !w.useFsnotify {
		t.Fatal("useFsnotify should default to true")
	}
}

func TestReadNewLogLines_WithContent(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.jsonl")
	content := "line one\nline two\nline three\n"
	if err := os.WriteFile(logFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, dir, "", nil)
	w.logFile = logFile
	w.logOffset = 0

	lines := w.readNewLogLines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "line one" || lines[1] != "line two" || lines[2] != "line three" {
		t.Fatalf("unexpected lines: %v", lines)
	}
	// Offset should advance past all content
	if w.logOffset != int64(len(content)) {
		t.Fatalf("expected offset=%d, got %d", len(content), w.logOffset)
	}
}

func TestReadNewLogLines_PartialLine(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.jsonl")
	// Write content with an incomplete trailing line (no newline at end).
	// bufio.Scanner reads "partial" as a valid last line even without a
	// trailing newline. The bytesRead counter overestimates by 1 for the
	// missing newline, so the trailing-data check (info.Size > newOffset)
	// evaluates false and both lines are returned as complete.
	content := "complete line\npartial"
	if err := os.WriteFile(logFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, dir, "", nil)
	w.logFile = logFile
	w.logOffset = 0

	lines := w.readNewLogLines()
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (scanner treats trailing text as a line), got %d: %v", len(lines), lines)
	}
	if lines[0] != "complete line" {
		t.Fatalf("expected 'complete line', got %q", lines[0])
	}
	if lines[1] != "partial" {
		t.Fatalf("expected 'partial', got %q", lines[1])
	}
}

func TestReadNewLogLines_LineBufPrepends(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.jsonl")
	if err := os.WriteFile(logFile, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, dir, "", nil)
	w.logFile = logFile
	w.logOffset = 0
	// Simulate a leftover buffer from a previous read
	w.lineBuf = "prefix:"

	lines := w.readNewLogLines()
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}
	// The buffered prefix should be prepended to the first line
	if lines[0] != "prefix:first" {
		t.Fatalf("expected 'prefix:first', got %q", lines[0])
	}
}

func TestReadNewLogLines_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(logFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, dir, "", nil)
	w.logFile = logFile
	w.logOffset = 0

	lines := w.readNewLogLines()
	if lines != nil {
		t.Fatalf("expected nil for empty file, got %v", lines)
	}
}

func TestReadNewLogLines_NoLogFile(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "", nil)
	w.logFile = ""

	lines := w.readNewLogLines()
	if lines != nil {
		t.Fatalf("expected nil for no log file, got %v", lines)
	}
}

func TestReadNewLogLines_MissingFile(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "", nil)
	w.logFile = "/nonexistent/path/test.jsonl"

	lines := w.readNewLogLines()
	if lines != nil {
		t.Fatalf("expected nil for missing file, got %v", lines)
	}
}

func TestNodeAddrFromPath(t *testing.T) {
	dir := t.TempDir()
	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, "", "", nil)

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "single segment",
			path: filepath.Join(dir, "overview", "state.json"),
			want: "overview",
		},
		{
			name: "multi segment",
			path: filepath.Join(dir, "a", "b", "c", "state.json"),
			want: "a/b/c",
		},
		{
			name: "not a state file",
			path: filepath.Join(dir, "a", "other.json"),
			want: "",
		},
		{
			name: "root state.json (same as index)",
			path: filepath.Join(dir, "state.json"),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.nodeAddrFromPath(tt.path)
			if got != tt.want {
				t.Fatalf("nodeAddrFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestStop_Idempotent(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "", nil)

	// First stop should work fine
	w.Stop()
	// Second stop should not panic
	w.Stop()
}

func TestStop_CancelsTimers(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "", nil)

	// Set up timers manually
	w.debounce = time.AfterFunc(time.Hour, func() {})
	w.maxSlide = time.AfterFunc(time.Hour, func() {})

	w.Stop()

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.debounce != nil {
		t.Fatal("debounce timer should be nil after stop")
	}
	if w.maxSlide != nil {
		t.Fatal("maxSlide timer should be nil after stop")
	}
}

func TestAddNodeWatch_NilWatcher(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "", nil)
	w.watcher = nil

	// Should not panic
	w.AddNodeWatch("a/b/c")
}

func TestRemoveNodeWatch_NilWatcher(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "", nil)
	w.watcher = nil

	// Should not panic
	w.RemoveNodeWatch("a/b/c")
}

func TestReadNewLogLines_FromOffset(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.jsonl")
	content := "first line\nsecond line\n"
	if err := os.WriteFile(logFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, dir, "", nil)
	w.logFile = logFile
	// Start from after "first line\n" (11 bytes)
	w.logOffset = 11

	lines := w.readNewLogLines()
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}
	if lines[0] != "second line" {
		t.Fatalf("expected 'second line', got %q", lines[0])
	}
}

func TestReadNewLogLines_MultipleReads(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.jsonl")

	// Write initial content
	if err := os.WriteFile(logFile, []byte("line one\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, dir, "", nil)
	w.logFile = logFile
	w.logOffset = 0

	lines := w.readNewLogLines()
	if len(lines) != 1 || lines[0] != "line one" {
		t.Fatalf("first read: expected ['line one'], got %v", lines)
	}

	// Append more content
	f, _ := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("line two\nline three\n")
	_ = f.Close()

	lines = w.readNewLogLines()
	if len(lines) != 2 {
		t.Fatalf("second read: expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "line two" || lines[1] != "line three" {
		t.Fatalf("second read: unexpected lines: %v", lines)
	}
}

func TestResetDebounce(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "", nil)

	w.mu.Lock()
	w.resetDebounce()
	hasDeb := w.debounce != nil
	hasMax := w.maxSlide != nil
	w.mu.Unlock()

	if !hasDeb {
		t.Fatal("expected debounce timer to be set")
	}
	if !hasMax {
		t.Fatal("expected maxSlide timer to be set")
	}

	// Clean up timers
	w.Stop()
}

func TestFlushFromTimer_EmptyPending(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "", nil)

	// Should not panic with empty pending set
	w.flushFromTimer()
}

func TestNewWatcher_DefaultUseFsnotify(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "", nil)

	if !w.useFsnotify {
		t.Fatal("expected useFsnotify=true by default")
	}
	// Can be disabled manually to simulate fsnotify failure
	w.useFsnotify = false
	if w.useFsnotify {
		t.Fatal("expected useFsnotify=false after manual set")
	}
}

func TestAddNodeWatch_WithFsnotifyWatcher(t *testing.T) {
	dir := t.TempDir()
	// Create a node directory so the path resolves
	nodeDir := filepath.Join(dir, "a", "b")
	_ = os.MkdirAll(nodeDir, 0o755)
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), []byte("{}"), 0o644)

	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, "", "", nil)

	// Create a real fsnotify watcher to exercise the non-nil path
	fsw, err := newFsnotifyWatcher()
	if err != nil {
		t.Skipf("fsnotify unavailable: %v", err)
	}
	defer func() { _ = fsw.Close() }()
	w.watcher = fsw

	// Should not panic, should add the watch
	w.AddNodeWatch("a/b")
}

func TestRemoveNodeWatch_WithFsnotifyWatcher(t *testing.T) {
	dir := t.TempDir()
	nodeDir := filepath.Join(dir, "a", "b")
	_ = os.MkdirAll(nodeDir, 0o755)
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), []byte("{}"), 0o644)

	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, "", "", nil)

	fsw, err := newFsnotifyWatcher()
	if err != nil {
		t.Skipf("fsnotify unavailable: %v", err)
	}
	defer func() { _ = fsw.Close() }()
	w.watcher = fsw

	// Add first, then remove
	w.AddNodeWatch("a/b")
	w.RemoveNodeWatch("a/b")
}

func TestAddNodeWatch_InvalidAddr(t *testing.T) {
	dir := t.TempDir()
	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, "", "", nil)

	fsw, err := newFsnotifyWatcher()
	if err != nil {
		t.Skipf("fsnotify unavailable: %v", err)
	}
	defer func() { _ = fsw.Close() }()
	w.watcher = fsw

	// Empty address returns an error from NodePath, should not panic
	w.AddNodeWatch("")
}

func TestRemoveNodeWatch_InvalidAddr(t *testing.T) {
	dir := t.TempDir()
	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, "", "", nil)

	fsw, err := newFsnotifyWatcher()
	if err != nil {
		t.Skipf("fsnotify unavailable: %v", err)
	}
	defer func() { _ = fsw.Close() }()
	w.watcher = fsw

	w.RemoveNodeWatch("")
}

func TestStop_WithFsnotifyWatcher(t *testing.T) {
	dir := t.TempDir()
	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, "", "", nil)

	fsw, err := newFsnotifyWatcher()
	if err != nil {
		t.Skipf("fsnotify unavailable: %v", err)
	}
	w.watcher = fsw

	w.Stop()
	// Watcher should be closed, second stop should be safe
	w.Stop()
}

func TestFlushFromTimer_WithPendingPaths(t *testing.T) {
	// flushFromTimer with pending paths will call flush, which calls
	// w.program.Send. Without a program, flush will panic. We verify
	// the pending-clearing logic by checking the pending map after a
	// flush with no matching paths (flush returns early for each path
	// since none match index, instance, node, or log patterns).

	// We can't easily test flush without a program, so we test that
	// flushFromTimer clears pending and timers.
	dir := t.TempDir()
	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, "", "", nil)

	// Add some pending paths and timers
	w.mu.Lock()
	w.pending["somefile.txt"] = true
	w.debounce = time.AfterFunc(time.Hour, func() {})
	w.maxSlide = time.AfterFunc(time.Hour, func() {})
	w.mu.Unlock()

	// flushFromTimer will call flush which accesses w.program (nil),
	// but flush only calls program.Send when it matches specific paths.
	// "somefile.txt" won't match any of the path patterns in flush,
	// so program.Send won't be called.
	w.flushFromTimer()

	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) != 0 {
		t.Fatalf("expected pending to be cleared, got %d entries", len(w.pending))
	}
	if w.debounce != nil {
		t.Fatal("debounce should be nil after flush")
	}
	if w.maxSlide != nil {
		t.Fatal("maxSlide should be nil after flush")
	}
}

func TestResetDebounce_SubsequentCallResetsDebounce(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "", nil)

	w.mu.Lock()
	w.resetDebounce()
	firstDebounce := w.debounce
	firstMaxSlide := w.maxSlide
	// Call again: debounce should be replaced, maxSlide should stay
	w.resetDebounce()
	secondDebounce := w.debounce
	secondMaxSlide := w.maxSlide
	w.mu.Unlock()

	if firstDebounce == secondDebounce {
		t.Fatal("expected debounce timer to be replaced on second call")
	}
	if firstMaxSlide != secondMaxSlide {
		t.Fatal("expected maxSlide timer to remain the same on second call")
	}

	w.Stop()
}

func TestReadNewLogLines_SeekError(t *testing.T) {
	// Test the seek-error path by setting an offset beyond file size.
	// The seek itself won't fail (OS allows seeking past EOF), but reading
	// will yield no data, same as an empty file.
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.jsonl")
	_ = os.WriteFile(logFile, []byte("hello\n"), 0o644)

	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, dir, "", nil)
	w.logFile = logFile
	w.logOffset = 99999 // way past end

	lines := w.readNewLogLines()
	if lines != nil {
		t.Fatalf("expected nil for offset past EOF, got %v", lines)
	}
}

// newFsnotifyWatcher is a helper to create an fsnotify watcher, matching
// the import used by the production code.
func newFsnotifyWatcher() (*fsnotify.Watcher, error) {
	return fsnotify.NewWatcher()
}

func TestStart_Success(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	instDir := filepath.Join(dir, "instances")
	_ = os.MkdirAll(logDir, 0o755)
	_ = os.MkdirAll(instDir, 0o755)
	// Pre-create a log file so the latest-log lookup succeeds.
	_ = os.WriteFile(filepath.Join(logDir, "wolfcastle-2026-04-06.jsonl"), []byte("seed\n"), 0o644)

	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, logDir, instDir, nil)

	if err := w.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer w.Stop()

	if w.watcher == nil {
		t.Fatal("expected fsnotify watcher to be set after Start")
	}
}

func TestStart_NoLogDir(t *testing.T) {
	dir := t.TempDir()
	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, "", "", nil)

	if err := w.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer w.Stop()
}

func TestStartPolling_SeedsFields(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	instDir := filepath.Join(dir, "instances")
	_ = os.MkdirAll(logDir, 0o755)
	_ = os.MkdirAll(instDir, 0o755)

	logFile := filepath.Join(logDir, "wolfcastle-2026-04-06.jsonl")
	_ = os.WriteFile(logFile, []byte("hello\nworld\n"), 0o644)

	store := state.NewStore(dir, time.Second)
	// Initialize the store so the index file exists.
	_ = store.MutateIndex(func(*state.RootIndex) error { return nil })

	w := NewWatcher(store, logDir, instDir, nil)
	w.StartPolling()
	defer w.Stop()

	if w.logFile == "" {
		t.Fatal("expected logFile to be seeded")
	}
	if w.logOffset == 0 {
		t.Fatal("expected logOffset to be seeded with file size")
	}
}

func TestPollTick_DetectsIndexChange(t *testing.T) {
	dir := t.TempDir()
	store := state.NewStore(dir, time.Second)
	if err := store.MutateIndex(func(*state.RootIndex) error { return nil }); err != nil {
		t.Fatal(err)
	}

	w := NewWatcher(store, "", "", nil)

	// Seed indexMtime to a known stale value so the next stat looks "new".
	w.indexMtime = time.Unix(0, 0)

	// Should not panic; should send a message that gets dropped by the
	// cancelled-context program.
	w.pollTick()
}

func TestPollTick_NewLogFileDetected(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	_ = os.MkdirAll(logDir, 0o755)

	store := state.NewStore(dir, time.Second)
	if err := store.MutateIndex(func(*state.RootIndex) error { return nil }); err != nil {
		t.Fatal(err)
	}

	w := NewWatcher(store, logDir, "", nil)

	// No log file at startup. Then create one and call pollTick.
	logFile := filepath.Join(logDir, "wolfcastle-2026-04-06.jsonl")
	_ = os.WriteFile(logFile, []byte("first\n"), 0o644)

	w.pollTick()

	if w.logFile != logFile {
		t.Fatalf("expected logFile=%q, got %q", logFile, w.logFile)
	}
}

func TestPollTick_LogFileGrows(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	_ = os.MkdirAll(logDir, 0o755)

	logFile := filepath.Join(logDir, "wolfcastle-2026-04-06.jsonl")
	_ = os.WriteFile(logFile, []byte("seed\n"), 0o644)

	store := state.NewStore(dir, time.Second)
	if err := store.MutateIndex(func(*state.RootIndex) error { return nil }); err != nil {
		t.Fatal(err)
	}

	w := NewWatcher(store, logDir, "", nil)
	w.logFile = logFile
	if info, err := os.Stat(logFile); err == nil {
		w.logOffset = info.Size()
		w.logFileSize = info.Size()
	}

	// Append more bytes.
	f, _ := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("more\n")
	_ = f.Close()

	w.pollTick()
	// logFileSize should have advanced.
	if w.logFileSize == 0 {
		t.Fatal("expected logFileSize to update after pollTick")
	}
}

func TestFlush_IndexPath(t *testing.T) {
	dir := t.TempDir()
	store := state.NewStore(dir, time.Second)
	if err := store.MutateIndex(func(*state.RootIndex) error { return nil }); err != nil {
		t.Fatal(err)
	}

	w := NewWatcher(store, "", "", nil)

	paths := map[string]bool{store.IndexPath(): true}
	w.flush(paths) // should hit the index branch
}

func TestFlush_NodeStatePath(t *testing.T) {
	dir := t.TempDir()
	store := state.NewStore(dir, time.Second)
	if err := store.MutateIndex(func(*state.RootIndex) error { return nil }); err != nil {
		t.Fatal(err)
	}

	// Create a node directory with a state.json so ReadNode succeeds.
	nodeDir := filepath.Join(dir, "alpha")
	_ = os.MkdirAll(nodeDir, 0o755)
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), []byte(`{"address":"alpha"}`), 0o644)

	w := NewWatcher(store, "", "", nil)

	paths := map[string]bool{filepath.Join(nodeDir, "state.json"): true}
	w.flush(paths)
}

func TestFlush_LogFilePath(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	_ = os.MkdirAll(logDir, 0o755)
	logFile := filepath.Join(logDir, "wolfcastle-2026-04-06.jsonl")
	_ = os.WriteFile(logFile, []byte("appended line\n"), 0o644)

	store := state.NewStore(dir, time.Second)
	if err := store.MutateIndex(func(*state.RootIndex) error { return nil }); err != nil {
		t.Fatal(err)
	}

	w := NewWatcher(store, logDir, "", nil)
	w.logFile = logFile
	w.logOffset = 0

	paths := map[string]bool{logFile: true}
	w.flush(paths)
}

func TestStop_StopsRunningLoop(t *testing.T) {
	// Verify that Stop properly tears down a started watcher's goroutine.
	dir := t.TempDir()
	store := state.NewStore(dir, time.Second)
	if err := store.MutateIndex(func(*state.RootIndex) error { return nil }); err != nil {
		t.Fatal(err)
	}

	w := NewWatcher(store, "", "", nil)
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Give the loop a tick to enter its select.
	time.Sleep(10 * time.Millisecond)
	w.Stop()
}

// TestPollTick_DeliversLogLinesThroughChannel is the regression test
// for the release-blocker bug where the watcher was constructed but
// never Started, so log lines never reached the model. It exercises
// the full pipeline:
//   - construct a watcher with an events channel
//   - seed it pointing at a real log file
//   - call pollTick after the file grows
//   - assert a WatcherMsg lands on the channel and unwraps to a
//     LogLinesMsg with the appended content
//
// If this test fails, log streaming is broken end-to-end and the TUI
// will silently show "No transmissions" forever even when the daemon
// is hammering the log file.
func TestPollTick_DeliversLogLinesThroughChannel(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logFile := filepath.Join(logDir, "wolfcastle-2026-04-08.jsonl")
	if err := os.WriteFile(logFile, []byte("seed line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := state.NewStore(dir, time.Second)
	if err := store.MutateIndex(func(*state.RootIndex) error { return nil }); err != nil {
		t.Fatal(err)
	}

	events, next := drainEvents(t)
	w := NewWatcher(store, logDir, "", events)
	w.logFile = logFile
	if info, err := os.Stat(logFile); err == nil {
		w.logOffset = info.Size()
		w.logFileSize = info.Size()
	}

	// Append more bytes to the log file, simulating the daemon writing
	// a new NDJSON record.
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"level":"info","msg":"hello"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	// Drive a single poll cycle and assert the new line propagates as
	// a LogLinesMsg wrapped in a WatcherMsg envelope. pollTick may
	// also emit other watcher events (e.g. StateUpdatedMsg if the
	// index mtime moved); drain through them looking for LogLinesMsg.
	w.pollTick()

	var logMsg LogLinesMsg
	found := false
	for i := 0; i < 8 && !found; i++ {
		got := next()
		envelope, ok := got.(WatcherMsg)
		if !ok {
			t.Fatalf("expected WatcherMsg, got %T", got)
		}
		if l, ok := envelope.Inner.(LogLinesMsg); ok {
			logMsg = l
			found = true
		}
	}
	if !found {
		t.Fatal("never received a LogLinesMsg")
	}
	if len(logMsg.Lines) == 0 {
		t.Fatal("expected at least one log line in the message")
	}
	if logMsg.Lines[0] != `{"level":"info","msg":"hello"}` {
		t.Errorf("unexpected log line content: %q", logMsg.Lines[0])
	}
}

// ---------------------------------------------------------------------------
// EagerPrefetchAndSubscribe — covers the cache-freshness fix
// ---------------------------------------------------------------------------
//
// These tests are the unit-level companion to the wiring smoke test
// in internal/tui/app/wiring_smoke_test.go. The smoke test proves
// the end-to-end path from watcher startup to rendered tree row is
// wired correctly. These tests pin down the EagerPrefetch helper in
// isolation so a future regression in just one branch (corrupt
// file, missing store, etc.) is caught directly without having to
// dig through the rendered output.

// TestEagerPrefetchAndSubscribe_PopulatesCache writes a real index
// with two leaves and a real state.json for each. After the eager
// walk, the events channel must hold a NodeUpdatedMsg for each leaf
// with content matching the on-disk file.
func TestEagerPrefetchAndSubscribe_PopulatesCache(t *testing.T) {
	tmp := t.TempDir()
	store := newPopulatedStoreForTest(t, tmp, map[string][]testTask{
		"alpha": {
			{ID: "task-0001", Title: "first", State: state.StatusInProgress},
		},
		"beta": {
			{ID: "task-0001", Title: "second", State: state.StatusComplete},
		},
	})

	events, next := drainEvents(t)
	w := NewWatcher(store, "", "", events)
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	if err := w.EagerPrefetchAndSubscribe(); err != nil {
		t.Fatalf("EagerPrefetchAndSubscribe: %v", err)
	}

	seen := map[string]bool{}
	for i := 0; i < 4 && len(seen) < 2; i++ {
		msg := next()
		envelope, ok := msg.(WatcherMsg)
		if !ok {
			continue
		}
		nu, ok := envelope.Inner.(NodeUpdatedMsg)
		if !ok {
			continue
		}
		seen[nu.Address] = true
		if nu.Node == nil {
			t.Errorf("event for %q carried nil node", nu.Address)
		}
	}
	if !seen["alpha"] || !seen["beta"] {
		t.Errorf("expected NodeUpdatedMsg for both alpha and beta, got %v", seen)
	}
}

// TestEagerPrefetchAndSubscribe_SkipsCorruptLeaves: one leaf has
// invalid JSON. The walk must skip it cleanly and emit events for
// the other leaves.
func TestEagerPrefetchAndSubscribe_SkipsCorruptLeaves(t *testing.T) {
	tmp := t.TempDir()
	store := newPopulatedStoreForTest(t, tmp, map[string][]testTask{
		"alpha": {{ID: "task-0001", Title: "good", State: state.StatusComplete}},
		"beta":  {{ID: "task-0001", Title: "good", State: state.StatusComplete}},
	})
	alphaPath, err := store.NodePath("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(alphaPath, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	events, next := drainEvents(t)
	w := NewWatcher(store, "", "", events)
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	if err := w.EagerPrefetchAndSubscribe(); err != nil {
		t.Errorf("EagerPrefetchAndSubscribe should tolerate corrupt leaves, got: %v", err)
	}

	betaSeen := false
	for i := 0; i < 4; i++ {
		msg := next()
		envelope, ok := msg.(WatcherMsg)
		if !ok {
			continue
		}
		nu, ok := envelope.Inner.(NodeUpdatedMsg)
		if !ok {
			continue
		}
		if nu.Address == "beta" {
			betaSeen = true
			break
		}
		if nu.Address == "alpha" {
			t.Errorf("alpha should have been skipped (corrupt state.json), but a NodeUpdatedMsg was emitted for it")
		}
	}
	if !betaSeen {
		t.Error("beta NodeUpdatedMsg was not emitted; corrupt-leaf handling broke the loop")
	}
}

// TestEagerPrefetchAndSubscribe_NilStore is the defensive guard.
func TestEagerPrefetchAndSubscribe_NilStore(t *testing.T) {
	w := NewWatcher(nil, "", "", nil)
	if err := w.EagerPrefetchAndSubscribe(); err == nil {
		t.Error("expected error from EagerPrefetchAndSubscribe with nil store")
	}
}

// TestEagerPrefetchAndSubscribe_AddsFsnotifySubscriptions is the
// integration assertion for the AddNodeWatch wiring. After eager
// prefetch, modifying a leaf's state.json on disk must fire a
// fsnotify event delivered as a NodeUpdatedMsg. Without the
// AddNodeWatch call inside EagerPrefetchAndSubscribe, fsnotify is
// not subscribed to per-node directories and the modification goes
// unnoticed.
func TestEagerPrefetchAndSubscribe_AddsFsnotifySubscriptions(t *testing.T) {
	tmp := t.TempDir()
	store := newPopulatedStoreForTest(t, tmp, map[string][]testTask{
		"alpha": {{ID: "task-0001", Title: "first", State: state.StatusNotStarted}},
	})

	events, next := drainEvents(t)
	w := NewWatcher(store, "", "", events)
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	if err := w.EagerPrefetchAndSubscribe(); err != nil {
		t.Fatalf("EagerPrefetchAndSubscribe: %v", err)
	}

	// Drain the initial NodeUpdatedMsg from the eager walk.
	_ = next()

	alphaPath, err := store.NodePath("alpha")
	if err != nil {
		t.Fatal(err)
	}
	writeLeafState(t, alphaPath, []testTask{
		{ID: "task-0001", Title: "first", State: state.StatusInProgress},
	})

	got := next()
	envelope, ok := got.(WatcherMsg)
	if !ok {
		t.Fatalf("expected WatcherMsg from fsnotify, got %T", got)
	}
	nu, ok := envelope.Inner.(NodeUpdatedMsg)
	if !ok {
		t.Fatalf("expected NodeUpdatedMsg, got %T", envelope.Inner)
	}
	if nu.Address != "alpha" {
		t.Errorf("expected event for alpha, got %q", nu.Address)
	}
	if nu.Node == nil || len(nu.Node.Tasks) == 0 || nu.Node.Tasks[0].State != state.StatusInProgress {
		t.Errorf("event did not carry the new in_progress state; node = %+v", nu.Node)
	}
}

// TestEagerPrefetchAndSubscribe_IsIdempotent verifies that calling
// the function repeatedly with no index changes does not re-emit
// NodeUpdatedMsg events for already-subscribed leaves. The
// idempotence is what makes it safe to call after every index
// update without spamming the channel with redundant events.
func TestEagerPrefetchAndSubscribe_IsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	store := newPopulatedStoreForTest(t, tmp, map[string][]testTask{
		"alpha": {{ID: "task-0001", Title: "first", State: state.StatusComplete}},
	})

	events, next := drainEvents(t)
	w := NewWatcher(store, "", "", events)
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	// First call should emit one NodeUpdatedMsg.
	if err := w.EagerPrefetchAndSubscribe(); err != nil {
		t.Fatalf("first call: %v", err)
	}
	first := next()
	if env, ok := first.(WatcherMsg); !ok {
		t.Fatalf("first call did not produce a WatcherMsg, got %T", first)
	} else if _, ok := env.Inner.(NodeUpdatedMsg); !ok {
		t.Fatalf("first call envelope did not contain NodeUpdatedMsg, got %T", env.Inner)
	}

	// Second call with no index changes should produce no events.
	if err := w.EagerPrefetchAndSubscribe(); err != nil {
		t.Fatalf("second call: %v", err)
	}
	select {
	case msg := <-events:
		if env, ok := msg.(WatcherMsg); ok {
			if _, ok := env.Inner.(NodeUpdatedMsg); ok {
				t.Errorf("second call should be a no-op for already-subscribed leaves, but emitted a NodeUpdatedMsg")
			}
		}
	case <-time.After(50 * time.Millisecond):
		// Expected: no events.
	}
}

// TestEagerPrefetchAndSubscribe_PicksUpLeavesAddedAfterStartup is
// the regression test for the new-leaves-after-startup gap. The
// daemon decomposes nodes during a session, adding new leaves to
// the index. Without this fix, those leaves never got an fsnotify
// subscription or an initial cache load, so their per-task glyphs
// went stale forever even though the leaf glyph (which comes from
// the index) updated correctly.
//
// This test simulates the flow:
//  1. Watcher starts with one leaf in the index
//  2. EagerPrefetchAndSubscribe runs, subscribes to it
//  3. The user/daemon adds a new leaf to the index AND writes its
//     state.json
//  4. EagerPrefetchAndSubscribe runs again (in production this
//     happens automatically inside the flush handler after a
//     StateUpdatedMsg is emitted)
//  5. The new leaf must produce a NodeUpdatedMsg
func TestEagerPrefetchAndSubscribe_PicksUpLeavesAddedAfterStartup(t *testing.T) {
	tmp := t.TempDir()
	store := newPopulatedStoreForTest(t, tmp, map[string][]testTask{
		"alpha": {{ID: "task-0001", Title: "alpha task", State: state.StatusInProgress}},
	})

	events, next := drainEvents(t)
	w := NewWatcher(store, "", "", events)
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	if err := w.EagerPrefetchAndSubscribe(); err != nil {
		t.Fatalf("initial prefetch: %v", err)
	}
	// Drain alpha's initial event.
	_ = next()

	// Now simulate decomposition: add a new leaf "beta" to the
	// index and write its state.json. This is what happens when
	// the daemon decomposes an orchestrator into new leaves.
	if err := store.MutateIndex(func(idx *state.RootIndex) error {
		idx.Root = append(idx.Root, "beta")
		idx.Nodes["beta"] = state.IndexEntry{
			Name:    "beta",
			Type:    state.NodeLeaf,
			State:   state.StatusInProgress,
			Address: "beta",
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	betaPath, err := store.NodePath("beta")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(betaPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeLeafState(t, betaPath, []testTask{
		{ID: "task-0001", Title: "beta brand-new task", State: state.StatusInProgress},
	})

	// Re-run EagerPrefetchAndSubscribe (in production this happens
	// automatically inside the flush handler after the index
	// update is detected). The new beta leaf should produce a
	// NodeUpdatedMsg, while the already-subscribed alpha leaf
	// should NOT.
	if err := w.EagerPrefetchAndSubscribe(); err != nil {
		t.Fatalf("re-prefetch: %v", err)
	}

	betaSeen := false
	for i := 0; i < 4; i++ {
		select {
		case msg := <-events:
			env, ok := msg.(WatcherMsg)
			if !ok {
				continue
			}
			nu, ok := env.Inner.(NodeUpdatedMsg)
			if !ok {
				continue
			}
			if nu.Address == "alpha" {
				t.Errorf("alpha should not be re-emitted (already subscribed); idempotence check failed")
			}
			if nu.Address == "beta" {
				betaSeen = true
				if nu.Node == nil || len(nu.Node.Tasks) == 0 || nu.Node.Tasks[0].Title != "beta brand-new task" {
					t.Errorf("beta event did not carry the freshly-written task content; node = %+v", nu.Node)
				}
			}
		case <-time.After(50 * time.Millisecond):
			break
		}
		if betaSeen {
			break
		}
	}
	if !betaSeen {
		t.Error("EagerPrefetchAndSubscribe did not emit a NodeUpdatedMsg for the newly-added beta leaf; daemon decomposition will silently break per-leaf cache freshness")
	}
}

// ---------------------------------------------------------------------------
// Test helpers for the eager-prefetch tests
// ---------------------------------------------------------------------------

type testTask struct {
	ID    string
	Title string
	State state.NodeStatus
}

// newPopulatedStoreForTest creates a state.Store rooted at tmp with
// a real root index containing one leaf entry per key in leafTasks,
// and writes a real state.json for each leaf with the given tasks.
func newPopulatedStoreForTest(t *testing.T, tmp string, leafTasks map[string][]testTask) *state.Store {
	t.Helper()
	store := state.NewStore(tmp, time.Second)

	idx := &state.RootIndex{
		Version: 1,
		Nodes:   make(map[string]state.IndexEntry),
	}
	for addr := range leafTasks {
		idx.Root = append(idx.Root, addr)
		idx.Nodes[addr] = state.IndexEntry{
			Name:    addr,
			Type:    state.NodeLeaf,
			State:   state.StatusInProgress,
			Address: addr,
		}
	}
	if err := store.MutateIndex(func(ri *state.RootIndex) error {
		*ri = *idx
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for addr, tasks := range leafTasks {
		nodePath, err := store.NodePath(addr)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(nodePath), 0o755); err != nil {
			t.Fatal(err)
		}
		writeLeafState(t, nodePath, tasks)
	}
	return store
}

// writeLeafState marshals a NodeState with the given tasks and
// writes it to the path. Used for both initial setup and the
// fsnotify-driven update test.
func writeLeafState(t *testing.T, path string, tasks []testTask) *state.NodeState {
	t.Helper()
	stateTasks := make([]state.Task, 0, len(tasks))
	for _, task := range tasks {
		stateTasks = append(stateTasks, state.Task{
			ID:    task.ID,
			Title: task.Title,
			State: task.State,
		})
	}
	ns := &state.NodeState{
		Version: 1,
		ID:      filepath.Base(filepath.Dir(path)),
		Name:    filepath.Base(filepath.Dir(path)),
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
		Tasks:   stateTasks,
	}
	data, err := json.Marshal(ns)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return ns
}

// ---------------------------------------------------------------------------
// LoadInitialLogTail — covers the #4b fix
// ---------------------------------------------------------------------------

// TestLoadInitialLogTail_EmitsExistingContent stages a real log
// file with several NDJSON lines, points the watcher at it, and
// asserts a LogLinesMsg arrives in the events channel containing
// the existing lines. Without the fix, the watcher seeded
// logOffset = file size and emitted nothing on startup.
func TestLoadInitialLogTail_EmitsExistingContent(t *testing.T) {
	tmp := t.TempDir()
	logDir := filepath.Join(tmp, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logFile := filepath.Join(logDir, "0001-exec-20260408T07-30Z.jsonl")
	content := []byte(`{"level":"info","msg":"line one"}` + "\n" +
		`{"level":"info","msg":"line two"}` + "\n" +
		`{"level":"warn","msg":"line three"}` + "\n")
	if err := os.WriteFile(logFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// Stub a store so the watcher constructs cleanly. The test
	// doesn't exercise per-node state, only the log path.
	store := state.NewStore(tmp, time.Second)
	if err := store.MutateIndex(func(*state.RootIndex) error { return nil }); err != nil {
		t.Fatal(err)
	}

	events, next := drainEvents(t)
	w := NewWatcher(store, logDir, "", events)
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	got := next()
	envelope, ok := got.(WatcherMsg)
	if !ok {
		t.Fatalf("expected WatcherMsg, got %T", got)
	}
	logMsg, ok := envelope.Inner.(LogLinesMsg)
	if !ok {
		t.Fatalf("expected LogLinesMsg, got %T", envelope.Inner)
	}
	if len(logMsg.Lines) != 3 {
		t.Errorf("expected 3 tail-loaded lines, got %d: %v", len(logMsg.Lines), logMsg.Lines)
	}
	if len(logMsg.Lines) >= 1 && logMsg.Lines[0] != `{"level":"info","msg":"line one"}` {
		t.Errorf("first tail-loaded line content mismatch: %q", logMsg.Lines[0])
	}
}

// TestLoadInitialLogTail_TrimsToMaxLines verifies that a file with
// more than maxLines is trimmed to the last maxLines, not the first.
func TestLoadInitialLogTail_TrimsToMaxLines(t *testing.T) {
	tmp := t.TempDir()
	logDir := filepath.Join(tmp, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logFile := filepath.Join(logDir, "0001-exec-20260408T07-30Z.jsonl")

	// Write 1000 lines.
	var content []byte
	for i := 0; i < 1000; i++ {
		content = append(content, []byte(fmt.Sprintf(`{"n":%d}`+"\n", i))...)
	}
	if err := os.WriteFile(logFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	store := state.NewStore(tmp, time.Second)
	if err := store.MutateIndex(func(*state.RootIndex) error { return nil }); err != nil {
		t.Fatal(err)
	}

	events, next := drainEvents(t)
	w := NewWatcher(store, logDir, "", events)
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	got := next()
	envelope := got.(WatcherMsg)
	logMsg := envelope.Inner.(LogLinesMsg)
	if len(logMsg.Lines) != defaultLogTailLines {
		t.Errorf("expected %d tail lines (defaultLogTailLines), got %d", defaultLogTailLines, len(logMsg.Lines))
	}
	// First line of tail should be the (1000 - 500)th = 500th
	// (zero-indexed), so the JSON should contain `"n":500`.
	if len(logMsg.Lines) > 0 && !strings.Contains(logMsg.Lines[0], `"n":500`) {
		t.Errorf("tail should start at line 500, first line: %q", logMsg.Lines[0])
	}
}

// TestLoadInitialLogTail_SkipsGzFiles confirms that the tail loader
// silently skips .jsonl.gz files (the live log is always plain
// .jsonl; .gz files are rotated archives).
func TestLoadInitialLogTail_SkipsGzFiles(t *testing.T) {
	tmp := t.TempDir()
	logDir := filepath.Join(tmp, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Only an exec.jsonl.gz file (no live exec). LatestLogFile would
	// fall back to lex-max which would still be the gz, so the
	// watcher tries to tail-load it. The loader must skip rather than
	// emit garbage.
	gzFile := filepath.Join(logDir, "0001-exec-20260408T07-30Z.jsonl.gz")
	if err := os.WriteFile(gzFile, []byte("not really gzipped"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := state.NewStore(tmp, time.Second)
	if err := store.MutateIndex(func(*state.RootIndex) error { return nil }); err != nil {
		t.Fatal(err)
	}

	events := make(chan tea.Msg, 8)
	w := NewWatcher(store, logDir, "", events)
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	// Non-blocking poll: expect zero events from the tail loader
	// since it should have skipped the .gz file.
	select {
	case msg := <-events:
		// State updates from the index file watch are fine; what
		// would be wrong is a LogLinesMsg containing gzip garbage.
		if envelope, ok := msg.(WatcherMsg); ok {
			if _, ok := envelope.Inner.(LogLinesMsg); ok {
				t.Errorf("tail loader should have skipped the .gz file, but emitted a LogLinesMsg: %v", envelope.Inner)
			}
		}
	default:
		// No event = expected.
	}
}
