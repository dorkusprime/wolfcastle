package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestBuildIterationContext_RichTaskFields(t *testing.T) {
	ns := state.NewNodeState("test", "Test", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{
			ID:          "task-0001",
			Description: "Implement cache package",
			State:       state.StatusInProgress,
			Body:        "Create internal/cache with a TTL-based struct using sync.RWMutex.",
			TaskType:    "implementation",
			Deliverables: []string{
				"internal/cache/cache.go",
				"internal/cache/cache_test.go",
			},
			AcceptanceCriteria: []string{
				"Cache.Get returns value and true for non-expired entries",
				"Cache.Get returns zero and false for expired entries",
				"go test -race ./internal/cache/ passes",
			},
			Constraints: []string{
				"Do not modify internal/config/",
				"Do not add external dependencies",
			},
			References: []string{
				"docs/specs/cache-contract.md",
				"internal/state/store.go (example of mutex usage)",
			},
			Integration: "Wire into App.Init after ConfigRepository. Used by PromptRepository for base tier caching.",
		},
	}

	ctx := BuildIterationContext("test", ns, "task-0001")

	// Task type
	if !strings.Contains(ctx, "**Task Type:** implementation") {
		t.Error("context should include task type")
	}

	// Body
	if !strings.Contains(ctx, "## Task Details") {
		t.Error("context should include task details section")
	}
	if !strings.Contains(ctx, "TTL-based struct") {
		t.Error("context should include body content")
	}

	// Integration
	if !strings.Contains(ctx, "## Integration") {
		t.Error("context should include integration section")
	}
	if !strings.Contains(ctx, "Wire into App.Init") {
		t.Error("context should include integration content")
	}

	// Deliverables
	if !strings.Contains(ctx, "internal/cache/cache.go") {
		t.Error("context should include deliverables")
	}

	// Acceptance criteria
	if !strings.Contains(ctx, "**Acceptance Criteria:**") {
		t.Error("context should include acceptance criteria section")
	}
	if !strings.Contains(ctx, "Cache.Get returns value and true") {
		t.Error("context should include acceptance criterion content")
	}

	// Constraints
	if !strings.Contains(ctx, "**Constraints:**") {
		t.Error("context should include constraints section")
	}
	if !strings.Contains(ctx, "Do not modify internal/config/") {
		t.Error("context should include constraint content")
	}

	// References
	if !strings.Contains(ctx, "**Reference Material:**") {
		t.Error("context should include references section")
	}
	if !strings.Contains(ctx, "docs/specs/cache-contract.md") {
		t.Error("context should include reference path")
	}
}

func TestBuildIterationContext_EmptyRichFields(t *testing.T) {
	ns := state.NewNodeState("test", "Test", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{
			ID:          "task-0001",
			Description: "Simple task with no rich fields",
			State:       state.StatusInProgress,
		},
	}

	ctx := BuildIterationContext("test", ns, "task-0001")

	// None of the rich sections should appear
	if strings.Contains(ctx, "**Task Type:**") {
		t.Error("empty task type should not render")
	}
	if strings.Contains(ctx, "## Task Details") {
		t.Error("empty body should not render details section")
	}
	if strings.Contains(ctx, "## Integration") {
		t.Error("empty integration should not render section")
	}
	if strings.Contains(ctx, "**Acceptance Criteria:**") {
		t.Error("empty acceptance criteria should not render")
	}
	if strings.Contains(ctx, "**Constraints:**") {
		t.Error("empty constraints should not render")
	}
	if strings.Contains(ctx, "**Reference Material:**") {
		t.Error("empty references should not render")
	}
}

func TestBuildIterationContext_InlinesSpecReferences(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "spec.md")
	_ = os.WriteFile(specPath, []byte("# Cache Contract\n\nGet(key) returns (value, bool)."), 0644)

	ns := state.NewNodeState("test", "Test", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{
			ID:          "task-0001",
			Description: "Implement cache",
			State:       state.StatusInProgress,
			References:  []string{specPath},
		},
	}

	ctx := BuildIterationContext("test", ns, "task-0001")

	if !strings.Contains(ctx, "### Reference:") {
		t.Error("context should inline spec content")
	}
	if !strings.Contains(ctx, "Cache Contract") {
		t.Error("context should contain spec body")
	}
	if !strings.Contains(ctx, "Get(key) returns") {
		t.Error("context should contain spec details")
	}
}

func TestBuildIterationContext_PartialRichFields(t *testing.T) {
	ns := state.NewNodeState("test", "Test", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{
			ID:          "task-0001",
			Description: "Task with some rich fields",
			State:       state.StatusInProgress,
			TaskType:    "spec",
			Constraints: []string{"Do not create implementation code"},
		},
	}

	ctx := BuildIterationContext("test", ns, "task-0001")

	if !strings.Contains(ctx, "**Task Type:** spec") {
		t.Error("task type should render")
	}
	if !strings.Contains(ctx, "**Constraints:**") {
		t.Error("constraints should render")
	}
	// Fields not set should not appear
	if strings.Contains(ctx, "## Task Details") {
		t.Error("empty body should not render")
	}
	if strings.Contains(ctx, "**Acceptance Criteria:**") {
		t.Error("empty acceptance should not render")
	}
}
