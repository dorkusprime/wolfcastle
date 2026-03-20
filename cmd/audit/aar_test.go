package audit

import (
	"testing"
)

func TestAAR_Success(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{
		"audit", "aar",
		"--node", "my-project",
		"--task", "task-0001",
		"--objective", "Build auth module",
		"--what-happened", "Implemented JWT validation",
		"--went-well", "Clean separation of concerns",
		"--went-well", "Good test coverage",
		"--improvements", "Error messages need work",
		"--action-items", "Add token refresh endpoint",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("aar failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	if len(ns.AARs) != 1 {
		t.Fatalf("expected 1 AAR, got %d", len(ns.AARs))
	}
	aar, ok := ns.AARs["task-0001"]
	if !ok {
		t.Fatal("AAR for task-0001 not found")
	}
	if aar.Objective != "Build auth module" {
		t.Errorf("expected objective 'Build auth module', got %q", aar.Objective)
	}
	if aar.WhatHappened != "Implemented JWT validation" {
		t.Errorf("unexpected what happened: %q", aar.WhatHappened)
	}
	if len(aar.WentWell) != 2 {
		t.Errorf("expected 2 went well items, got %d", len(aar.WentWell))
	}
	if len(aar.Improvements) != 1 {
		t.Errorf("expected 1 improvement, got %d", len(aar.Improvements))
	}
	if len(aar.ActionItems) != 1 {
		t.Errorf("expected 1 action item, got %d", len(aar.ActionItems))
	}
}

func TestAAR_MinimalFields(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{
		"audit", "aar",
		"--node", "my-project",
		"--task", "task-0001",
		"--objective", "Do the thing",
		"--what-happened", "Did the thing",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("aar failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	aar := ns.AARs["task-0001"]
	if aar.WentWell != nil {
		t.Error("went well should be nil when not provided")
	}
}

func TestAAR_MissingObjective(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{
		"audit", "aar",
		"--node", "my-project",
		"--task", "task-0001",
		"--what-happened", "Something happened",
	})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when objective is missing")
	}
}

func TestAAR_MissingWhatHappened(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{
		"audit", "aar",
		"--node", "my-project",
		"--task", "task-0001",
		"--objective", "Do the thing",
	})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when what-happened is missing")
	}
}

func TestAAR_MissingTask(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{
		"audit", "aar",
		"--node", "my-project",
		"--objective", "Do the thing",
		"--what-happened", "Did it",
	})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when task is missing")
	}
}

func TestAAR_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil
	env.App.State = nil

	env.RootCmd.SetArgs([]string{
		"audit", "aar",
		"--node", "my-project",
		"--task", "task-0001",
		"--objective", "Do the thing",
		"--what-happened", "Did it",
	})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error without identity")
	}
}

func TestAAR_OverwritesPrior(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	// First AAR
	env.RootCmd.SetArgs([]string{
		"audit", "aar",
		"--node", "my-project",
		"--task", "task-0001",
		"--objective", "First attempt",
		"--what-happened", "Failed",
	})
	_ = env.RootCmd.Execute()

	// Second AAR for same task
	env.RootCmd.SetArgs([]string{
		"audit", "aar",
		"--node", "my-project",
		"--task", "task-0001",
		"--objective", "Second attempt",
		"--what-happened", "Succeeded",
	})
	_ = env.RootCmd.Execute()

	ns := env.loadNodeState(t, "my-project")
	if len(ns.AARs) != 1 {
		t.Fatalf("expected 1 AAR after overwrite, got %d", len(ns.AARs))
	}
	if ns.AARs["task-0001"].Objective != "Second attempt" {
		t.Error("AAR should be overwritten with second attempt")
	}
}

func TestAAR_MultipleTaskIDs(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	for _, taskID := range []string{"task-0001", "task-0002", "task-0003"} {
		env.RootCmd.SetArgs([]string{
			"audit", "aar",
			"--node", "my-project",
			"--task", taskID,
			"--objective", "Work on " + taskID,
			"--what-happened", "Completed " + taskID,
		})
		if err := env.RootCmd.Execute(); err != nil {
			t.Fatalf("aar for %s failed: %v", taskID, err)
		}
	}

	ns := env.loadNodeState(t, "my-project")
	if len(ns.AARs) != 3 {
		t.Fatalf("expected 3 AARs, got %d", len(ns.AARs))
	}

	// Verify each is stored correctly
	for _, taskID := range []string{"task-0001", "task-0002", "task-0003"} {
		aar, ok := ns.AARs[taskID]
		if !ok {
			t.Errorf("missing AAR for %s", taskID)
			continue
		}
		if aar.TaskID != taskID {
			t.Errorf("expected TaskID %s, got %s", taskID, aar.TaskID)
		}
	}
}
