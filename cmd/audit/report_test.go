package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestReportCmd_ExistingReportFile(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)
	env.createLeafNode(t, "rpt-node", "Report Node")

	// Write an audit report file
	nodeDir := filepath.Join(env.ProjectsDir, "rpt-node")
	reportContent := "# Audit Report\nAll good."
	if err := os.WriteFile(filepath.Join(nodeDir, "audit-20260101T000000.md"), []byte(reportContent), 0644); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"audit", "report", "--node", "rpt-node"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("report with existing file: %v", err)
	}
}

func TestReportCmd_ExistingReportFile_JSON(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)
	env.createLeafNode(t, "rpt-json", "Report JSON")

	nodeDir := filepath.Join(env.ProjectsDir, "rpt-json")
	if err := os.WriteFile(filepath.Join(nodeDir, "audit-20260101T000000.md"), []byte("report"), 0644); err != nil {
		t.Fatal(err)
	}

	env.App.JSON = true
	env.RootCmd.SetArgs([]string{"audit", "report", "--node", "rpt-json"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("report JSON with file: %v", err)
	}
}

func TestReportCmd_NoReport_FallsBackToPreview(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)
	env.createLeafNode(t, "preview-node", "Preview Node")

	env.RootCmd.SetArgs([]string{"audit", "report", "--node", "preview-node"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("report preview fallback: %v", err)
	}
}

func TestReportCmd_NoReport_FallsBackToPreview_JSON(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)
	env.createLeafNode(t, "prev-json", "Preview JSON")

	env.App.JSON = true
	env.RootCmd.SetArgs([]string{"audit", "report", "--node", "prev-json"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("report preview JSON: %v", err)
	}
}

func TestReportCmd_PathFlag_WithReport(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)
	env.createLeafNode(t, "path-node", "Path Node")

	nodeDir := filepath.Join(env.ProjectsDir, "path-node")
	if err := os.WriteFile(filepath.Join(nodeDir, "audit-20260101T000000.md"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"audit", "report", "--node", "path-node", "--path"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("report --path with file: %v", err)
	}
}

func TestReportCmd_PathFlag_WithReport_JSON(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)
	env.createLeafNode(t, "path-json", "Path JSON")

	nodeDir := filepath.Join(env.ProjectsDir, "path-json")
	if err := os.WriteFile(filepath.Join(nodeDir, "audit-20260101T000000.md"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	env.App.JSON = true
	env.RootCmd.SetArgs([]string{"audit", "report", "--node", "path-json", "--path"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("report --path JSON with file: %v", err)
	}
}

func TestReportCmd_PathFlag_NoReport(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)
	env.createLeafNode(t, "no-rpt", "No Report")

	env.RootCmd.SetArgs([]string{"audit", "report", "--node", "no-rpt", "--path"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("report --path no file: %v", err)
	}
}

func TestReportCmd_PathFlag_NoReport_JSON(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)
	env.createLeafNode(t, "no-rpt-j", "No Report JSON")

	env.App.JSON = true
	env.RootCmd.SetArgs([]string{"audit", "report", "--node", "no-rpt-j", "--path"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("report --path JSON no file: %v", err)
	}
}

func TestReportCmd_NodeNotFound(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "report", "--node", "nonexistent"})
	if err := env.RootCmd.Execute(); err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestReportCmd_MissingNodeFlag(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "report"})
	if err := env.RootCmd.Execute(); err == nil {
		t.Error("expected error when --node is missing")
	}
}

func TestReportCmd_NoIdentity(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)
	env.App.Identity = nil
	env.App.State = nil

	env.RootCmd.SetArgs([]string{"audit", "report", "--node", "anything"})
	if err := env.RootCmd.Execute(); err == nil {
		t.Error("expected error when identity not configured")
	}
}

func TestReportCmd_ReportFileReadError(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)
	env.createLeafNode(t, "read-err", "Read Error")

	// Create a report file that is actually a directory to cause ReadFile to fail
	nodeDir := filepath.Join(env.ProjectsDir, "read-err")
	if err := os.MkdirAll(filepath.Join(nodeDir, "audit-20260101T000000.md"), 0755); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"audit", "report", "--node", "read-err"})
	if err := env.RootCmd.Execute(); err == nil {
		t.Error("expected error when report file cannot be read")
	}
}

func TestReportCmd_NodeStateLoadError(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	// Create index entry but corrupt the state file
	nodeDir := filepath.Join(env.ProjectsDir, "corrupt")
	_ = os.MkdirAll(nodeDir, 0755)

	idx, _ := env.App.State.ReadIndex()
	idx.Nodes["corrupt"] = state.IndexEntry{
		Name: "Corrupt", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "corrupt",
	}
	idx.Root = append(idx.Root, "corrupt")
	saveJSON(t, filepath.Join(env.ProjectsDir, "state.json"), idx)

	// Write invalid JSON as state
	if err := os.WriteFile(filepath.Join(nodeDir, "state.json"), []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"audit", "report", "--node", "corrupt"})
	if err := env.RootCmd.Execute(); err == nil {
		t.Error("expected error when node state cannot be loaded")
	}
}
