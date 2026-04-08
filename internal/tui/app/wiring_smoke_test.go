package app

// Wiring smoke tests cover the gap that lets pieces of the TUI be
// implemented end-to-end at the leaf layer (renderer, watcher, log
// view) but never connected by the message-routing layer at the top
// of app.go. The watcher dead-wiring bug, the active-target dead
// wiring, and the search-after-fold staleness all share that shape:
// the leaf is correct, the wiring is missing, and unit tests of
// either side in isolation can't catch it.
//
// Each test in this file feeds a real message into a real model,
// runs Update, then renders the model and asserts on the rendered
// output (or on the model's surface state). The minimum-viable
// assertion is "the bug is no longer reproducible," not "the
// behavior matches a specific design choice" — the design choice
// gets made later when the fix is implemented.
//
// All tests in this file should fail against the version of main
// that introduced the wiring bug, and pass once the wiring is
// repaired. Skipping or hand-waving an assertion to make a test
// green is a regression of the bug it was written to catch.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// smokeIndex builds a tiny RootIndex with one orchestrator parent and
// two leaf children that have IDENTICAL status. The identical status
// is critical for tests that compare row rendering — if the children
// had different statuses (different glyphs, different colors), their
// rendered strings would naturally differ and the test would pass
// even when the property under test (e.g., current-target highlight)
// is missing.
func smokeIndex() *state.RootIndex {
	return &state.RootIndex{
		Version: 1,
		Root:    []string{"alpha"},
		Nodes: map[string]state.IndexEntry{
			"alpha": {
				Name:     "alpha",
				Type:     state.NodeOrchestrator,
				State:    state.StatusInProgress,
				Address:  "alpha",
				Children: []string{"alpha/beta", "alpha/gamma"},
			},
			"alpha/beta": {
				Name:    "beta",
				Type:    state.NodeLeaf,
				State:   state.StatusInProgress,
				Address: "alpha/beta",
				Parent:  "alpha",
			},
			"alpha/gamma": {
				Name:    "gamma",
				Type:    state.NodeLeaf,
				State:   state.StatusInProgress,
				Address: "alpha/gamma",
				Parent:  "alpha",
			},
		},
	}
}

// smokeLeafState constructs a leaf NodeState with the given task
// titles, used to populate the tree's node cache via NodeUpdatedMsg.
func smokeLeafState(addr, name string, taskTitles ...string) *state.NodeState {
	tasks := make([]state.Task, 0, len(taskTitles))
	for i, title := range taskTitles {
		tasks = append(tasks, state.Task{
			ID:    fmt.Sprintf("task-%04d", i+1),
			Title: title,
			State: state.StatusNotStarted,
		})
	}
	return &state.NodeState{
		ID:    addr,
		Name:  name,
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: tasks,
	}
}

// renderTree returns the tree pane's rendered string from a fully
// propagated model.
func renderTree(m TUIModel) string {
	m.propagateSize()
	return m.tree.View()
}

// ---------------------------------------------------------------------------
// Issue #3: active task highlight reaches the rendered tree
// ---------------------------------------------------------------------------

// TestWiring_ActiveTargetReachesTreeRender feeds a DaemonStatusMsg
// with CurrentNode set, then asserts the rendered tree visually
// distinguishes that node from its sibling. The minimum signal is
// "the row containing the current target renders differently from
// the row of an unrelated sibling."
//
// Should fail today: nothing in app.Update pipes
// DaemonStatusMsg.CurrentNode into m.tree.SetCurrentTarget, so the
// tree renders both rows identically.
func TestWiring_ActiveTargetReachesTreeRender(t *testing.T) {
	m := newColdModel(t)
	idx := smokeIndex()

	// Seed the index so the tree has rows.
	result, _ := m.Update(tui.StateUpdatedMsg{Index: idx})
	m = toModel(t, result)

	// Expand alpha so its children (beta and gamma) become visible
	// rows. The smoke test cares about the highlight state of the
	// children, not the parent.
	m.tree.SetCursor(0)
	tm, _ := m.tree.Update(keyMsg("enter"))
	m.tree = tm

	// Tell the model the daemon is currently working on alpha/beta.
	// This is the message detectEntryState produces in real runs.
	result, _ = m.Update(tui.DaemonStatusMsg{
		IsRunning:    true,
		CurrentNode:  "alpha/beta",
		CurrentTask:  "task-0001",
		LastActivity: time.Now(),
	})
	m = toModel(t, result)

	rendered := renderTree(m)
	if rendered == "" {
		t.Fatal("tree rendered empty; cannot assert highlight")
	}

	// Beta and gamma have identical statuses, so any difference in
	// their rendered styling must come from one of them being the
	// daemon's current target. Rename both to the same placeholder
	// so the literal name is not the source of the difference, then
	// strip whitespace padding (which depends on name length) so
	// only style/decoration bytes remain.
	lines := strings.Split(rendered, "\n")
	var betaLine, gammaLine string
	for _, line := range lines {
		if strings.Contains(line, "beta") && betaLine == "" {
			betaLine = line
		}
		if strings.Contains(line, "gamma") && gammaLine == "" {
			gammaLine = line
		}
	}
	if betaLine == "" || gammaLine == "" {
		t.Fatalf("tree rendering missing expected rows; got:\n%s", rendered)
	}

	normalize := func(s string) string {
		// Replace the literal name with a placeholder so name length
		// doesn't affect padding.
		s = strings.ReplaceAll(s, "beta", "X")
		s = strings.ReplaceAll(s, "gamma", "X")
		// Collapse runs of spaces so trailing-padding differences
		// don't matter.
		for strings.Contains(s, "  ") {
			s = strings.ReplaceAll(s, "  ", " ")
		}
		return strings.TrimSpace(s)
	}

	if normalize(betaLine) == normalize(gammaLine) {
		t.Errorf("active target alpha/beta should render differently from sibling alpha/gamma, but both rows look identical after name and whitespace normalization\nbeta:  %q\ngamma: %q", betaLine, gammaLine)
	}
}

// ---------------------------------------------------------------------------
// Issue #4b: log view shows existing file content on switch
// ---------------------------------------------------------------------------

// TestWiring_LogViewShowsExistingContent stages a real log file
// containing valid NDJSON records, points the model at that log
// directory, switches the detail pane to log stream view, and
// asserts the rendered view does not show the empty-state
// placeholder.
//
// Should fail today: LogViewModel.Update only handles incremental
// LogLinesMsg / NewLogFileMsg. There is no code path that loads
// existing file content on switch-to-log-view, and the watcher's
// Start seeds logOffset = file size so the first poll tick sees no
// growth and emits nothing.
func TestWiring_LogViewShowsExistingContent(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	logDir := filepath.Join(wcDir, "system", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Stage a real .jsonl file with one valid record. The exact
	// schema doesn't matter for this test — only that the log view
	// has SOMETHING to render.
	rec := map[string]any{
		"ts":    "2026-04-08T07:30:00Z",
		"level": "info",
		"msg":   "wiring smoke test record",
	}
	data, _ := json.Marshal(rec)
	logFile := filepath.Join(logDir, "0001-exec-20260408T07-30Z.jsonl")
	if err := os.WriteFile(logFile, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	// Construct a model whose store and daemon repo point at the
	// staged worktree, then switch the detail pane to log view.
	store := state.NewStore(filepath.Join(wcDir, "system", "projects", "default"), 0)
	m := NewTUIModel(store, daemon.NewDaemonRepository(wcDir), tmp, "1.0.0")
	m.entryState = StateLive
	m.width = 120
	m.height = 40
	m.propagateSize()
	m.detail.SwitchToLogView()
	m.focused = PaneDetail
	m.syncFocus()

	view := m.detail.View()
	if strings.Contains(view, "No transmissions") {
		t.Errorf("log view should show existing log file content on switch, but rendered the empty-state placeholder. Staged file: %s\nView:\n%s", logFile, view)
	}
}

// ---------------------------------------------------------------------------
// Issue #4c: LatestLogFile selection
// ---------------------------------------------------------------------------

// TestWiring_LatestLogFile_PrefersNewestExec creates a logs directory
// with a mix of exec and intake files where lex-sort and "newest by
// activity" disagree. The intake counter is far higher than the exec
// counter (10000+ vs 200s), which mirrors the actual situation in
// /tmp/wc-tui-test today. The latest file by user-meaningful order
// should be the most recent EXEC file, not the lex-max intake/inbox
// file.
//
// Should fail today: logging.LatestLogFile uses sort.Strings which
// is dominated by leading-digit lex order, so it picks the
// 10168-inbox-init file instead of 0280-exec.
func TestWiring_LatestLogFile_PrefersNewestExec(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"0279-exec-20260408T14-42Z.jsonl.gz",
		"10166-inbox-init-20260408T14-30Z.jsonl",
		"10167-intake-20260408T14-37Z.jsonl",
		"0280-exec-20260408T14-46Z.jsonl",
		"10168-inbox-init-20260408T14-50Z.jsonl",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := logging.LatestLogFile(dir)
	if err != nil {
		t.Fatalf("LatestLogFile: %v", err)
	}
	want := filepath.Join(dir, "0280-exec-20260408T14-46Z.jsonl")
	if got != want {
		t.Errorf("LatestLogFile picked %q, want %q (the newest exec file by iteration; lex-max would incorrectly pick the inbox-init file because of digit-width disparity)", got, want)
	}
}

// ---------------------------------------------------------------------------
// Issue #1: search highlight survives a fold
// ---------------------------------------------------------------------------

// TestWiring_SearchHighlightSurvivesFold sets up a tree where a
// search match exists deep in an expanded subtree, computes the
// matches, then collapses the parent. After collapse the rendered
// tree must NOT show a highlight at a stale flat-list index. The
// minimum signal: every row that the tree marks as a search hit
// must be a row whose name actually contains the query.
//
// Should fail today: handleCollapse rebuilds the flat list but does
// not recompute m.tree.searchMatches, so the map's old indices now
// point at unrelated rows in the new list.
func TestWiring_SearchHighlightSurvivesFold(t *testing.T) {
	m := newColdModel(t)
	idx := smokeIndex()
	result, _ := m.Update(tui.StateUpdatedMsg{Index: idx})
	m = toModel(t, result)

	// Populate beta with tasks via the cache, one of which contains
	// the search query.
	betaState := smokeLeafState("alpha/beta", "beta",
		"deploy frobnicator",
		"tune knobs",
	)
	result, _ = m.Update(tui.NodeUpdatedMsg{Address: "alpha/beta", Node: betaState})
	m = toModel(t, result)

	// Expand alpha and alpha/beta so the task rows enter the flat list.
	m.tree.SetCursor(0)
	tm, _ := m.tree.Update(keyMsg("enter"))
	m.tree = tm
	for i, row := range m.tree.FlatList() {
		if row.Addr == "alpha/beta" {
			m.tree.SetCursor(i)
			break
		}
	}
	tm, _ = m.tree.Update(keyMsg("enter"))
	m.tree = tm

	// Sanity: the frobnicator task should now be visible.
	taskVisible := false
	for _, row := range m.tree.FlatList() {
		if strings.Contains(row.Name, "frobnicator") {
			taskVisible = true
			break
		}
	}
	if !taskVisible {
		t.Fatal("setup: frobnicator task not in flat list after expand; smoke test cannot proceed")
	}

	// Run a search that matches the frobnicator task.
	m.search.Activate(int(PaneTree))
	for _, ch := range "frob" {
		s, _ := m.search.Update(keyMsg(string(ch)))
		m.search = s
	}
	m.computeTreeSearchMatches()

	// Find the alpha/beta row index in the current flat list,
	// position the cursor on it, and collapse.
	for i, row := range m.tree.FlatList() {
		if row.Addr == "alpha/beta" {
			m.tree.SetCursor(i)
			break
		}
	}
	tm, _ = m.tree.Update(keyMsg("h"))
	m.tree = tm

	// Sanity: the frobnicator row should no longer be in the flat
	// list (it's hidden inside the collapsed leaf).
	for _, row := range m.tree.FlatList() {
		if strings.Contains(row.Name, "frobnicator") {
			t.Fatal("frobnicator should be hidden after collapse; smoke test cannot proceed")
		}
	}

	// Smoke assertion: every row currently marked as a search hit
	// must be a row whose name actually contains the query. A stale
	// index from the pre-fold flat list will mark a row that has
	// nothing to do with "frob".
	matches := m.tree.SearchMatches()
	for idx, row := range m.tree.FlatList() {
		if matches[idx] && !strings.Contains(strings.ToLower(row.Name), "frob") {
			t.Errorf("row %d (%q) is marked as a search hit but does not contain the query 'frob'; this is a stale highlight from before the fold", idx, row.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Issue #2: search finds tasks inside collapsed leaves
// ---------------------------------------------------------------------------

// TestWiring_SearchFindsTasksInUnexpandedLeaves builds a tree with a
// leaf whose tasks are present in the cache but the leaf has never
// been expanded. A search query matching one of those task titles
// must produce at least one match.
//
// Should fail today: computeTreeSearchMatches walks m.tree.FlatList()
// which is the visible flat list. Tasks in unexpanded leaves are not
// in the flat list, so they are not searched.
func TestWiring_SearchFindsTasksInUnexpandedLeaves(t *testing.T) {
	m := newColdModel(t)
	idx := smokeIndex()
	result, _ := m.Update(tui.StateUpdatedMsg{Index: idx})
	m = toModel(t, result)

	// Pre-cache beta's task list. In real runs this happens when
	// the user expands the leaf for the first time, but the cache
	// can also be populated by daemon-side state updates. The
	// search should look at this content regardless of expansion.
	betaState := smokeLeafState("alpha/beta", "beta",
		"deploy frobnicator",
		"tune knobs",
	)
	result, _ = m.Update(tui.NodeUpdatedMsg{Address: "alpha/beta", Node: betaState})
	m = toModel(t, result)

	// alpha/beta is collapsed by default. Confirm the frobnicator
	// task is NOT in the visible flat list.
	for _, row := range m.tree.FlatList() {
		if strings.Contains(row.Name, "frobnicator") {
			t.Fatal("setup error: alpha/beta is supposed to be collapsed but the task is already in the flat list")
		}
	}

	// Run a search that matches the frobnicator task.
	m.search.Activate(int(PaneTree))
	for _, ch := range "frob" {
		s, _ := m.search.Update(keyMsg(string(ch)))
		m.search = s
	}
	m.computeTreeSearchMatches()

	if !m.search.HasMatches() {
		t.Errorf("search for 'frob' should find the cached task in collapsed leaf alpha/beta, but the search model reports zero matches")
	}
}
