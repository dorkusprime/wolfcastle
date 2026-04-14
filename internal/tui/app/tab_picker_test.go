package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dorkusprime/wolfcastle/internal/instance"
)

// helpers ---------------------------------------------------------------

func pickerKey(s string) tea.KeyPressMsg {
	if len(s) == 1 {
		return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
	// Named keys (down, up, enter, esc, home, end, left, h, l).
	switch s {
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "home":
		return tea.KeyPressMsg{Code: 'g', Text: "g"}
	case "end":
		return tea.KeyPressMsg{Code: 'G', Text: "G"}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	}
	return tea.KeyPressMsg{}
}

func mkProjectDir(t *testing.T, parent, name string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.MkdirAll(filepath.Join(dir, ".wolfcastle"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return dir
}

func mkPlainDir(t *testing.T, parent, name string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return dir
}

// tests -----------------------------------------------------------------

func TestNewTabPicker_LoadsCurrentDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkPlainDir(t, root, "alpha")
	mkProjectDir(t, root, "beta-proj")
	mkPlainDir(t, root, "gamma")
	// Hidden dir should be filtered.
	mkPlainDir(t, root, ".hidden")

	m := newTabPicker(root, nil)

	if len(m.entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(m.entries), m.entries)
	}
	// Projects sort first.
	if m.entries[0].name != "beta-proj" || !m.entries[0].hasWolfcastle {
		t.Errorf("expected beta-proj first with hasWolfcastle=true, got %+v", m.entries[0])
	}
	if m.entries[1].name != "alpha" || m.entries[1].hasWolfcastle {
		t.Errorf("expected alpha second, got %+v", m.entries[1])
	}
	if m.entries[2].name != "gamma" {
		t.Errorf("expected gamma third, got %+v", m.entries[2])
	}
}

func TestNewTabPicker_IncludesSessionsFromOtherDirs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkPlainDir(t, root, "here")

	// Session lives outside root — should appear at the top as a session
	// entry.
	other := t.TempDir()
	sessionWorktree := mkProjectDir(t, other, "external-proj")

	m := newTabPicker(root, []instance.Entry{
		{PID: 1234, Worktree: sessionWorktree},
	})

	if len(m.entries) < 1 || !m.entries[0].isSession {
		t.Fatalf("expected session at top of entries, got %+v", m.entries)
	}
	if m.entries[0].worktree != sessionWorktree {
		t.Errorf("session worktree mismatch: got %q", m.entries[0].worktree)
	}
}

func TestNewTabPicker_SkipsSessionsInCurrentDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sessionWorktree := mkProjectDir(t, root, "inner-proj")

	m := newTabPicker(root, []instance.Entry{
		{PID: 42, Worktree: sessionWorktree},
	})

	// Should not produce a duplicate: the session is in root, so it
	// appears as a regular project entry instead of a session header.
	sessionEntries := 0
	for _, e := range m.entries {
		if e.isSession {
			sessionEntries++
		}
	}
	if sessionEntries != 0 {
		t.Errorf("expected 0 session entries (session is in current dir), got %d", sessionEntries)
	}
	if len(m.entries) != 1 || !m.entries[0].hasWolfcastle {
		t.Errorf("expected one project entry for the session dir, got %+v", m.entries)
	}
}

func TestTabPicker_LoadDirError(t *testing.T) {
	t.Parallel()
	m := newTabPicker("/nonexistent/path/that/does/not/exist", nil)
	if m.err == "" {
		t.Error("expected err to be set on unreadable directory")
	}
}

func TestTabPicker_SetSize(t *testing.T) {
	t.Parallel()
	m := newTabPicker(t.TempDir(), nil)
	m.SetSize(100, 40)
	if m.width != 100 || m.height != 40 {
		t.Errorf("SetSize not applied: got %dx%d", m.width, m.height)
	}
}

func TestTabPicker_Movement(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkPlainDir(t, root, "a")
	mkPlainDir(t, root, "b")
	mkPlainDir(t, root, "c")

	m := newTabPicker(root, nil)
	m.SetSize(80, 30)

	if m.cursor != 0 {
		t.Fatalf("initial cursor should be 0, got %d", m.cursor)
	}

	m, _ = m.Update(pickerKey("down"))
	m, _ = m.Update(pickerKey("down"))
	if m.cursor != 2 {
		t.Errorf("after 2 downs, cursor=%d, want 2", m.cursor)
	}

	// Clamp at bottom.
	m, _ = m.Update(pickerKey("down"))
	if m.cursor != 2 {
		t.Errorf("cursor should clamp at last entry, got %d", m.cursor)
	}

	m, _ = m.Update(pickerKey("up"))
	if m.cursor != 1 {
		t.Errorf("after up, cursor=%d, want 1", m.cursor)
	}

	// Clamp at top.
	m, _ = m.Update(pickerKey("up"))
	m, _ = m.Update(pickerKey("up"))
	if m.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", m.cursor)
	}

	// End jumps to last.
	m, _ = m.Update(pickerKey("end"))
	if m.cursor != 2 {
		t.Errorf("end should jump to last, got %d", m.cursor)
	}

	// Home jumps to first.
	m, _ = m.Update(pickerKey("home"))
	if m.cursor != 0 {
		t.Errorf("home should jump to first, got %d", m.cursor)
	}
}

func TestTabPicker_EnterOnProjectEmitsResult(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	proj := mkProjectDir(t, root, "hit-me")

	m := newTabPicker(root, nil)
	m.SetSize(80, 30)

	_, cmd := m.Update(pickerKey("enter"))
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	result, ok := msg.(TabPickerResultMsg)
	if !ok {
		t.Fatalf("expected TabPickerResultMsg, got %T", msg)
	}
	if result.Dir != proj {
		t.Errorf("expected dir %q, got %q", proj, result.Dir)
	}
}

func TestTabPicker_EnterOnSessionEmitsWorktree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkPlainDir(t, root, "placeholder")

	other := t.TempDir()
	session := mkProjectDir(t, other, "remote")

	m := newTabPicker(root, []instance.Entry{
		{PID: 99, Worktree: session},
	})
	m.SetSize(80, 30)

	// Session sits at index 0.
	_, cmd := m.Update(pickerKey("enter"))
	if cmd == nil {
		t.Fatal("expected command")
	}
	result, ok := cmd().(TabPickerResultMsg)
	if !ok {
		t.Fatalf("expected TabPickerResultMsg, got %T", cmd())
	}
	if result.Dir != session {
		t.Errorf("session enter should emit worktree path %q, got %q", session, result.Dir)
	}
}

func TestTabPicker_EnterOnPlainDirDescends(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	child := mkPlainDir(t, root, "only-plain")
	mkPlainDir(t, child, "grandchild")

	m := newTabPicker(root, nil)
	m.SetSize(80, 30)

	m, cmd := m.Update(pickerKey("enter"))
	if cmd != nil {
		t.Errorf("expected nil cmd when descending into plain dir")
	}
	if m.dir != child {
		t.Errorf("expected dir to become %q, got %q", child, m.dir)
	}
	if len(m.entries) != 1 || m.entries[0].name != "grandchild" {
		t.Errorf("expected grandchild entry after descent, got %+v", m.entries)
	}
}

func TestTabPicker_BackNavigatesToParent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	child := mkPlainDir(t, root, "child")

	m := newTabPicker(child, nil)
	m.SetSize(80, 30)

	m, _ = m.Update(pickerKey("left"))
	if m.dir != root {
		t.Errorf("expected back to go to parent %q, got %q", root, m.dir)
	}
}

func TestTabPicker_BackAtRootIsNoop(t *testing.T) {
	t.Parallel()
	m := newTabPicker("/", nil)
	m.SetSize(80, 30)

	m, _ = m.Update(pickerKey("left"))
	if m.dir != "/" {
		t.Errorf("back at root should be noop, got dir=%q", m.dir)
	}
}

func TestTabPicker_EscEmitsCancel(t *testing.T) {
	t.Parallel()
	m := newTabPicker(t.TempDir(), nil)
	m.SetSize(80, 30)

	_, cmd := m.Update(pickerKey("esc"))
	if cmd == nil {
		t.Fatal("expected cancel command")
	}
	if _, ok := cmd().(TabPickerCancelMsg); !ok {
		t.Errorf("expected TabPickerCancelMsg, got %T", cmd())
	}
}

func TestTabPicker_ViewRendersTitle(t *testing.T) {
	t.Parallel()
	m := newTabPicker(t.TempDir(), nil)
	m.SetSize(80, 30)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "NEW TAB") {
		t.Errorf("view should contain NEW TAB title, got %q", view)
	}
}

func TestTabPicker_ViewEmptyDir(t *testing.T) {
	t.Parallel()
	m := newTabPicker(t.TempDir(), nil)
	m.SetSize(80, 30)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Empty directory") {
		t.Errorf("empty dir view should say so, got %q", view)
	}
}

func TestTabPicker_ViewShowsError(t *testing.T) {
	t.Parallel()
	m := newTabPicker("/nonexistent/path/for/view/test", nil)
	m.SetSize(80, 30)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "no such file") && !strings.Contains(view, "not exist") {
		t.Errorf("error view should include error text, got %q", view)
	}
}

func TestTabPicker_ViewRendersEntries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkPlainDir(t, root, "plain-one")
	mkProjectDir(t, root, "project-one")

	m := newTabPicker(root, nil)
	m.SetSize(80, 30)
	view := ansi.Strip(m.View())

	if !strings.Contains(view, "project-one") {
		t.Errorf("view should list project-one, got %q", view)
	}
	if !strings.Contains(view, "plain-one") {
		t.Errorf("view should list plain-one, got %q", view)
	}
	// Project dirs render with the diamond marker.
	if !strings.Contains(view, "◆") {
		t.Errorf("view should mark projects with ◆, got %q", view)
	}
}

func TestTabPicker_ViewRendersSessionEntries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkPlainDir(t, root, "here")

	other := t.TempDir()
	session := mkProjectDir(t, other, "external")

	m := newTabPicker(root, []instance.Entry{
		{PID: 1, Worktree: session},
	})
	m.SetSize(80, 30)
	view := ansi.Strip(m.View())

	if !strings.Contains(view, "external") {
		t.Errorf("view should include session name, got %q", view)
	}
	if !strings.Contains(view, "●") {
		t.Errorf("view should mark sessions with ●, got %q", view)
	}
}

func TestTabPicker_ViewScrollsWithCursor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		mkPlainDir(t, root, name)
	}

	m := newTabPicker(root, nil)
	m.SetSize(80, 10) // visibleHeight = 4

	// Jump to bottom; scroll window should shift so the cursor stays visible.
	m, _ = m.Update(pickerKey("end"))
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "h") {
		t.Errorf("bottom entry should be visible after scrolling, got %q", view)
	}
	// Top-of-list entries should have scrolled off.
	if strings.Contains(view, "  a/") {
		t.Errorf("top entry a should have scrolled off, got %q", view)
	}
}

func TestTabPicker_ViewClampsVisibleHeight(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkPlainDir(t, root, "only")

	m := newTabPicker(root, nil)
	// Height small enough that visibleHeight would be <1 without clamp.
	m.SetSize(80, 3)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "only") {
		t.Errorf("view should still render when height is tight, got %q", view)
	}
}
