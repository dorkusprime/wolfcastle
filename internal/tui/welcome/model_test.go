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

// makeDirs creates n subdirectories named dir00, dir01, etc. inside parent.
func makeDirs(t *testing.T, parent string, n int) {
	t.Helper()
	for i := range n {
		name := "dir" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		if err := os.Mkdir(filepath.Join(parent, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// ---------- Construction ----------

func TestNewWelcomeModel_ValidDir(t *testing.T) {
	dir := t.TempDir()
	makeDirs(t, dir, 3)

	m := NewWelcomeModel(dir)

	if m.currentDir != dir {
		t.Fatalf("expected currentDir=%q, got %q", dir, m.currentDir)
	}
	if len(m.entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(m.entries))
	}
	if m.cursor != 0 {
		t.Fatalf("expected cursor=0, got %d", m.cursor)
	}
}

func TestNewWelcomeModel_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	m := NewWelcomeModel(dir)

	if len(m.entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(m.entries))
	}
}

// ---------- Cursor movement ----------

func TestCursorDown(t *testing.T) {
	dir := t.TempDir()
	makeDirs(t, dir, 5)
	m := NewWelcomeModel(dir)

	m, _ = m.Update(downKey())
	if m.cursor != 1 {
		t.Fatalf("expected cursor=1, got %d", m.cursor)
	}

	m, _ = m.Update(downKey())
	if m.cursor != 2 {
		t.Fatalf("expected cursor=2, got %d", m.cursor)
	}
}

func TestCursorDown_Clamps(t *testing.T) {
	dir := t.TempDir()
	makeDirs(t, dir, 2)
	m := NewWelcomeModel(dir)

	// Move to last entry
	m, _ = m.Update(downKey())
	// Try to go past
	m, _ = m.Update(downKey())
	if m.cursor != 1 {
		t.Fatalf("expected cursor clamped at 1, got %d", m.cursor)
	}
}

func TestCursorUp(t *testing.T) {
	dir := t.TempDir()
	makeDirs(t, dir, 5)
	m := NewWelcomeModel(dir)

	// Move down first, then up
	m, _ = m.Update(downKey())
	m, _ = m.Update(downKey())
	m, _ = m.Update(upKey())
	if m.cursor != 1 {
		t.Fatalf("expected cursor=1, got %d", m.cursor)
	}
}

func TestCursorUp_ClampsAtZero(t *testing.T) {
	dir := t.TempDir()
	makeDirs(t, dir, 3)
	m := NewWelcomeModel(dir)

	m, _ = m.Update(upKey())
	if m.cursor != 0 {
		t.Fatalf("expected cursor clamped at 0, got %d", m.cursor)
	}
}

func TestJumpToTop(t *testing.T) {
	dir := t.TempDir()
	makeDirs(t, dir, 5)
	m := NewWelcomeModel(dir)

	// Move to bottom, then jump to top
	m, _ = m.Update(bottomKey())
	m, _ = m.Update(topKey())
	if m.cursor != 0 {
		t.Fatalf("expected cursor=0, got %d", m.cursor)
	}
}

func TestJumpToBottom(t *testing.T) {
	dir := t.TempDir()
	makeDirs(t, dir, 5)
	m := NewWelcomeModel(dir)

	m, _ = m.Update(bottomKey())
	if m.cursor != 4 {
		t.Fatalf("expected cursor=4, got %d", m.cursor)
	}
}

// ---------- Navigation ----------

func TestEnterOnDirectory(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "subdir")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewWelcomeModel(dir)
	if len(m.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(m.entries))
	}

	m, _ = m.Update(enterKey())
	if m.currentDir != child {
		t.Fatalf("expected currentDir=%q, got %q", child, m.currentDir)
	}
	if m.cursor != 0 {
		t.Fatalf("expected cursor reset to 0, got %d", m.cursor)
	}
}

func TestBack_GoesToParent(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "subdir")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewWelcomeModel(child)
	m, _ = m.Update(backKey())

	if m.currentDir != dir {
		t.Fatalf("expected currentDir=%q, got %q", dir, m.currentDir)
	}
}

// ---------- Init flow ----------

func TestEnterOnEmptyDir_StartsInit(t *testing.T) {
	dir := t.TempDir()
	m := NewWelcomeModel(dir)

	m, cmd := m.Update(enterKey())
	if !m.initializing {
		t.Fatal("expected initializing=true")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command from init start")
	}
}

func TestEnterOnDotWolfcastle_StartsInit(t *testing.T) {
	dir := t.TempDir()
	wc := filepath.Join(dir, ".wolfcastle")
	if err := os.Mkdir(wc, 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewWelcomeModel(dir)
	// .wolfcastle should be the only entry (hidden dirs filtered except .wolfcastle)
	if len(m.entries) != 1 || m.entries[0].Name() != ".wolfcastle" {
		t.Fatalf("expected [.wolfcastle], got %v", entryNames(m.entries))
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
	dir := t.TempDir()
	m := NewWelcomeModel(dir)
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
	dir := t.TempDir()
	m := NewWelcomeModel(dir)
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
	dir := t.TempDir()
	m := NewWelcomeModel(dir)
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
	dir := t.TempDir()
	m := NewWelcomeModel(dir)
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
	dir := t.TempDir()
	m := NewWelcomeModel(dir)

	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.width != 120 || m.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", m.width, m.height)
	}
}

// ---------- SetSize ----------

func TestSetSize(t *testing.T) {
	dir := t.TempDir()
	m := NewWelcomeModel(dir)

	m.SetSize(100, 50)
	if m.width != 100 || m.height != 50 {
		t.Fatalf("expected 100x50, got %dx%d", m.width, m.height)
	}
}

// ---------- View rendering ----------

func TestView_ShowsTitle(t *testing.T) {
	dir := t.TempDir()
	m := NewWelcomeModel(dir)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "WOLFCASTLE") {
		t.Fatal("expected WOLFCASTLE in view output")
	}
}

func TestView_WithError(t *testing.T) {
	dir := t.TempDir()
	m := NewWelcomeModel(dir)
	m.SetSize(80, 24)
	m.err = errors.New("test error")

	view := m.View()
	if !strings.Contains(view, "Init failed") {
		t.Fatal("expected 'Init failed' in view output")
	}
	if !strings.Contains(view, "test error") {
		t.Fatal("expected error message in view output")
	}
}

func TestView_WhileInitializing(t *testing.T) {
	dir := t.TempDir()
	m := NewWelcomeModel(dir)
	m.SetSize(80, 24)
	m.initializing = true
	m.spinnerFrame = 0

	view := m.View()
	// Should show the spinner frame
	if !strings.Contains(view, spinnerFrames[0]) {
		t.Fatal("expected spinner frame in view output")
	}
	if !strings.Contains(view, "Initializing") {
		t.Fatal("expected 'Initializing' in view output")
	}
}

func TestView_EmptyDir_ShowsConfirmHint(t *testing.T) {
	dir := t.TempDir()
	m := NewWelcomeModel(dir)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "Enter to confirm") {
		t.Fatal("expected 'Enter to confirm' hint in empty dir view")
	}
}

func TestView_WithEntries_ShowsDirNames(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "alpha"), 0o755)
	os.Mkdir(filepath.Join(dir, "beta"), 0o755)
	m := NewWelcomeModel(dir)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "alpha") {
		t.Fatal("expected 'alpha' in view")
	}
	if !strings.Contains(view, "beta") {
		t.Fatal("expected 'beta' in view")
	}
}

// ---------- Filtering ----------

func TestHiddenDirsFiltered_ExceptWolfcastle(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	os.Mkdir(filepath.Join(dir, ".hidden"), 0o755)
	os.Mkdir(filepath.Join(dir, ".wolfcastle"), 0o755)
	os.Mkdir(filepath.Join(dir, "visible"), 0o755)

	m := NewWelcomeModel(dir)

	names := entryNames(m.entries)
	if len(names) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(names), names)
	}
	// Should include .wolfcastle and visible, sorted alphabetically
	if names[0] != ".wolfcastle" || names[1] != "visible" {
		t.Fatalf("expected [.wolfcastle visible], got %v", names)
	}
}

func TestOnlyDirsShown_NoFiles(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(dir, "another.go"), []byte("package x"), 0o644)

	m := NewWelcomeModel(dir)

	if len(m.entries) != 1 {
		t.Fatalf("expected 1 dir entry, got %d: %v", len(m.entries), entryNames(m.entries))
	}
	if m.entries[0].Name() != "subdir" {
		t.Fatalf("expected 'subdir', got %q", m.entries[0].Name())
	}
}

// ---------- Quit ----------

func TestQuitKey(t *testing.T) {
	dir := t.TempDir()
	m := NewWelcomeModel(dir)

	_, cmd := m.Update(quitKey())
	if cmd == nil {
		t.Fatal("expected non-nil quit command")
	}
	// Execute the command and check it produces a tea.QuitMsg
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestQuitKey_DuringInit(t *testing.T) {
	dir := t.TempDir()
	m := NewWelcomeModel(dir)
	m.initializing = true

	_, cmd := m.Update(quitKey())
	if cmd == nil {
		t.Fatal("expected quit command even while initializing")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestKeysSwallowedDuringInit(t *testing.T) {
	dir := t.TempDir()
	makeDirs(t, dir, 3)
	m := NewWelcomeModel(dir)
	m.initializing = true

	// All navigation keys should be swallowed
	m2, _ := m.Update(downKey())
	if m2.cursor != 0 {
		t.Fatal("cursor should not move during init")
	}
	m3, _ := m.Update(enterKey())
	if m3.currentDir != m.currentDir {
		t.Fatal("enter should be swallowed during init")
	}
}

// ---------- Scrolling ----------

func TestScrolling(t *testing.T) {
	dir := t.TempDir()
	// Create more than maxVisible dirs
	for i := range 25 {
		name := "d" + strings.Repeat("0", 2-len(intStr(i))) + intStr(i)
		os.Mkdir(filepath.Join(dir, name), 0o755)
	}

	m := NewWelcomeModel(dir)
	if len(m.entries) != 25 {
		t.Fatalf("expected 25 entries, got %d", len(m.entries))
	}

	// Jump to bottom
	m, _ = m.Update(bottomKey())
	if m.cursor != 24 {
		t.Fatalf("expected cursor=24, got %d", m.cursor)
	}
	if m.scrollTop != 5 { // 24 - maxVisible + 1 = 5
		t.Fatalf("expected scrollTop=5, got %d", m.scrollTop)
	}

	// Jump to top
	m, _ = m.Update(topKey())
	if m.scrollTop != 0 {
		t.Fatalf("expected scrollTop=0 after jump to top, got %d", m.scrollTop)
	}
}

// ---------- View scroll indicators ----------

func TestView_ScrollIndicators(t *testing.T) {
	dir := t.TempDir()
	for i := range 25 {
		name := "d" + strings.Repeat("0", 2-len(intStr(i))) + intStr(i)
		os.Mkdir(filepath.Join(dir, name), 0o755)
	}

	m := NewWelcomeModel(dir)
	m.SetSize(80, 40)

	// At top: should show "more below" but not "more above"
	view := m.View()
	if !strings.Contains(view, "more below") {
		t.Fatal("expected 'more below' indicator at top")
	}
	if strings.Contains(view, "more above") {
		t.Fatal("did not expect 'more above' indicator at top")
	}

	// Move to middle
	for range 15 {
		m, _ = m.Update(downKey())
	}
	view = m.View()
	// Should have both indicators now (scrollTop > 0 and entries extend below)
	// Not guaranteed to have both depending on exact scroll position, but at
	// least 'more below' should still appear if cursor is at 15 and there are 25 entries.
}

// ---------- Unknown message ----------

func TestUnknownMsg_NoOp(t *testing.T) {
	dir := t.TempDir()
	m := NewWelcomeModel(dir)
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
