package welcome

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// helper: create a key press for a printable character like 'j', 'q', etc.
func keyPress(char rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: char, Text: string(char)}
}

// helper: create a key press for a special key (enter, backspace, etc.)
func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func enterKey() tea.KeyPressMsg  { return specialKey(tea.KeyEnter) }
func downKey() tea.KeyPressMsg   { return keyPress('j') }
func upKey() tea.KeyPressMsg     { return keyPress('k') }
func topKey() tea.KeyPressMsg    { return keyPress('g') }
func bottomKey() tea.KeyPressMsg { return keyPress('G') }
func backKey() tea.KeyPressMsg   { return keyPress('h') }
func quitKey() tea.KeyPressMsg   { return keyPress('q') }
func lKey() tea.KeyPressMsg      { return keyPress('l') }
func initKey() tea.KeyPressMsg   { return keyPress('I') }

// setupTestDir creates a directory with the given subdirectories inside it.
// NewModel(dir, nil) will open in dir showing its subdirectories.
func setupTestDir(t *testing.T, subdirs ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, s := range subdirs {
		if err := os.Mkdir(filepath.Join(dir, s), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// ---------- Construction ----------

func TestNewModel_StartsInCWD(t *testing.T) {
	dir := setupTestDir(t, "aaa", "bbb")

	m := NewModel(dir, nil)

	if m.currentDir != dir {
		t.Fatalf("expected currentDir=%q, got %q", dir, m.currentDir)
	}
	if len(m.entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m.entries))
	}
}

func TestNewModel_CursorStartsAtZero(t *testing.T) {
	dir := setupTestDir(t, "aaa", "zzz")

	m := NewModel(dir, nil)

	if m.dirCursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.dirCursor)
	}
	if m.entries[0].Name() != "aaa" {
		t.Fatalf("expected first entry 'aaa', got %q", m.entries[0].Name())
	}
}

// ---------- Cursor movement ----------

func TestCursorDown(t *testing.T) {
	dir := setupTestDir(t, "aaa", "bbb", "ccc", "ddd", "eee")
	m := NewModel(dir, nil)
	start := m.dirCursor

	m, _ = m.Update(downKey())
	if m.dirCursor != start+1 {
		t.Fatalf("expected cursor=%d, got %d", start+1, m.dirCursor)
	}
}

func TestCursorDown_Clamps(t *testing.T) {
	dir := setupTestDir(t, "aaa", "bbb")
	m := NewModel(dir, nil)

	// Move well past the end
	for range len(m.entries) + 2 {
		m, _ = m.Update(downKey())
	}
	if m.dirCursor != len(m.entries)-1 {
		t.Fatalf("expected cursor clamped at %d, got %d", len(m.entries)-1, m.dirCursor)
	}
}

func TestCursorUp(t *testing.T) {
	dir := setupTestDir(t, "aaa", "bbb", "ccc", "ddd", "eee")
	m := NewModel(dir, nil)

	// Move down twice, then up
	m, _ = m.Update(downKey())
	m, _ = m.Update(downKey())
	pos := m.dirCursor
	m, _ = m.Update(upKey())
	if m.dirCursor != pos-1 {
		t.Fatalf("expected cursor=%d, got %d", pos-1, m.dirCursor)
	}
}

func TestCursorUp_ClampsAtZero(t *testing.T) {
	dir := setupTestDir(t, "aaa", "zzz")
	m := NewModel(dir, nil)

	// Move to top first
	m, _ = m.Update(topKey())
	m, _ = m.Update(upKey())
	if m.dirCursor != 0 {
		t.Fatalf("expected cursor clamped at 0, got %d", m.dirCursor)
	}
}

func TestJumpToTop(t *testing.T) {
	dir := setupTestDir(t, "aaa", "bbb", "ccc", "ddd", "eee")
	m := NewModel(dir, nil)

	m, _ = m.Update(bottomKey())
	m, _ = m.Update(topKey())
	if m.dirCursor != 0 {
		t.Fatalf("expected cursor=0, got %d", m.dirCursor)
	}
}

func TestJumpToBottom(t *testing.T) {
	dir := setupTestDir(t, "aaa", "bbb", "ccc", "ddd", "eee")
	m := NewModel(dir, nil)

	m, _ = m.Update(bottomKey())
	if m.dirCursor != len(m.entries)-1 {
		t.Fatalf("expected cursor=%d, got %d", len(m.entries)-1, m.dirCursor)
	}
}

// ---------- Navigation ----------

func TestEnterOnDirectory_Descends(t *testing.T) {
	dir := setupTestDir(t, "target")
	// Create a grandchild so "target" has contents
	_ = os.Mkdir(filepath.Join(dir, "target", "inner"), 0o755)

	m := NewModel(dir, nil)
	// Cursor is on "target". Enter should descend into it.
	targetDir := filepath.Join(dir, "target")
	m, _ = m.Update(enterKey())
	if m.currentDir != targetDir {
		t.Fatalf("expected currentDir=%q, got %q", targetDir, m.currentDir)
	}
}

func TestLKey_DescendsButDoesNotInit(t *testing.T) {
	dir := setupTestDir(t, "empty")

	m := NewModel(dir, nil)
	// Cursor is on "empty". Enter descends into it.
	m, _ = m.Update(enterKey())
	// Now inside "empty" which has no entries. l should NOT init.
	m, cmd := m.Update(lKey())
	if m.initializing {
		t.Fatal("l should not trigger init, only Enter should")
	}
	if cmd != nil {
		t.Fatal("expected nil cmd from l in empty dir")
	}
}

func TestBack_GoesToParent(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)
	parentDir := m.currentDir

	// Descend first, then back
	m, _ = m.Update(enterKey())
	m, _ = m.Update(backKey())

	if m.currentDir != parentDir {
		t.Fatalf("expected currentDir=%q, got %q", parentDir, m.currentDir)
	}
}

// ---------- Init flow ----------

func TestEnterOnEmptyDir_StartsInit(t *testing.T) {
	dir := setupTestDir(t, "empty")

	m := NewModel(dir, nil)
	// Descend into "empty"
	m, _ = m.Update(enterKey())

	// Now in empty dir, I key should init
	m, cmd := m.Update(initKey())
	if !m.initializing {
		t.Fatal("expected initializing=true")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command from init start")
	}
}

func TestInitKey_StartsInitInCurrentDir(t *testing.T) {
	dir := setupTestDir(t, "subdir")

	m := NewModel(dir, nil)

	m, cmd := m.Update(initKey())
	if !m.initializing {
		t.Fatal("expected initializing=true after pressing I")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
}

func TestEnterOnDotWolfcastle_DoesNotInit(t *testing.T) {
	dir := setupTestDir(t, ".wolfcastle")

	m := NewModel(dir, nil)
	for i, e := range m.entries {
		if e.Name() == ".wolfcastle" {
			m.dirCursor = i
			break
		}
	}

	m, _ = m.Update(enterKey())
	if m.initializing {
		t.Fatal("Enter on .wolfcastle should not trigger init, use I")
	}
}

func TestInitCompleteMsg_Success(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)
	m.initializing = true

	m, _ = m.Update(tui.InitCompleteMsg{Dir: dir, Err: nil})
	if m.initializing {
		t.Fatal("expected initializing=false after success")
	}
	if m.err != nil {
		t.Fatalf("expected err=nil, got %v", m.err)
	}
}

func TestInitCompleteMsg_Failure(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)
	m.initializing = true

	testErr := errors.New("boom")
	m, _ = m.Update(tui.InitCompleteMsg{Dir: dir, Err: testErr})
	if m.initializing {
		t.Fatal("expected initializing=false")
	}
	if m.err == nil || m.err.Error() != "boom" {
		t.Fatalf("expected err=boom, got %v", m.err)
	}
}

// ---------- Spinner ----------

func TestSpinnerTickMsg_WhileInitializing(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)
	m.initializing = true
	m.spinnerFrame = 0

	m, cmd := m.Update(tui.SpinnerTickMsg{})
	if m.spinnerFrame != 1 {
		t.Fatalf("expected spinnerFrame=1, got %d", m.spinnerFrame)
	}
	if cmd == nil {
		t.Fatal("expected tick command while initializing")
	}
}

func TestSpinnerTickMsg_WhileNotInitializing(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)
	m.initializing = false

	m, cmd := m.Update(tui.SpinnerTickMsg{})
	if m.spinnerFrame != 0 {
		t.Fatalf("expected spinnerFrame=0, got %d", m.spinnerFrame)
	}
	if cmd != nil {
		t.Fatal("expected nil command when not initializing")
	}
}

// ---------- Window size ----------

func TestWindowSizeMsg(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)

	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.width != 120 || m.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", m.width, m.height)
	}
}

func TestSetSize(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)

	m.SetSize(100, 50)
	if m.width != 100 || m.height != 50 {
		t.Fatalf("expected 100x50, got %dx%d", m.width, m.height)
	}
}

// ---------- View rendering ----------

func TestView_ShowsTitle(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "WOLFCASTLE") {
		t.Fatal("expected WOLFCASTLE in view output")
	}
}

func TestView_ShowsTreeConnectors(t *testing.T) {
	dir := setupTestDir(t, "aaa", "bbb", "ccc")
	m := NewModel(dir, nil)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "├──") && !strings.Contains(view, "└──") {
		t.Fatal("expected tree connectors in view")
	}
}

func TestView_WithError(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)
	m.SetSize(80, 24)
	m.err = errors.New("test error")

	view := m.View()
	if !strings.Contains(view, "Init failed") {
		t.Fatal("expected 'Init failed' in view output")
	}
}

func TestView_WhileInitializing(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)
	m.SetSize(80, 24)
	m.initializing = true

	view := m.View()
	if !strings.Contains(view, "Initializing") {
		t.Fatal("expected 'Initializing' in view output")
	}
}

func TestView_EmptyDir_ShowsHint(t *testing.T) {
	dir := setupTestDir(t, "empty")

	m := NewModel(dir, nil)
	// Descend into empty
	m, _ = m.Update(enterKey())
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "no subdirectories") {
		t.Fatal("expected 'no subdirectories' hint")
	}
}

func TestView_WithEntries_ShowsDirNames(t *testing.T) {
	dir := setupTestDir(t, "alpha", "beta", "target")
	m := NewModel(dir, nil)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "alpha") {
		t.Fatal("expected 'alpha' in view")
	}
	if !strings.Contains(view, "beta") {
		t.Fatal("expected 'beta' in view")
	}
}

func TestView_SelectedEntryHasMarker(t *testing.T) {
	dir := setupTestDir(t, "target", "other")
	m := NewModel(dir, nil)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "▸") {
		t.Fatal("expected ▸ marker on selected entry")
	}
}

func TestView_KeyHints(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "quit") {
		t.Fatal("expected quit hint in view")
	}
}

// ---------- Filtering ----------

func TestHiddenDirsFiltered_ExceptWolfcastle(t *testing.T) {
	dir := t.TempDir()
	_ = os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	_ = os.Mkdir(filepath.Join(dir, ".hidden"), 0o755)
	_ = os.Mkdir(filepath.Join(dir, ".wolfcastle"), 0o755)
	_ = os.Mkdir(filepath.Join(dir, "visible"), 0o755)

	m := NewModel(dir, nil)

	names := entryNames(m.entries)
	if len(names) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(names), names)
	}
	if names[0] != ".wolfcastle" || names[1] != "visible" {
		t.Fatalf("expected [.wolfcastle visible], got %v", names)
	}
}

func TestOnlyDirsShown_NoFiles(t *testing.T) {
	dir := t.TempDir()
	_ = os.Mkdir(filepath.Join(dir, "subdir"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644)

	m := NewModel(dir, nil)

	if len(m.entries) != 1 {
		t.Fatalf("expected 1 dir entry, got %d: %v", len(m.entries), entryNames(m.entries))
	}
	if m.entries[0].Name() != "subdir" {
		t.Fatalf("expected 'subdir', got %q", m.entries[0].Name())
	}
}

// ---------- Quit ----------

func TestQuitKey(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)

	_, cmd := m.Update(quitKey())
	if cmd == nil {
		t.Fatal("expected non-nil quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestQuitKey_DuringInit(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)
	m.initializing = true

	_, cmd := m.Update(quitKey())
	if cmd == nil {
		t.Fatal("expected quit command even while initializing")
	}
}

func TestKeysSwallowedDuringInit(t *testing.T) {
	dir := setupTestDir(t, "aaa", "bbb", "ccc")
	m := NewModel(dir, nil)
	m.initializing = true
	startCursor := m.dirCursor

	m2, _ := m.Update(downKey())
	if m2.dirCursor != startCursor {
		t.Fatal("cursor should not move during init")
	}
}

// ---------- Scrolling ----------

func TestScrolling(t *testing.T) {
	dir := t.TempDir()
	for i := range 25 {
		name := "d" + strings.Repeat("0", 2-len(intStr(i))) + intStr(i)
		_ = os.Mkdir(filepath.Join(dir, name), 0o755)
	}

	m := NewModel(dir, nil)
	if len(m.entries) != 25 {
		t.Fatalf("expected 25 entries, got %d", len(m.entries))
	}

	m, _ = m.Update(bottomKey())
	if m.dirCursor != 24 {
		t.Fatalf("expected cursor=24, got %d", m.dirCursor)
	}
	if m.scrollTop < 1 {
		t.Fatalf("expected scrollTop > 0, got %d", m.scrollTop)
	}

	m, _ = m.Update(topKey())
	if m.scrollTop != 0 {
		t.Fatalf("expected scrollTop=0, got %d", m.scrollTop)
	}
}

func TestView_ScrollIndicators(t *testing.T) {
	dir := t.TempDir()
	for i := range 25 {
		name := "d" + strings.Repeat("0", 2-len(intStr(i))) + intStr(i)
		_ = os.Mkdir(filepath.Join(dir, name), 0o755)
	}

	m := NewModel(dir, nil)
	m.SetSize(80, 40)

	// Jump to bottom so we have "more above"
	m, _ = m.Update(bottomKey())
	view := m.View()
	if !strings.Contains(view, "more above") {
		t.Fatal("expected 'more above' indicator when scrolled down")
	}
}

// ---------- Unknown message ----------

func TestUnknownMsg_NoOp(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)
	origCursor := m.dirCursor
	origDir := m.currentDir

	type unknownMsg struct{}
	m, cmd := m.Update(unknownMsg{})
	if m.dirCursor != origCursor || m.currentDir != origDir || cmd != nil {
		t.Fatal("unknown message should be a no-op")
	}
}

// ---------- SetInstances ----------

func TestSetInstances(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)

	entries := []instance.Entry{
		{PID: 1, Worktree: "/a", Branch: "main"},
		{PID: 2, Worktree: "/b", Branch: "dev"},
	}
	m.SetInstances(entries)

	if len(m.instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(m.instances))
	}
}

func TestSetInstances_ClampsSessionCursor(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{
		{PID: 1, Worktree: "/a", Branch: "main"},
		{PID: 2, Worktree: "/b", Branch: "dev"},
		{PID: 3, Worktree: "/c", Branch: "fix"},
	}
	m := NewModel(dir, entries)
	m.sessionCursor = 2

	// Reduce to 1 instance; cursor should clamp.
	m.SetInstances([]instance.Entry{{PID: 1, Worktree: "/a", Branch: "main"}})
	if m.sessionCursor != 0 {
		t.Fatalf("expected sessionCursor=0, got %d", m.sessionCursor)
	}
}

func TestSetInstances_EmptyList(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{{PID: 1, Worktree: "/a", Branch: "main"}}
	m := NewModel(dir, entries)
	m.sessionCursor = 0

	m.SetInstances(nil)
	if m.sessionCursor != 0 {
		t.Fatalf("expected sessionCursor=0 on empty list, got %d", m.sessionCursor)
	}
}

// ---------- InstancesUpdatedMsg ----------

func TestInstancesUpdatedMsg(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)

	entries := []instance.Entry{{PID: 42, Worktree: "/x", Branch: "main"}}
	m, _ = m.Update(tui.InstancesUpdatedMsg{Instances: entries})
	if len(m.instances) != 1 {
		t.Fatalf("expected 1 instance after update, got %d", len(m.instances))
	}
}

// ---------- Session key handling ----------

func tabKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyTab}
}

func TestTabSwitchesFocus(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{{PID: 1, Worktree: "/a", Branch: "main"}}
	m := NewModel(dir, entries)

	if m.focus != panelSessions {
		t.Fatal("expected sessions panel focused initially")
	}

	m, _ = m.Update(tabKey())
	if m.focus != panelDirs {
		t.Fatal("expected dirs panel focused after tab")
	}

	m, _ = m.Update(tabKey())
	if m.focus != panelSessions {
		t.Fatal("expected sessions panel focused after second tab")
	}
}

func TestTabIgnoredWithNoInstances(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)

	m, _ = m.Update(tabKey())
	if m.focus != panelDirs {
		t.Fatal("expected focus unchanged with no instances")
	}
}

func TestSessionKey_MoveDown(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{
		{PID: 1, Worktree: "/a", Branch: "a"},
		{PID: 2, Worktree: "/b", Branch: "b"},
		{PID: 3, Worktree: "/c", Branch: "c"},
	}
	m := NewModel(dir, entries)

	m, _ = m.Update(downKey())
	if m.sessionCursor != 1 {
		t.Fatalf("expected sessionCursor=1, got %d", m.sessionCursor)
	}
}

func TestSessionKey_MoveDownClamps(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{
		{PID: 1, Worktree: "/a", Branch: "a"},
		{PID: 2, Worktree: "/b", Branch: "b"},
	}
	m := NewModel(dir, entries)

	for range 5 {
		m, _ = m.Update(downKey())
	}
	if m.sessionCursor != 1 {
		t.Fatalf("expected sessionCursor clamped at 1, got %d", m.sessionCursor)
	}
}

func TestSessionKey_MoveUp(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{
		{PID: 1, Worktree: "/a", Branch: "a"},
		{PID: 2, Worktree: "/b", Branch: "b"},
	}
	m := NewModel(dir, entries)

	m, _ = m.Update(downKey())
	m, _ = m.Update(upKey())
	if m.sessionCursor != 0 {
		t.Fatalf("expected sessionCursor=0, got %d", m.sessionCursor)
	}
}

func TestSessionKey_MoveUpClampsAtZero(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{{PID: 1, Worktree: "/a", Branch: "a"}}
	m := NewModel(dir, entries)

	m, _ = m.Update(upKey())
	if m.sessionCursor != 0 {
		t.Fatalf("expected sessionCursor=0, got %d", m.sessionCursor)
	}
}

func TestSessionKey_Top(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{
		{PID: 1, Worktree: "/a", Branch: "a"},
		{PID: 2, Worktree: "/b", Branch: "b"},
		{PID: 3, Worktree: "/c", Branch: "c"},
	}
	m := NewModel(dir, entries)
	m, _ = m.Update(bottomKey())
	m, _ = m.Update(topKey())

	if m.sessionCursor != 0 {
		t.Fatalf("expected sessionCursor=0 after top, got %d", m.sessionCursor)
	}
}

func TestSessionKey_Bottom(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{
		{PID: 1, Worktree: "/a", Branch: "a"},
		{PID: 2, Worktree: "/b", Branch: "b"},
		{PID: 3, Worktree: "/c", Branch: "c"},
	}
	m := NewModel(dir, entries)

	m, _ = m.Update(bottomKey())
	if m.sessionCursor != 2 {
		t.Fatalf("expected sessionCursor=2, got %d", m.sessionCursor)
	}
}

func TestSessionKey_BottomEmptyList(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, []instance.Entry{})
	m.focus = panelSessions
	m.instances = []instance.Entry{} // empty

	m, _ = m.handleSessionKey(bottomKey())
	if m.sessionCursor != 0 {
		t.Fatalf("expected sessionCursor=0 with empty list, got %d", m.sessionCursor)
	}
}

func TestSessionKey_EnterConnects(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{
		{PID: 42, Worktree: "/some/path", Branch: "main"},
	}
	m := NewModel(dir, entries)

	_, cmd := m.Update(enterKey())
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Enter on session")
	}
	msg := cmd()
	conn, ok := msg.(ConnectInstanceMsg)
	if !ok {
		t.Fatalf("expected ConnectInstanceMsg, got %T", msg)
	}
	if conn.Entry.PID != 42 {
		t.Fatalf("expected PID=42, got %d", conn.Entry.PID)
	}
}

func TestSessionKey_Quit(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{{PID: 1, Worktree: "/a", Branch: "a"}}
	m := NewModel(dir, entries)

	_, cmd := m.Update(quitKey())
	if cmd == nil {
		t.Fatal("expected quit cmd from session panel")
	}
}

// ---------- renderSessions ----------

func TestRenderSessions_ShowsInstances(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{
		{PID: 100, Worktree: "/path/to/main", Branch: "main"},
		{PID: 200, Worktree: "/path/to/dev", Branch: "dev"},
	}
	m := NewModel(dir, entries)
	m.SetSize(80, 40)

	view := m.View()
	if !strings.Contains(view, "RUNNING SESSIONS") {
		t.Fatal("expected RUNNING SESSIONS heading")
	}
	if !strings.Contains(view, "2 active") {
		t.Fatal("expected '2 active' count")
	}
}

func TestRenderSessions_DimmedWhenUnfocused(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{
		{PID: 1, Worktree: "/a", Branch: "main"},
	}
	m := NewModel(dir, entries)
	m.SetSize(80, 40)

	// Switch focus to dirs panel
	m.focus = panelDirs
	view := m.View()
	// Still renders sessions panel even when unfocused.
	if !strings.Contains(view, "RUNNING SESSIONS") {
		t.Fatal("expected sessions panel when unfocused")
	}
}

// ---------- renderHints ----------

func TestRenderHints_WithInstances(t *testing.T) {
	dir := setupTestDir(t, "target")
	entries := []instance.Entry{{PID: 1, Worktree: "/a", Branch: "a"}}
	m := NewModel(dir, entries)
	m.SetSize(80, 40)

	view := m.View()
	if !strings.Contains(view, "switch panel") {
		t.Fatal("expected 'switch panel' hint when instances exist")
	}
}

func TestRenderHints_WithoutInstances(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)
	m.SetSize(80, 40)

	view := m.View()
	if strings.Contains(view, "switch panel") {
		t.Fatal("should not show 'switch panel' hint without instances")
	}
}

// ---------- place helper ----------

func TestPlace_ZeroDimensions(t *testing.T) {
	dir := setupTestDir(t, "target")
	m := NewModel(dir, nil)
	// Width/height both 0; place should use defaults.
	m.SetSize(0, 0)

	view := m.View()
	if !strings.Contains(view, "WOLFCASTLE") {
		t.Fatal("expected WOLFCASTLE in view even with zero dimensions")
	}
}

// ---------- Symlink directory handling ----------

func TestSymlinkToDir_ShowsInList(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	_ = os.Mkdir(realDir, 0o755)
	symlink := filepath.Join(dir, "linked")
	err := os.Symlink(realDir, symlink)
	if err != nil {
		t.Skipf("symlink creation failed: %v", err)
	}

	m := NewModel(dir, nil)
	names := entryNames(m.entries)
	found := false
	for _, n := range names {
		if n == "linked" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'linked' symlink in entries, got %v", names)
	}
}

// ---------- Helpers ----------

// TestRunInit_ScaffoldsRealProject is the regression test for the
// release-blocker bug where runInit only created an empty .wolfcastle
// directory. The daemon then refused to start because identity wasn't
// configured. After the fix, runInit must produce a real scaffold:
// the system tier, base config, custom config, and local identity all
// have to be on disk.
func TestRunInit_ScaffoldsRealProject(t *testing.T) {
	tmp := t.TempDir()
	m := Model{currentDir: tmp}
	cmd := m.runInit(tmp)
	if cmd == nil {
		t.Fatal("runInit returned nil cmd")
	}
	msg := cmd()
	complete, ok := msg.(tui.InitCompleteMsg)
	if !ok {
		t.Fatalf("expected InitCompleteMsg, got %T", msg)
	}
	if complete.Err != nil {
		t.Fatalf("runInit reported error: %v", complete.Err)
	}
	wcDir := filepath.Join(tmp, ".wolfcastle")
	mustExist := []string{
		"system/base/config.json",
		"system/custom/config.json",
		"system/local/config.json",
		"system/base/prompts",
	}
	for _, rel := range mustExist {
		full := filepath.Join(wcDir, rel)
		if _, err := os.Stat(full); err != nil {
			t.Errorf("expected scaffold artifact %s to exist after init: %v", rel, err)
		}
	}
}

// TestRunInit_AlreadyInitialized treats a pre-existing .wolfcastle as
// a no-op success rather than blowing up. This matches the CLI's
// "Already initialized" behavior so the user can press I in a worktree
// that's already a project without seeing an error.
func TestRunInit_AlreadyInitialized(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".wolfcastle"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	m := Model{currentDir: tmp}
	msg := m.runInit(tmp)()
	complete, ok := msg.(tui.InitCompleteMsg)
	if !ok {
		t.Fatalf("expected InitCompleteMsg, got %T", msg)
	}
	if complete.Err != nil {
		t.Errorf("re-init on existing dir should not error, got: %v", complete.Err)
	}
}

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

func intStr(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}
