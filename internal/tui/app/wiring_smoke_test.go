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
	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
	"github.com/dorkusprime/wolfcastle/internal/tui/tree"
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

// ---------------------------------------------------------------------------
// Issue #3: per-node task cache stays fresh during a session
// ---------------------------------------------------------------------------
//
// The original framing of #3 was a "missing active-task highlight," but
// digging into a real session showed something deeper: the leaf-level
// glyph (◐) does update via the index file watcher, but the per-task
// glyphs underneath stay frozen at whatever the cache held when the
// leaf was first expanded. The actual gap is that the watcher only
// subscribes to the index/instance/log directories, never to per-node
// state.json files. Address-keyed task content reads from the cache,
// the cache is loaded lazily on first expand, and nothing ever
// refreshes it for the rest of the session. AddNodeWatch and
// RemoveNodeWatch exist on the Watcher class but are never called.
//
// The fix is to (a) eagerly walk every leaf in the index at watcher
// startup, populating the cache with the on-disk task state, and
// (b) call AddNodeWatch for each leaf so subsequent state.json
// rewrites by the daemon trigger NodeUpdatedMsg events. This shares
// the leaf-walking code path with #2's eager prefetch for search.

// TestWiring_TaskCacheReflectsStateFile is the smoke test for #3.
// It writes a real state.json with one task in_progress and one
// task complete, runs the watcher startup path that should eagerly
// populate the cache, and asserts the rendered tree shows each task
// with the glyph for its actual on-disk state — not the not_started
// glyph that stale cache would produce.
//
// Should fail today: the eager prefetch does not exist, so the cache
// for the leaf is empty and the tree renders the leaf with no task
// rows at all (or with stale task rows if some other path populated
// them earlier).
func TestWiring_TaskCacheReflectsStateFile(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	storeDir := filepath.Join(wcDir, "system", "projects", "default")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write the root index pointing at one leaf.
	rootIndex := map[string]any{
		"version": 1,
		"root":    []string{"alpha"},
		"nodes": map[string]any{
			"alpha": map[string]any{
				"name":    "alpha",
				"type":    "leaf",
				"state":   "in_progress",
				"address": "alpha",
			},
		},
	}
	rootData, _ := json.Marshal(rootIndex)
	if err := os.WriteFile(filepath.Join(storeDir, "state.json"), rootData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Write the leaf state.json with one in_progress and one complete
	// task. This is the on-disk truth that the cache must reflect.
	leafDir := filepath.Join(storeDir, "alpha")
	if err := os.MkdirAll(leafDir, 0o755); err != nil {
		t.Fatal(err)
	}
	leafState := map[string]any{
		"version": 1,
		"id":      "alpha",
		"name":    "alpha",
		"type":    "leaf",
		"state":   "in_progress",
		"tasks": []map[string]any{
			{
				"id":            "task-0001",
				"title":         "first task",
				"description":   "first task",
				"state":         "complete",
				"failure_count": 0,
			},
			{
				"id":            "task-0002",
				"title":         "second task",
				"description":   "second task",
				"state":         "in_progress",
				"failure_count": 0,
			},
		},
	}
	leafData, _ := json.Marshal(leafState)
	if err := os.WriteFile(filepath.Join(leafDir, "state.json"), leafData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Construct the model and drive the same startup sequence Init()
	// would, except for the blocking waitForWatcherEvent drain. The
	// eager prefetch fix should populate m.tree's node cache for
	// alpha so the rendered tree knows the task states without the
	// user having to expand the leaf manually first.
	store := state.NewStore(storeDir, 0)
	m := NewTUIModel(store, daemon.NewDaemonRepository(wcDir), tmp, "1.0.0")
	m.entryState = StateLive
	m.width = 120
	m.height = 40
	m.propagateSize()

	// Run startWatcher: the watcher gets created and (after the fix)
	// eagerly walks the leaves into the cache via the events channel.
	if cmd := m.startWatcher(); cmd != nil {
		_ = cmd()
	}
	// Run loadInitialState to seed the index so the tree has rows.
	if cmd := m.loadInitialState(); cmd != nil {
		msg := cmd()
		if msg != nil {
			updated, _ := m.Update(msg)
			m = toModel(t, updated)
		}
	}
	// Drain whatever the watcher's eager prefetch deposited into the
	// events channel without blocking. After the fix, this should
	// include one NodeUpdatedMsg per leaf.
	drainNonBlocking(t, &m)

	// Expand alpha so its task rows enter the flat list. After the
	// fix, the tasks should already be in the cache so expand
	// produces fresh rows immediately.
	m.tree.SetCursor(0)
	tm, _ := m.tree.Update(keyMsg("enter"))
	m.tree = tm

	// Find the task rows in the flat list.
	var task1Row, task2Row *tree.TreeRow
	for i := range m.tree.FlatList() {
		row := &m.tree.FlatList()[i]
		if strings.Contains(row.Addr, "task-0001") {
			task1Row = row
		}
		if strings.Contains(row.Addr, "task-0002") {
			task2Row = row
		}
	}
	if task1Row == nil || task2Row == nil {
		t.Fatalf("expected both task rows to be present after expand; got flat list: %+v", m.tree.FlatList())
	}

	// The smoke assertion: each task row's Status field reflects the
	// on-disk state, not the not_started default that an empty cache
	// would produce.
	if task1Row.Status != state.StatusComplete {
		t.Errorf("task-0001 should be StatusComplete (matches state.json), got %v; cache is stale or never loaded", task1Row.Status)
	}
	if task2Row.Status != state.StatusInProgress {
		t.Errorf("task-0002 should be StatusInProgress (matches state.json), got %v; cache is stale or never loaded", task2Row.Status)
	}
}

// drainNonBlocking pops every message currently sitting in the
// model's watcherEvents channel and dispatches each one through
// Update so eager-loaded watcher events reach the tree cache. It
// uses a non-blocking select so it can be called safely even when
// the channel is empty (no production goroutine, just whatever the
// startWatcher cmd already put there before returning).
func drainNonBlocking(t *testing.T, m *TUIModel) {
	t.Helper()
	for i := 0; i < 256; i++ {
		select {
		case msg := <-m.watcherEvents:
			updated, _ := m.Update(msg)
			if tm, ok := updated.(TUIModel); ok {
				*m = tm
			}
		default:
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Model-level wiring test for #3 (startWatcher → eager prefetch)
// ---------------------------------------------------------------------------

// TestStartWatcher_TriggersEagerPrefetch is the model-level
// companion to TestEagerPrefetchAndSubscribe_PopulatesCache. The
// watcher unit test proves the eager-prefetch helper works in
// isolation; this test proves the production wiring (startWatcher
// cmd → NewWatcher → Start → EagerPrefetchAndSubscribe) actually
// invokes it. Without this, someone could rip the
// EagerPrefetchAndSubscribe call out of startWatcher and the
// helper's unit tests would still pass while the cache went stale
// in real use.
func TestStartWatcher_TriggersEagerPrefetch(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	storeDir := filepath.Join(wcDir, "system", "projects", "default")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Stage the index pointing at one leaf.
	rootIndex := map[string]any{
		"version": 1,
		"root":    []string{"alpha"},
		"nodes": map[string]any{
			"alpha": map[string]any{
				"name":    "alpha",
				"type":    "leaf",
				"state":   "in_progress",
				"address": "alpha",
			},
		},
	}
	rootData, _ := json.Marshal(rootIndex)
	if err := os.WriteFile(filepath.Join(storeDir, "state.json"), rootData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Stage the leaf state with a known set of tasks.
	leafDir := filepath.Join(storeDir, "alpha")
	if err := os.MkdirAll(leafDir, 0o755); err != nil {
		t.Fatal(err)
	}
	leafState := map[string]any{
		"version": 1,
		"id":      "alpha",
		"name":    "alpha",
		"type":    "leaf",
		"state":   "in_progress",
		"tasks": []map[string]any{
			{"id": "task-0001", "title": "wired", "description": "wired", "state": "complete", "failure_count": 0},
		},
	}
	leafData, _ := json.Marshal(leafState)
	if err := os.WriteFile(filepath.Join(leafDir, "state.json"), leafData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Construct the model and run startWatcher cmd.
	store := state.NewStore(storeDir, 0)
	m := NewTUIModel(store, daemon.NewDaemonRepository(wcDir), tmp, "1.0.0")
	m.entryState = StateLive
	m.width = 120
	m.height = 40
	m.propagateSize()

	cmd := m.startWatcher()
	if cmd == nil {
		t.Fatal("startWatcher returned nil cmd")
	}
	if msg := cmd(); msg != nil {
		t.Fatalf("startWatcher cmd should return nil msg, got %T", msg)
	}

	// At this point the watcher should have eagerly prefetched
	// alpha's state into the events channel. Drain and assert.
	found := false
drainLoop:
	for i := 0; i < 8; i++ {
		select {
		case msg := <-m.watcherEvents:
			envelope, ok := msg.(tui.WatcherMsg)
			if !ok {
				continue
			}
			nu, ok := envelope.Inner.(tui.NodeUpdatedMsg)
			if !ok {
				continue
			}
			if nu.Address == "alpha" && nu.Node != nil && len(nu.Node.Tasks) == 1 {
				found = true
				break drainLoop
			}
		default:
			break drainLoop
		}
	}
	if !found {
		t.Error("startWatcher did not trigger eager prefetch; no NodeUpdatedMsg for alpha arrived in the events channel")
	}
}

// TestWiring_InstanceSwitchTriggersEagerPrefetch is the regression
// test for the dead-wiring bug found in the debug log: the
// InstanceSwitchedMsg handler used to inline tui.NewWatcher calls
// without calling EagerPrefetchAndSubscribe, so users who switched
// instances mid-session lost per-leaf cache freshness for the new
// worktree. The fix routes both call sites through newWatcherFor;
// this test exercises the InstanceSwitchedMsg path end-to-end so
// any future regression that re-introduces an inline construction
// is caught immediately.
func TestWiring_InstanceSwitchTriggersEagerPrefetch(t *testing.T) {
	// Set up a target worktree with a real config (so
	// storeFromWolfcastleDir resolves an identity), a real index,
	// and one leaf so the eager prefetch has something to find.
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")

	// Local-tier config provides an identity, which makes
	// storeFromWolfcastleDir return a non-nil store. Without this,
	// the InstanceSwitchedMsg handler silently fails to construct
	// a watcher and the test doesn't actually exercise the code
	// path under test.
	if err := os.MkdirAll(filepath.Join(wcDir, "system", "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgJSON := `{"identity": {"user": "tester", "machine": "box"}}`
	if err := os.WriteFile(filepath.Join(wcDir, "system", "local", "config.json"), []byte(cfgJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stage the index and leaf state under the namespace dir that
	// id.ProjectsDir() will derive from the identity above:
	// <wcDir>/system/projects/<user>-<machine>
	storeDir := filepath.Join(wcDir, "system", "projects", "tester-box")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rootIndex := map[string]any{
		"version": 1,
		"root":    []string{"target"},
		"nodes": map[string]any{
			"target": map[string]any{
				"name":    "target",
				"type":    "leaf",
				"state":   "in_progress",
				"address": "target",
			},
		},
	}
	rootData, _ := json.Marshal(rootIndex)
	if err := os.WriteFile(filepath.Join(storeDir, "state.json"), rootData, 0o644); err != nil {
		t.Fatal(err)
	}
	leafDir := filepath.Join(storeDir, "target")
	if err := os.MkdirAll(leafDir, 0o755); err != nil {
		t.Fatal(err)
	}
	leafState := map[string]any{
		"version": 1,
		"id":      "target",
		"name":    "target",
		"type":    "leaf",
		"state":   "in_progress",
		"tasks": []map[string]any{
			{"id": "task-0001", "title": "switched", "description": "switched", "state": "in_progress", "failure_count": 0},
		},
	}
	leafData, _ := json.Marshal(leafState)
	if err := os.WriteFile(filepath.Join(leafDir, "state.json"), leafData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Build a model that's already in StateLive with a (different)
	// store, then feed an InstanceSwitchedMsg pointing at the
	// staged target. This is exactly what happens when a user
	// presses < or > or a digit key to switch instances.
	m := newColdModel(t)
	m.entryState = StateLive

	// Drain anything left in the events channel from newColdModel
	// setup so we can assert against a fresh state below.
	for {
		select {
		case <-m.watcherEvents:
		default:
			goto drained
		}
	}
drained:

	freshStore := state.NewStore(storeDir, 0)
	freshIdx, err := freshStore.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}
	_ = freshStore // only used for the index read above
	switchMsg := tui.InstanceSwitchedMsg{
		Index: freshIdx,
		Entry: instance.Entry{
			Worktree:  tmp,
			PID:       os.Getpid(),
			Branch:    "main",
			StartedAt: time.Now(),
		},
	}
	result, _ := m.Update(switchMsg)
	m = toModel(t, result)

	// After the handler runs, the events channel should contain a
	// WatcherMsg{NodeUpdatedMsg} for the target leaf because the
	// new watcher's eager prefetch fired during construction.
	found := false
drainSwitchLoop:
	for i := 0; i < 8; i++ {
		select {
		case msg := <-m.watcherEvents:
			envelope, ok := msg.(tui.WatcherMsg)
			if !ok {
				continue
			}
			nu, ok := envelope.Inner.(tui.NodeUpdatedMsg)
			if !ok {
				continue
			}
			if nu.Address == "target" && nu.Node != nil && len(nu.Node.Tasks) == 1 {
				found = true
				break drainSwitchLoop
			}
		default:
			break drainSwitchLoop
		}
	}
	if !found {
		t.Error("InstanceSwitchedMsg handler constructed a watcher without triggering eager prefetch; no NodeUpdatedMsg for target arrived in the events channel. The handler must route through newWatcherFor.")
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
	// staged worktree, drive the production startup sequence (which
	// is what triggers the watcher's tail-load), drain the events
	// channel so the LogLinesMsg reaches the LogViewModel, then
	// switch the detail pane to log view.
	store := state.NewStore(filepath.Join(wcDir, "system", "projects", "default"), 0)
	m := NewTUIModel(store, daemon.NewDaemonRepository(wcDir), tmp, "1.0.0")
	m.entryState = StateLive
	m.width = 120
	m.height = 40
	m.propagateSize()

	if cmd := m.startWatcher(); cmd != nil {
		_ = cmd()
	}
	drainNonBlocking(t, &m)

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

	// Smoke assertion: every row currently marked as a literal search
	// hit must have a name that actually contains the query. The
	// previous bug was a stale flat-list-index map that pointed at
	// unrelated rows after collapse; the address-keyed map can't
	// produce that failure mode by construction, but the assertion
	// stays because it's the most direct expression of correctness.
	literal := m.tree.SearchLiteralAddresses()
	for addr := range literal {
		// The address might point at a task (alpha/beta/task-0001),
		// in which case we look up the task title from the cached
		// node state. For node addresses we look up the index entry.
		var name string
		if cached := m.tree.CachedNode(strings.SplitN(addr, "/", 2)[0]); cached != nil {
			for _, task := range cached.Tasks {
				if addr == "alpha/beta/"+task.ID {
					name = task.Title
				}
			}
		}
		if name == "" {
			if entry, ok := m.tree.Index().Nodes[addr]; ok {
				name = entry.Name
			}
		}
		if name != "" && !strings.Contains(strings.ToLower(name), "frob") {
			t.Errorf("address %q is marked as a literal search hit but its name %q does not contain the query 'frob'", addr, name)
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
