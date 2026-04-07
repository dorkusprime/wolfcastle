package welcome

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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

// setupTestDir creates a parent with a named child directory and returns both paths.
// NewWelcomeModel(child) will open in parent with child pre-selected.
func setupTestDir(t *testing.T, childName string, siblings ...string) (parent, child string) {
	t.Helper()
	parent = t.TempDir()
	child = filepath.Join(parent, childName)
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, s := range siblings {
		if err := os.Mkdir(filepath.Join(parent, s), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return parent, child
}

// ---------- Construction ----------

func TestNewWelcomeModel_StartsInParent(t *testing.T) {
	parent, child := setupTestDir(t, "myproject", "other")

	m := NewWelcomeModel(child)

	if m.currentDir != parent {
		t.Fatalf("expected currentDir=%q (parent), got %q", parent, m.currentDir)
	}
	if len(m.entries) != 2 {
		t.Fatalf("expected 2 entries in parent, got %d", len(m.entries))
	}
}

func TestNewWelcomeModel_PreSelectsCWD(t *testing.T) {
	_, child := setupTestDir(t, "target", "aaa", "zzz")

	m := NewWelcomeModel(child)

	// "target" should be pre-selected (cursor points to it)
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		t.Fatalf("cursor %d out of range [0, %d)", m.cursor, len(m.entries))
	}
	if m.entries[m.cursor].Name() != "target" {
		t.Fatalf("expected cursor on 'target', got %q", m.entries[m.cursor].Name())
	}
}

// ---------- Cursor movement ----------

func TestCursorDown(t *testing.T) {
	_, child := setupTestDir(t, "aaa", "bbb", "ccc", "ddd", "eee")
	m := NewWelcomeModel(child)
	// cursor starts on "aaa" (index 0)
	start := m.cursor

	m, _ = m.Update(downKey())
	if m.cursor != start+1 {
		t.Fatalf("expected cursor=%d, got %d", start+1, m.cursor)
	}
}

func TestCursorDown_Clamps(t *testing.T) {
	_, child := setupTestDir(t, "aaa", "bbb")
	m := NewWelcomeModel(child)

	// Move to last
	for range len(m.entries) + 2 {
		m, _ = m.Update(downKey())
	}
	if m.cursor != len(m.entries)-1 {
		t.Fatalf("expected cursor clamped at %d, got %d", len(m.entries)-1, m.cursor)
	}
}

func TestCursorUp(t *testing.T) {
	_, child := setupTestDir(t, "aaa", "bbb", "ccc", "ddd", "eee")
	m := NewWelcomeModel(child)

	// Move down twice, then up
	m, _ = m.Update(downKey())
	m, _ = m.Update(downKey())
	pos := m.cursor
	m, _ = m.Update(upKey())
	if m.cursor != pos-1 {
		t.Fatalf("expected cursor=%d, got %d", pos-1, m.cursor)
	}
}

func TestCursorUp_ClampsAtZero(t *testing.T) {
	_, child := setupTestDir(t, "zzz", "aaa")
	m := NewWelcomeModel(child)

	// Move to top first
	m, _ = m.Update(topKey())
	m, _ = m.Update(upKey())
	if m.cursor != 0 {
		t.Fatalf("expected cursor clamped at 0, got %d", m.cursor)
	}
}

func TestJumpToTop(t *testing.T) {
	_, child := setupTestDir(t, "aaa", "bbb", "ccc", "ddd", "eee")
	m := NewWelcomeModel(child)

	m, _ = m.Update(bottomKey())
	m, _ = m.Update(topKey())
	if m.cursor != 0 {
		t.Fatalf("expected cursor=0, got %d", m.cursor)
	}
}

func TestJumpToBottom(t *testing.T) {
	_, child := setupTestDir(t, "aaa", "bbb", "ccc", "ddd", "eee")
	m := NewWelcomeModel(child)

	m, _ = m.Update(bottomKey())
	if m.cursor != len(m.entries)-1 {
		t.Fatalf("expected cursor=%d, got %d", len(m.entries)-1, m.cursor)
	}
}

// ---------- Navigation ----------

func TestEnterOnDirectory_Descends(t *testing.T) {
	parent, _ := setupTestDir(t, "target")
	// Create a grandchild so "target" has contents
	grandchild := filepath.Join(parent, "target", "inner")
	os.Mkdir(grandchild, 0o755)

	m := NewWelcomeModel(filepath.Join(parent, "target"))
	// Cursor is on "target" in parent listing. Enter should descend.
	targetDir := filepath.Join(parent, m.entries[m.cursor].Name())
	m, _ = m.Update(enterKey())
	if m.currentDir != targetDir {
		t.Fatalf("expected currentDir=%q, got %q", targetDir, m.currentDir)
	}
}

func TestLKey_DescendsButDoesNotInit(t *testing.T) {
	parent := t.TempDir()
	// Create an empty child (no subdirs) so enter would try init
	child := filepath.Join(parent, "empty")
	os.Mkdir(child, 0o755)

	m := NewWelcomeModel(child)
	// Cursor should be on "empty". l should descend into it.
	m, _ = m.Update(enterKey()) // descend into empty
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
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)
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
	parent := t.TempDir()
	child := filepath.Join(parent, "empty")
	os.Mkdir(child, 0o755)

	m := NewWelcomeModel(child)
	// Descend into "empty" first
	m, _ = m.Update(enterKey())

	// Now in empty dir, Enter should init
	m, cmd := m.Update(enterKey())
	if !m.initializing {
		t.Fatal("expected initializing=true")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command from init start")
	}
}

func TestEnterOnDotWolfcastle_StartsInit(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "project")
	os.Mkdir(child, 0o755)
	os.Mkdir(filepath.Join(child, ".wolfcastle"), 0o755)

	m := NewWelcomeModel(child)
	// Descend into "project"
	m, _ = m.Update(enterKey())
	// .wolfcastle should be visible
	found := false
	for i, e := range m.entries {
		if e.Name() == ".wolfcastle" {
			m.cursor = i
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected .wolfcastle in entries")
	}

	m, cmd := m.Update(enterKey())
	if !m.initializing {
		t.Fatal("expected initializing=true after selecting .wolfcastle")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
}

func TestInitCompleteMsg_Success(t *testing.T) {
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)
	m.initializing = true

	m, _ = m.Update(tui.InitCompleteMsg{Dir: child, Err: nil})
	if m.initializing {
		t.Fatal("expected initializing=false after success")
	}
	if m.err != nil {
		t.Fatalf("expected err=nil, got %v", m.err)
	}
}

func TestInitCompleteMsg_Failure(t *testing.T) {
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)
	m.initializing = true

	testErr := errors.New("boom")
	m, _ = m.Update(tui.InitCompleteMsg{Dir: child, Err: testErr})
	if m.initializing {
		t.Fatal("expected initializing=false")
	}
	if m.err == nil || m.err.Error() != "boom" {
		t.Fatalf("expected err=boom, got %v", m.err)
	}
}

// ---------- Spinner ----------

func TestSpinnerTickMsg_WhileInitializing(t *testing.T) {
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)
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
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)
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
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)

	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.width != 120 || m.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", m.width, m.height)
	}
}

func TestSetSize(t *testing.T) {
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)

	m.SetSize(100, 50)
	if m.width != 100 || m.height != 50 {
		t.Fatalf("expected 100x50, got %dx%d", m.width, m.height)
	}
}

// ---------- View rendering ----------

func TestView_ShowsTitle(t *testing.T) {
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "WOLFCASTLE") {
		t.Fatal("expected WOLFCASTLE in view output")
	}
}

func TestView_ShowsTreeConnectors(t *testing.T) {
	_, child := setupTestDir(t, "bbb", "aaa", "ccc")
	m := NewWelcomeModel(child)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "├──") && !strings.Contains(view, "└──") {
		t.Fatal("expected tree connectors in view")
	}
}

func TestView_WithError(t *testing.T) {
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)
	m.SetSize(80, 24)
	m.err = errors.New("test error")

	view := m.View()
	if !strings.Contains(view, "Init failed") {
		t.Fatal("expected 'Init failed' in view output")
	}
}

func TestView_WhileInitializing(t *testing.T) {
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)
	m.SetSize(80, 24)
	m.initializing = true

	view := m.View()
	if !strings.Contains(view, "Initializing") {
		t.Fatal("expected 'Initializing' in view output")
	}
}

func TestView_EmptyDir_ShowsHint(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "empty")
	os.Mkdir(child, 0o755)

	m := NewWelcomeModel(child)
	// Descend into empty
	m, _ = m.Update(enterKey())
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "empty directory") {
		t.Fatal("expected empty directory hint")
	}
}

func TestView_WithEntries_ShowsDirNames(t *testing.T) {
	_, child := setupTestDir(t, "target", "alpha", "beta")
	m := NewWelcomeModel(child)
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
	_, child := setupTestDir(t, "target", "other")
	m := NewWelcomeModel(child)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "▸") {
		t.Fatal("expected ▸ marker on selected entry")
	}
}

func TestView_KeyHints(t *testing.T) {
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "quit") {
		t.Fatal("expected quit hint in view")
	}
}

// ---------- Filtering ----------

func TestHiddenDirsFiltered_ExceptWolfcastle(t *testing.T) {
	parent := t.TempDir()
	os.Mkdir(filepath.Join(parent, ".git"), 0o755)
	os.Mkdir(filepath.Join(parent, ".hidden"), 0o755)
	os.Mkdir(filepath.Join(parent, ".wolfcastle"), 0o755)
	os.Mkdir(filepath.Join(parent, "visible"), 0o755)
	child := filepath.Join(parent, "visible")

	m := NewWelcomeModel(child)

	// m is in parent, should see .wolfcastle and visible
	names := entryNames(m.entries)
	if len(names) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(names), names)
	}
	if names[0] != ".wolfcastle" || names[1] != "visible" {
		t.Fatalf("expected [.wolfcastle visible], got %v", names)
	}
}

func TestOnlyDirsShown_NoFiles(t *testing.T) {
	parent := t.TempDir()
	os.Mkdir(filepath.Join(parent, "subdir"), 0o755)
	os.WriteFile(filepath.Join(parent, "file.txt"), []byte("hello"), 0o644)
	child := filepath.Join(parent, "subdir")

	m := NewWelcomeModel(child)

	// In parent, should see only subdir
	if len(m.entries) != 1 {
		t.Fatalf("expected 1 dir entry, got %d: %v", len(m.entries), entryNames(m.entries))
	}
	if m.entries[0].Name() != "subdir" {
		t.Fatalf("expected 'subdir', got %q", m.entries[0].Name())
	}
}

// ---------- Quit ----------

func TestQuitKey(t *testing.T) {
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)

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
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)
	m.initializing = true

	_, cmd := m.Update(quitKey())
	if cmd == nil {
		t.Fatal("expected quit command even while initializing")
	}
}

func TestKeysSwallowedDuringInit(t *testing.T) {
	_, child := setupTestDir(t, "aaa", "bbb", "ccc")
	m := NewWelcomeModel(child)
	m.initializing = true
	startCursor := m.cursor

	m2, _ := m.Update(downKey())
	if m2.cursor != startCursor {
		t.Fatal("cursor should not move during init")
	}
}

// ---------- Scrolling ----------

func TestScrolling(t *testing.T) {
	parent := t.TempDir()
	for i := range 25 {
		name := "d" + strings.Repeat("0", 2-len(intStr(i))) + intStr(i)
		os.Mkdir(filepath.Join(parent, name), 0o755)
	}
	// Pass one of the dirs as the "child"
	child := filepath.Join(parent, "d00")

	m := NewWelcomeModel(child)
	if len(m.entries) != 25 {
		t.Fatalf("expected 25 entries, got %d", len(m.entries))
	}

	m, _ = m.Update(bottomKey())
	if m.cursor != 24 {
		t.Fatalf("expected cursor=24, got %d", m.cursor)
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
	parent := t.TempDir()
	for i := range 25 {
		name := "d" + strings.Repeat("0", 2-len(intStr(i))) + intStr(i)
		os.Mkdir(filepath.Join(parent, name), 0o755)
	}
	child := filepath.Join(parent, "d00")

	m := NewWelcomeModel(child)
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
	_, child := setupTestDir(t, "target")
	m := NewWelcomeModel(child)
	origCursor := m.cursor
	origDir := m.currentDir

	type unknownMsg struct{}
	m, cmd := m.Update(unknownMsg{})
	if m.cursor != origCursor || m.currentDir != origDir || cmd != nil {
		t.Fatal("unknown message should be a no-op")
	}
}

// ---------- Helpers ----------

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
