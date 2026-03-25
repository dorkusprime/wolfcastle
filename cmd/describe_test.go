package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
	"github.com/spf13/cobra"
)

type describeTestEnv struct {
	WolfcastleDir string
	ProjectsDir   string
	App           *cmdutil.App
	RootCmd       *cobra.Command
	env           *testutil.Environment
}

func newDescribeTestEnv(t *testing.T) *describeTestEnv {
	t.Helper()

	env := testutil.NewEnvironment(t)
	af := env.ToAppFields()

	testApp := &cmdutil.App{
		Config:   af.Config,
		Identity: af.Identity,
		State:    af.State,
		Prompts:  af.Prompts,
		Classes:  af.Classes,
		Daemon:   af.Daemon,
		Git:      af.Git,
		Clock:    clock.New(),
	}

	root := &cobra.Command{Use: "wolfcastle"}
	root.PersistentFlags().BoolVar(&testApp.JSON, "json", false, "Output in JSON format")
	root.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "Lifecycle:"},
		&cobra.Group{ID: "work", Title: "Work Management:"},
		&cobra.Group{ID: "audit", Title: "Auditing:"},
		&cobra.Group{ID: "docs", Title: "Documentation:"},
		&cobra.Group{ID: "diagnostics", Title: "Diagnostics:"},
		&cobra.Group{ID: "integration", Title: "Integration:"},
	)

	// Register the describe command on the test root.
	descCmd := &cobra.Command{
		Use:   "describe [address]",
		Short: "Show everything about a node",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := testApp.RequireIdentity(); err != nil {
				return err
			}
			addr, err := resolveDescribeAddress(cmd, args)
			if err != nil {
				return err
			}
			idx, err := testApp.State.ReadIndex()
			if err != nil {
				return err
			}
			entry, ok := idx.Nodes[addr]
			if !ok {
				return fmt.Errorf("node %q not found in index", addr)
			}
			ns, err := testApp.State.ReadNode(addr)
			if err != nil {
				return err
			}
			if testApp.JSON {
				return describeJSON(testApp, ns, entry, addr)
			}
			return describeHuman(testApp, ns, entry, addr)
		},
	}
	descCmd.Flags().String("node", "", "Node address")
	descCmd.GroupID = "work"
	root.AddCommand(descCmd)

	return &describeTestEnv{
		WolfcastleDir: env.Root,
		ProjectsDir:   env.ProjectsDir(),
		App:           testApp,
		RootCmd:       root,
		env:           env,
	}
}

func (e *describeTestEnv) loadNodeState(t *testing.T, addr string) *state.NodeState {
	t.Helper()
	ns, err := e.env.State.ReadNode(addr)
	if err != nil {
		t.Fatalf("loading node state for %s: %v", addr, err)
	}
	return ns
}

func (e *describeTestEnv) populateLeaf(t *testing.T, addr string) {
	t.Helper()
	e.env.WithProject("Test Project", testutil.Leaf(addr))

	now := time.Now()
	err := e.env.State.MutateNode(addr, func(ns *state.NodeState) error {
		ns.State = state.StatusInProgress
		ns.Scope = "Health check handler implementation"
		ns.Tasks = []state.Task{
			{
				ID:           "task-0001",
				Title:        "Create handler",
				Description:  "Create internal/health/handler.go",
				State:        state.StatusComplete,
				Deliverables: []string{"internal/health/handler.go", "internal/health/handler_test.go"},
				Class:        "coding/go",
				References:   []string{"docs/decisions/ADR-001.md"},
			},
			{
				ID:          "audit",
				Description: "Verify all work in health",
				State:       state.StatusInProgress,
				IsAudit:     true,
			},
		}
		ns.Specs = []string{"2026-03-23T10-53Z-parallel-sibling-execution.md"}
		ns.Audit = state.AuditState{
			Status: state.AuditInProgress,
			Scope: &state.AuditScope{
				Description: "Health check handler implementation",
			},
			Breadcrumbs: []state.Breadcrumb{
				{Timestamp: now, Task: "task-0001", Text: "Created handler.go and handler_test.go"},
			},
			Gaps: []state.Gap{
				{ID: "gap-001", Description: "Missing error handling", Status: state.GapFixed},
				{ID: "gap-002", Description: "No timeout config", Status: state.GapFixed},
			},
			Escalations: []state.Escalation{},
		}
		ns.AARs = map[string]state.AAR{
			"task-0001": {
				TaskID:    "task-0001",
				Timestamp: now,
				Objective: "Implement health check endpoint",
			},
		}
		return nil
	})
	if err != nil {
		t.Fatalf("populating leaf node: %v", err)
	}
}

func (e *describeTestEnv) populateOrchestrator(t *testing.T) {
	t.Helper()
	e.env.WithProject("Test Project", testutil.Orchestrator("my-project",
		testutil.Leaf("child-a"),
		testutil.Leaf("child-b"),
	))

	now := time.Now()
	err := e.env.State.MutateNode("my-project", func(ns *state.NodeState) error {
		ns.State = state.StatusInProgress
		ns.Scope = "Build the API layer"
		ns.TotalReplans = 2
		ns.SuccessCriteria = []string{"All endpoints respond", "90% test coverage"}
		ns.PlanningHistory = []state.PlanningPass{
			{Timestamp: now, Trigger: "initial", Summary: "Created child-a and child-b"},
			{Timestamp: now, Trigger: "replan", Summary: "Added error handling scope"},
		}
		return nil
	})
	if err != nil {
		t.Fatalf("populating orchestrator: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestDescribe_LeafNode(t *testing.T) {
	env := newDescribeTestEnv(t)
	env.populateLeaf(t, "my-project")

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"describe", "my-project"})
	err := env.RootCmd.Execute()

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("describe failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	// Verify key sections appear
	checks := []string{
		"my-project (leaf, in_progress)",
		"Tasks:",
		"task-0001",
		"Create handler",
		"deliverables: internal/health/handler.go",
		"class: coding/go",
		"Audit:",
		"status: in_progress",
		"gaps: 0 open, 2 fixed",
		"Breadcrumbs:",
		"task-0001: Created handler.go",
		"Specs:",
		"2026-03-23T10-53Z-parallel-sibling-execution.md",
		"AARs:",
		"Implement health check endpoint",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}

	// Planning section should NOT appear for leaf nodes
	if strings.Contains(out, "Planning:") {
		t.Error("leaf node should not show Planning section")
	}
}

func TestDescribe_OrchestratorNode(t *testing.T) {
	env := newDescribeTestEnv(t)
	env.populateOrchestrator(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"describe", "my-project"})
	err := env.RootCmd.Execute()

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("describe failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	checks := []string{
		"my-project (project, in_progress)",
		"Children:",
		"child-a",
		"child-b",
		"Planning:",
		"children: 2, replans: 2",
		"initial: Created child-a and child-b",
		"success criteria:",
		"All endpoints respond",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestDescribe_MinimalNode(t *testing.T) {
	env := newDescribeTestEnv(t)
	env.env.WithProject("Minimal", testutil.Leaf("minimal"))

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"describe", "minimal"})
	err := env.RootCmd.Execute()

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("describe failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	// Minimal node should still show Audit section
	if !strings.Contains(out, "Audit:") {
		t.Errorf("expected Audit section, got:\n%s", out)
	}

	// Should NOT show Specs, AARs, Breadcrumbs, or Planning sections
	for _, absent := range []string{"Specs:", "AARs:", "Breadcrumbs:", "Planning:"} {
		if strings.Contains(out, absent) {
			t.Errorf("minimal node should not show %q section", absent)
		}
	}
}

func TestDescribe_JSON(t *testing.T) {
	env := newDescribeTestEnv(t)
	env.populateLeaf(t, "my-project")

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"describe", "my-project", "--json"})
	err := env.RootCmd.Execute()

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("describe --json failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, buf.String())
	}

	if resp["ok"] != true {
		t.Errorf("expected ok=true, got %v", resp["ok"])
	}
	if resp["action"] != "describe" {
		t.Errorf("expected action=describe, got %v", resp["action"])
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("data is not a map: %T", resp["data"])
	}

	if _, ok := data["node_state"]; !ok {
		t.Error("JSON data missing node_state")
	}
	if _, ok := data["index_entry"]; !ok {
		t.Error("JSON data missing index_entry")
	}
}

func TestDescribe_NonexistentNode(t *testing.T) {
	env := newDescribeTestEnv(t)

	env.RootCmd.SetArgs([]string{"describe", "does-not-exist"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestDescribe_NodeFlag(t *testing.T) {
	env := newDescribeTestEnv(t)
	env.populateLeaf(t, "my-project")

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"describe", "--node", "my-project"})
	err := env.RootCmd.Execute()

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("describe --node failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "my-project (leaf, in_progress)") {
		t.Errorf("expected node header in output, got:\n%s", out)
	}
}

func TestDescribe_BothPositionalAndFlag(t *testing.T) {
	env := newDescribeTestEnv(t)

	env.RootCmd.SetArgs([]string{"describe", "my-project", "--node", "my-project"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when both positional and --node are provided")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Errorf("expected 'not both' in error, got: %v", err)
	}
}

func TestDescribe_NoAddress(t *testing.T) {
	env := newDescribeTestEnv(t)

	env.RootCmd.SetArgs([]string{"describe"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no address is provided")
	}
	if !strings.Contains(err.Error(), "node address required") {
		t.Errorf("expected 'node address required' in error, got: %v", err)
	}
}

func TestDescribe_ShellCompletion(t *testing.T) {
	env := newDescribeTestEnv(t)
	env.env.WithProject("Completion Test", testutil.Leaf("comp-project"))

	// The ValidArgsFunction on the real describeCmd uses app which isn't
	// wired in these tests. Instead, verify the resolveDescribeAddress
	// function handles all cases correctly (tested above) and that
	// CompleteNodeAddresses returns addresses.
	addrs, directive := cmdutil.CompleteNodeAddresses(env.App)(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}
	found := false
	for _, a := range addrs {
		if a == "comp-project" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected comp-project in completions, got: %v", addrs)
	}
}

func TestDescribe_WithDescriptionMD(t *testing.T) {
	env := newDescribeTestEnv(t)
	env.env.WithProject("MD Test", testutil.Leaf("md-project"))

	// Write a description.md file into the node directory.
	nodePath, err := env.App.State.NodePath("md-project")
	if err != nil {
		t.Fatalf("getting node path: %v", err)
	}
	descPath := filepath.Join(filepath.Dir(nodePath), "description.md")
	if err := os.WriteFile(descPath, []byte("# Health Check\nReturns 200 OK."), 0644); err != nil {
		t.Fatalf("writing description.md: %v", err)
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.RootCmd.SetArgs([]string{"describe", "md-project", "--json"})
	err = env.RootCmd.Execute()

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("describe failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	data := resp["data"].(map[string]any)
	desc, ok := data["description_md"]
	if !ok {
		t.Error("JSON data missing description_md when file exists")
	}
	if !strings.Contains(desc.(string), "Health Check") {
		t.Errorf("description_md content wrong: %v", desc)
	}
}
