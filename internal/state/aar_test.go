package state

import (
	"strings"
	"testing"
	"time"
)

func TestAddAAR_CreatesMap(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("test", "Test", NodeLeaf)
	if ns.AARs != nil {
		t.Fatal("AARs should be nil initially")
	}

	aar := AAR{
		TaskID:       "task-0001",
		Timestamp:    time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		Objective:    "Build auth module",
		WhatHappened: "Created JWT middleware",
	}
	AddAAR(ns, aar)

	if ns.AARs == nil {
		t.Fatal("AARs map should be initialized")
	}
	if len(ns.AARs) != 1 {
		t.Fatalf("expected 1 AAR, got %d", len(ns.AARs))
	}
	got := ns.AARs["task-0001"]
	if got.Objective != "Build auth module" {
		t.Errorf("expected objective 'Build auth module', got %q", got.Objective)
	}
}

func TestAddAAR_OverwritesSameTaskID(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("test", "Test", NodeLeaf)

	first := AAR{
		TaskID:       "task-0001",
		Timestamp:    time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		Objective:    "First attempt",
		WhatHappened: "Failed",
	}
	AddAAR(ns, first)

	second := AAR{
		TaskID:       "task-0001",
		Timestamp:    time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
		Objective:    "Second attempt",
		WhatHappened: "Succeeded",
	}
	AddAAR(ns, second)

	if len(ns.AARs) != 1 {
		t.Fatalf("expected 1 AAR after overwrite, got %d", len(ns.AARs))
	}
	if ns.AARs["task-0001"].Objective != "Second attempt" {
		t.Error("AAR should be overwritten")
	}
}

func TestAddAAR_MultipleTaskIDs(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("test", "Test", NodeLeaf)

	AddAAR(ns, AAR{TaskID: "task-0001", Objective: "First"})
	AddAAR(ns, AAR{TaskID: "task-0002", Objective: "Second"})
	AddAAR(ns, AAR{TaskID: "task-0003", Objective: "Third"})

	if len(ns.AARs) != 3 {
		t.Fatalf("expected 3 AARs, got %d", len(ns.AARs))
	}
}

func TestRenderAARs_EmptyMap(t *testing.T) {
	t.Parallel()
	if RenderAARs(nil) != "" {
		t.Error("nil AARs should render empty")
	}
	if RenderAARs(map[string]AAR{}) != "" {
		t.Error("empty AARs should render empty")
	}
}

func TestRenderAARs_SingleAAR(t *testing.T) {
	t.Parallel()
	aars := map[string]AAR{
		"task-0001": {
			TaskID:       "task-0001",
			Timestamp:    time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
			Objective:    "Implement JWT validation",
			WhatHappened: "Added RS256 middleware",
			WentWell:     []string{"Clean separation", "Good test coverage"},
			Improvements: []string{"Error messages need work"},
			ActionItems:  []string{"Add token refresh endpoint"},
		},
	}

	result := RenderAARs(aars)

	expected := []string{
		"## Prior Task Reviews (AARs)",
		"### task-0001",
		"**Objective:** Implement JWT validation",
		"**What happened:** Added RS256 middleware",
		"**Went well:**",
		"- Clean separation",
		"- Good test coverage",
		"**Improvements:**",
		"- Error messages need work",
		"**Action items:**",
		"- Add token refresh endpoint",
	}
	for _, s := range expected {
		if !strings.Contains(result, s) {
			t.Errorf("expected output to contain %q", s)
		}
	}
}

func TestRenderAARs_ChronologicalOrder(t *testing.T) {
	t.Parallel()
	aars := map[string]AAR{
		"task-0003": {
			TaskID:       "task-0003",
			Timestamp:    time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
			Objective:    "Third",
			WhatHappened: "Third happened",
		},
		"task-0001": {
			TaskID:       "task-0001",
			Timestamp:    time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
			Objective:    "First",
			WhatHappened: "First happened",
		},
		"task-0002": {
			TaskID:       "task-0002",
			Timestamp:    time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
			Objective:    "Second",
			WhatHappened: "Second happened",
		},
	}

	result := RenderAARs(aars)

	firstIdx := strings.Index(result, "### task-0001")
	secondIdx := strings.Index(result, "### task-0002")
	thirdIdx := strings.Index(result, "### task-0003")

	if firstIdx >= secondIdx || secondIdx >= thirdIdx {
		t.Error("AARs should be rendered in chronological order")
	}
}

func TestRenderAARs_OmitsEmptyOptionalFields(t *testing.T) {
	t.Parallel()
	aars := map[string]AAR{
		"task-0001": {
			TaskID:       "task-0001",
			Timestamp:    time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
			Objective:    "Minimal AAR",
			WhatHappened: "Did the thing",
		},
	}

	result := RenderAARs(aars)

	if strings.Contains(result, "**Went well:**") {
		t.Error("should omit went well section when empty")
	}
	if strings.Contains(result, "**Improvements:**") {
		t.Error("should omit improvements section when empty")
	}
	if strings.Contains(result, "**Action items:**") {
		t.Error("should omit action items section when empty")
	}
}
