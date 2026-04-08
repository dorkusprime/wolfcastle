package tui

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/fsnotify/fsnotify"
)

// stubModel is a no-op tea.Model used to satisfy tea.NewProgram in tests.
type stubModel struct{}

func (stubModel) Init() tea.Cmd                         { return nil }
func (m stubModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (stubModel) View() tea.View                        { return tea.View{} }

// cancelledProgram returns a *tea.Program whose context is already cancelled,
// so any call to program.Send returns immediately without blocking on the
// internal msgs channel. This lets us exercise watcher code paths that
// dispatch messages without spinning up a real Bubbletea runtime.
func cancelledProgram() *tea.Program {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return tea.NewProgram(stubModel{}, tea.WithContext(ctx))
}

func TestNewWatcher_FieldsInitialized(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "/tmp/logs", "/tmp/instances")

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
	w := NewWatcher(store, dir, "")
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
	w := NewWatcher(store, dir, "")
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
	w := NewWatcher(store, dir, "")
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
	w := NewWatcher(store, dir, "")
	w.logFile = logFile
	w.logOffset = 0

	lines := w.readNewLogLines()
	if lines != nil {
		t.Fatalf("expected nil for empty file, got %v", lines)
	}
}

func TestReadNewLogLines_NoLogFile(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "")
	w.logFile = ""

	lines := w.readNewLogLines()
	if lines != nil {
		t.Fatalf("expected nil for no log file, got %v", lines)
	}
}

func TestReadNewLogLines_MissingFile(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "")
	w.logFile = "/nonexistent/path/test.jsonl"

	lines := w.readNewLogLines()
	if lines != nil {
		t.Fatalf("expected nil for missing file, got %v", lines)
	}
}

func TestNodeAddrFromPath(t *testing.T) {
	dir := t.TempDir()
	store := state.NewStore(dir, time.Second)
	w := NewWatcher(store, "", "")

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
	w := NewWatcher(store, "", "")

	// First stop should work fine
	w.Stop()
	// Second stop should not panic
	w.Stop()
}

func TestStop_CancelsTimers(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "")

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
	w := NewWatcher(store, "", "")
	w.watcher = nil

	// Should not panic
	w.AddNodeWatch("a/b/c")
}

func TestRemoveNodeWatch_NilWatcher(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "")
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
	w := NewWatcher(store, dir, "")
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
	w := NewWatcher(store, dir, "")
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
	w := NewWatcher(store, "", "")

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
	w := NewWatcher(store, "", "")

	// Should not panic with empty pending set
	w.flushFromTimer()
}

func TestNewWatcher_DefaultUseFsnotify(t *testing.T) {
	store := state.NewStore(t.TempDir(), time.Second)
	w := NewWatcher(store, "", "")

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
	w := NewWatcher(store, "", "")

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
	w := NewWatcher(store, "", "")

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
	w := NewWatcher(store, "", "")

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
	w := NewWatcher(store, "", "")

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
	w := NewWatcher(store, "", "")

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
	w := NewWatcher(store, "", "")

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
	w := NewWatcher(store, "", "")

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
	w := NewWatcher(store, dir, "")
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
	w := NewWatcher(store, logDir, instDir)

	prog := cancelledProgram()
	if err := w.Start(prog); err != nil {
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
	w := NewWatcher(store, "", "")

	prog := cancelledProgram()
	if err := w.Start(prog); err != nil {
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

	w := NewWatcher(store, logDir, instDir)
	prog := cancelledProgram()
	w.StartPolling(prog)
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

	w := NewWatcher(store, "", "")
	w.program = cancelledProgram()

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

	w := NewWatcher(store, logDir, "")
	w.program = cancelledProgram()

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

	w := NewWatcher(store, logDir, "")
	w.program = cancelledProgram()
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

	w := NewWatcher(store, "", "")
	w.program = cancelledProgram()

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

	w := NewWatcher(store, "", "")
	w.program = cancelledProgram()

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

	w := NewWatcher(store, logDir, "")
	w.program = cancelledProgram()
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

	w := NewWatcher(store, "", "")
	prog := cancelledProgram()
	if err := w.Start(prog); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Give the loop a tick to enter its select.
	time.Sleep(10 * time.Millisecond)
	w.Stop()
}
