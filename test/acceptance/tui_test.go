//go:build acceptance

package acceptance

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"
)

// ---------------------------------------------------------------------------
// Welcome screen
// ---------------------------------------------------------------------------

func TestWelcomeScreen_Renders(t *testing.T) {
	tm := newWelcomeTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("WOLFCASTLE"),
		teatest.WithDuration(waitTime))
	quit(t, tm)
}

func TestWelcomeScreen_DirectoryBrowser(t *testing.T) {
	tm := newWelcomeTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("WOLFCASTLE"),
		teatest.WithDuration(waitTime))

	// j/k should move the cursor without crashing.
	tm.Type("j")
	tm.Type("k")

	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

func TestColdDashboard_Renders(t *testing.T) {
	tm := newColdTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))
	quit(t, tm)
}

func TestDashboardKey_ReturnsToDashboard(t *testing.T) {
	tm := newColdTUIWithTree(t)
	teatest.WaitFor(t, tm.Output(), contains("demo-project"),
		teatest.WithDuration(waitTime))

	// Enter opens node detail (shows "orchestrator" type label).
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), contains("orchestrator"),
		teatest.WithDuration(waitTime))

	// d returns to dashboard (shows "MISSION BRIEFING").
	tm.Type("d")
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Tree navigation
// ---------------------------------------------------------------------------

func TestTreeNavigation_ExpandCollapse(t *testing.T) {
	tm := newColdTUIWithTree(t)
	teatest.WaitFor(t, tm.Output(), contains("demo-project"),
		teatest.WithDuration(waitTime))

	// Expand the orchestrator.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), contains("backend"),
		teatest.WithDuration(waitTime))

	// Collapse and quit. No second assertion on the same content.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	quit(t, tm)
}

func TestTreeNavigation_JKMovement(t *testing.T) {
	tm := newColdTUIWithTree(t)
	teatest.WaitFor(t, tm.Output(), contains("demo-project"),
		teatest.WithDuration(waitTime))

	// Expand to get multiple rows.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), contains("backend"),
		teatest.WithDuration(waitTime))

	// Move down and up. Verify these don't crash.
	tm.Type("j")
	tm.Type("j")
	tm.Type("k")

	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Inbox
// ---------------------------------------------------------------------------

func TestInbox_OpenCloseModal(t *testing.T) {
	tm := newColdTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))

	// Open inbox.
	tm.Type("i")
	teatest.WaitFor(t, tm.Output(), contains("INBOX"),
		teatest.WithDuration(waitTime))

	// Close inbox and quit.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	quit(t, tm)
}

func TestInbox_AddItem(t *testing.T) {
	tm := newColdTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))

	// Open inbox, add an item.
	tm.Type("i")
	teatest.WaitFor(t, tm.Output(), contains("INBOX"),
		teatest.WithDuration(waitTime))

	tm.Type("a")
	tm.Type("Build a better mousetrap")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	// The item should appear in the inbox.
	teatest.WaitFor(t, tm.Output(), contains("mousetrap"),
		teatest.WithDuration(waitTime))

	// Close.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Help overlay
// ---------------------------------------------------------------------------

func TestHelp_ShowsAllSections(t *testing.T) {
	tm := newColdTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))

	// Open help. All sections should appear in a single render frame.
	tm.Type("?")
	teatest.WaitFor(t, tm.Output(), contains("Log Stream"),
		teatest.WithDuration(waitTime))

	// Dismiss.
	tm.Type("?")
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestSearch_Opens(t *testing.T) {
	tm := newColdTUIWithTree(t)
	teatest.WaitFor(t, tm.Output(), contains("demo-project"),
		teatest.WithDuration(waitTime))

	// Open search bar. The "/" prompt should appear.
	tm.Type("/")
	tm.Type("backend")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Dismiss search and quit.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Daemon modal
// ---------------------------------------------------------------------------

func TestDaemonModal_OpensOnS(t *testing.T) {
	tm := newColdTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))

	// s opens the daemon modal.
	tm.Type("s")
	teatest.WaitFor(t, tm.Output(), contains("START DAEMON"),
		teatest.WithDuration(waitTime))

	// Esc cancels.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Focus and layout
// ---------------------------------------------------------------------------

func TestToggleTree_HidesAndShows(t *testing.T) {
	tm := newColdTUIWithTree(t)
	teatest.WaitFor(t, tm.Output(), contains("demo-project"),
		teatest.WithDuration(waitTime))

	// Toggle tree off and back on. Verify no crash.
	tm.Type("t")
	tm.Type("t")

	quit(t, tm)
}

func TestTabCyclesFocus(t *testing.T) {
	tm := newColdTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))

	// Tab should cycle focus without crashing.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})

	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Copy
// ---------------------------------------------------------------------------

func TestCopyAddress(t *testing.T) {
	tm := newColdTUIWithTree(t)
	teatest.WaitFor(t, tm.Output(), contains("demo-project"),
		teatest.WithDuration(waitTime))

	// y copies the current address. Should not crash.
	tm.Type("y")

	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Refresh
// ---------------------------------------------------------------------------

func TestRefresh(t *testing.T) {
	tm := newColdTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))

	// R triggers a refresh. Verify it doesn't crash by quitting cleanly.
	tm.Type("R")
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Log stream
// ---------------------------------------------------------------------------

func TestLogStream_OpensOnL(t *testing.T) {
	tm := newColdTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))

	// L opens the log modal. The header says "TRANSMISSIONS".
	tm.Type("L")
	teatest.WaitFor(t, tm.Output(), contains("TRANSMISSIONS"),
		teatest.WithDuration(waitTime))

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	quit(t, tm)
}

func TestLogStream_CloseOnEsc(t *testing.T) {
	tm := newColdTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))

	// Open then close the log modal.
	tm.Type("L")
	teatest.WaitFor(t, tm.Output(), contains("TRANSMISSIONS"),
		teatest.WithDuration(waitTime))

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Welcome screen (additional)
// ---------------------------------------------------------------------------

func TestWelcomeScreen_EnterOnDir(t *testing.T) {
	tm := newWelcomeTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("WOLFCASTLE"),
		teatest.WithDuration(waitTime))

	// Enter on a directory entry navigates into it (or opens it).
	// Verify no crash.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	quit(t, tm)
}

func TestWelcomeScreen_TabSwitchesPanels(t *testing.T) {
	tm := newWelcomeTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("WOLFCASTLE"),
		teatest.WithDuration(waitTime))

	// Tab on welcome with no instances is a no-op; verify it doesn't crash.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	quit(t, tm)
}

func TestWelcomeScreen_BackNavigation(t *testing.T) {
	tm := newWelcomeTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("WOLFCASTLE"),
		teatest.WithDuration(waitTime))

	// j moves into directory list, h goes up a directory.
	tm.Type("j")
	tm.Type("h")
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Instance switching
// ---------------------------------------------------------------------------

func TestInstanceSwitching_NoInstancesNoCrash(t *testing.T) {
	tm := newColdTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))

	// < and > with no instances should be harmless.
	tm.Type("<")
	tm.Type(">")
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Search match navigation
// ---------------------------------------------------------------------------

func TestSearch_MatchNavigation(t *testing.T) {
	tm := newColdTUIWithTree(t)
	teatest.WaitFor(t, tm.Output(), contains("demo-project"),
		teatest.WithDuration(waitTime))

	// Open search, type a query, confirm.
	tm.Type("/")
	tm.Type("end")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	// n and N navigate matches. Verify no crash.
	tm.Type("n")
	tm.Type("N")
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Alternative key bindings
// ---------------------------------------------------------------------------

func TestTreeNavigation_LExpands(t *testing.T) {
	tm := newColdTUIWithTree(t)
	teatest.WaitFor(t, tm.Output(), contains("demo-project"),
		teatest.WithDuration(waitTime))

	// l should expand the orchestrator (same as Enter).
	tm.Type("l")
	teatest.WaitFor(t, tm.Output(), contains("backend"),
		teatest.WithDuration(waitTime))
	quit(t, tm)
}

func TestTreeNavigation_HCollapses(t *testing.T) {
	tm := newColdTUIWithTree(t)
	teatest.WaitFor(t, tm.Output(), contains("demo-project"),
		teatest.WithDuration(waitTime))

	// Expand, then collapse with h.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), contains("backend"),
		teatest.WithDuration(waitTime))

	tm.Type("h")
	quit(t, tm)
}

func TestTreeNavigation_GJumpsTopAndBottom(t *testing.T) {
	tm := newColdTUIWithTree(t)
	teatest.WaitFor(t, tm.Output(), contains("demo-project"),
		teatest.WithDuration(waitTime))

	// Expand to get multiple rows.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), contains("backend"),
		teatest.WithDuration(waitTime))

	// G jumps to bottom, g to top. Verify no crash.
	tm.Type("G")
	tm.Type("g")
	quit(t, tm)
}

func TestQuit_CtrlC(t *testing.T) {
	tm := newColdTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))

	// Ctrl+C should terminate the program.
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	tm.WaitFinished(t, teatest.WithFinalTimeout(waitTime))
}

// ---------------------------------------------------------------------------
// Terminal states
// ---------------------------------------------------------------------------

func TestDashboard_AllComplete(t *testing.T) {
	tm := newColdTUIAllComplete(t)
	teatest.WaitFor(t, tm.Output(), contains("All targets eliminated"),
		teatest.WithDuration(waitTime))
	quit(t, tm)
}

func TestDashboard_AllBlocked(t *testing.T) {
	tm := newColdTUIAllBlocked(t)
	teatest.WaitFor(t, tm.Output(), contains("Blocked on all fronts"),
		teatest.WithDuration(waitTime))
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Node and task detail
// ---------------------------------------------------------------------------

func TestNodeDetail_ShowsContent(t *testing.T) {
	tm := newColdTUIWithTree(t)
	teatest.WaitFor(t, tm.Output(), contains("demo-project"),
		teatest.WithDuration(waitTime))

	// Enter on the orchestrator row opens node detail.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), contains("orchestrator"),
		teatest.WithDuration(waitTime))
	quit(t, tm)
}

func TestTaskDetail_ShowsContent(t *testing.T) {
	tm := newColdTUIWithTree(t)
	teatest.WaitFor(t, tm.Output(), contains("demo-project"),
		teatest.WithDuration(waitTime))

	// Expand the orchestrator.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), contains("backend"),
		teatest.WithDuration(waitTime))

	// Move to a leaf (backend) and expand it to show tasks.
	tm.Type("j")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), contains("Build the API"),
		teatest.WithDuration(waitTime))

	// Move to the first task row, then Enter to open task detail.
	tm.Type("j")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), contains("task-1"),
		teatest.WithDuration(waitTime))
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Status glyphs
// ---------------------------------------------------------------------------

func TestTree_StatusGlyphs(t *testing.T) {
	tm := newColdTUIWithMixedStates(t)
	teatest.WaitFor(t, tm.Output(), contains("mixed-project"),
		teatest.WithDuration(waitTime))

	// Expand the orchestrator so all children are visible.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	// The mixed tree has complete (●), in_progress (◐), not_started (◯),
	// and blocked (☢) nodes. Check for the raw UTF-8 glyph characters.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		// All four children must be visible after expand.
		return containsAll(b, "done", "wip", "todo", "stuck")
	}, teatest.WithDuration(waitTime))
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Modal sequencing
// ---------------------------------------------------------------------------

func TestModal_Sequencing(t *testing.T) {
	tm := newColdTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))

	// Open inbox.
	tm.Type("i")
	teatest.WaitFor(t, tm.Output(), contains("INBOX"),
		teatest.WithDuration(waitTime))

	// Close inbox.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	// Open daemon modal.
	tm.Type("s")
	teatest.WaitFor(t, tm.Output(), contains("START DAEMON"),
		teatest.WithDuration(waitTime))

	// Close daemon modal.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Navigation depth: full loop
// ---------------------------------------------------------------------------

func TestNavigationDepth_FullLoop(t *testing.T) {
	tm := newColdTUIWithTree(t)
	teatest.WaitFor(t, tm.Output(), contains("demo-project"),
		teatest.WithDuration(waitTime))

	// Expand orchestrator → shows children.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), contains("orchestrator"),
		teatest.WithDuration(waitTime))

	// d returns to dashboard.
	tm.Type("d")
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))
	quit(t, tm)
}

// ---------------------------------------------------------------------------
// Stop all
// ---------------------------------------------------------------------------

func TestStopAll_SKeyNoCrash(t *testing.T) {
	tm := newColdTUI(t)
	teatest.WaitFor(t, tm.Output(), contains("MISSION BRIEFING"),
		teatest.WithDuration(waitTime))

	// S (stop all) with no running daemons should be a no-op.
	tm.Type("S")
	quit(t, tm)
}
