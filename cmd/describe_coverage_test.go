package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

func TestReadNodeDescription_DescriptionMdExists(t *testing.T) {
	t.Parallel()
	env := newDescribeTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	// Write description.md in the node directory.
	nodePath := filepath.Join(env.env.ProjectsDir(), "proj")
	_ = os.WriteFile(filepath.Join(nodePath, "description.md"), []byte("# Project Description"), 0644)

	got := readNodeDescription(env.App, "proj")
	if got != "# Project Description" {
		t.Errorf("expected description content, got %q", got)
	}
}

func TestReadNodeDescription_FallbackToOtherMd(t *testing.T) {
	t.Parallel()
	env := newDescribeTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	// Write a non-description .md file (no description.md).
	nodePath := filepath.Join(env.env.ProjectsDir(), "proj")
	_ = os.WriteFile(filepath.Join(nodePath, "overview.md"), []byte("# Overview"), 0644)

	got := readNodeDescription(env.App, "proj")
	if got != "# Overview" {
		t.Errorf("expected fallback md content, got %q", got)
	}
}

func TestReadNodeDescription_IgnoresAuditMd(t *testing.T) {
	t.Parallel()
	env := newDescribeTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	// Write only an audit-report .md file which should be skipped.
	nodePath := filepath.Join(env.env.ProjectsDir(), "proj")
	_ = os.WriteFile(filepath.Join(nodePath, "audit-report.md"), []byte("# Audit"), 0644)

	got := readNodeDescription(env.App, "proj")
	if got != "" {
		t.Errorf("expected empty string when only audit .md exists, got %q", got)
	}
}

func TestReadNodeDescription_NoMarkdownFiles(t *testing.T) {
	t.Parallel()
	env := newDescribeTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	got := readNodeDescription(env.App, "proj")
	if got != "" {
		t.Errorf("expected empty string when no .md files exist, got %q", got)
	}
}

func TestReadNodeDescription_InvalidAddress(t *testing.T) {
	t.Parallel()
	env := newDescribeTestEnv(t)

	got := readNodeDescription(env.App, "nonexistent-node")
	if got != "" {
		t.Errorf("expected empty string for invalid address, got %q", got)
	}
}
