package pipeline_test

import (
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

func TestContextBuilder_IncludesAARs(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0002", Description: "Second task", State: state.StatusInProgress},
		},
		AARs: map[string]state.AAR{
			"task-0001": {
				TaskID:       "task-0001",
				Timestamp:    time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
				Objective:    "Build the widget",
				WhatHappened: "Widget built and tested",
				WentWell:     []string{"Clean API design"},
				Improvements: []string{"Needs better error messages"},
				ActionItems:  []string{"Add retry logic"},
			},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, _ := cb.Build("proj", "", ns, "task-0002", "", nil)

	if !strings.Contains(got, "## Prior Task Reviews (AARs)") {
		t.Error("missing AARs section")
	}
	if !strings.Contains(got, "### task-0001") {
		t.Error("missing AAR task header")
	}
	if !strings.Contains(got, "**Objective:** Build the widget") {
		t.Error("missing AAR objective")
	}
	if !strings.Contains(got, "**What happened:** Widget built and tested") {
		t.Error("missing AAR what happened")
	}
	if !strings.Contains(got, "- Clean API design") {
		t.Error("missing went well item")
	}
	if !strings.Contains(got, "- Needs better error messages") {
		t.Error("missing improvements item")
	}
	if !strings.Contains(got, "- Add retry logic") {
		t.Error("missing action items")
	}
}

func TestContextBuilder_OmitsAARsWhenEmpty(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "First task", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, _ := cb.Build("proj", "", ns, "task-0001", "", nil)

	if strings.Contains(got, "## Prior Task Reviews") {
		t.Error("AARs section should be absent when no AARs exist")
	}
}

func TestContextBuilder_AARsBeforeAuditContext(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0002", Description: "Work", State: state.StatusInProgress},
		},
		AARs: map[string]state.AAR{
			"task-0001": {
				TaskID:       "task-0001",
				Timestamp:    now,
				Objective:    "Prior work",
				WhatHappened: "Done",
			},
		},
		Audit: state.AuditState{
			Breadcrumbs: []state.Breadcrumb{
				{Timestamp: now, Task: "task-0001", Text: "breadcrumb"},
			},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, _ := cb.Build("proj", "", ns, "task-0002", "", nil)

	aarIdx := strings.Index(got, "## Prior Task Reviews (AARs)")
	bcIdx := strings.Index(got, "## Recent Breadcrumbs")

	if aarIdx < 0 {
		t.Fatal("missing AARs section")
	}
	if bcIdx < 0 {
		t.Fatal("missing breadcrumbs section")
	}
	if aarIdx >= bcIdx {
		t.Error("AARs should appear before audit breadcrumbs")
	}
}
