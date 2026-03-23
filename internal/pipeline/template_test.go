package pipeline_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

// seedArtifactTemplatesFromDisk reads artifact templates from the source tree
// and writes them into the test environment's tier FS. This avoids importing
// internal/project (which would create a cycle through project_create.go).
func seedArtifactTemplatesFromDisk(t *testing.T, env *testutil.Environment) {
	t.Helper()
	dir := filepath.Join("..", "project", "templates", "artifacts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading artifact templates dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tmpl") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading artifact template %s: %v", e.Name(), err)
		}
		env.WithTemplate("artifacts/"+e.Name(), string(data))
	}
}

// readGolden loads a golden file from testdata/snapshots/.
func readGolden(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", "snapshots", name+".golden")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file %s: %v", path, err)
	}
	return string(data)
}

func TestSnapshotADR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		data   pipeline.ADRData
		golden string
	}{
		{
			name: "with body",
			data: pipeline.ADRData{
				Title: "Use PostgreSQL",
				Date:  "2026-03-22",
				Body:  "## Context\nWe need a relational database.\n\n## Decision\nPostgreSQL.\n\n## Consequences\nRequires DBA support.\n",
			},
			golden: "adr_with_body",
		},
		{
			name: "without body",
			data: pipeline.ADRData{
				Title: "Switch to gRPC",
				Date:  "2026-01-15",
				Body:  "",
			},
			golden: "adr_without_body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := testutil.NewEnvironment(t)
			seedArtifactTemplatesFromDisk(t, env)

			repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
			got, err := repo.ResolveTemplate("artifacts/adr.md", tt.data)
			if err != nil {
				t.Fatalf("ResolveTemplate: %v", err)
			}

			want := readGolden(t, tt.golden)
			if got != want {
				t.Errorf("snapshot mismatch for %s\nwant (%d bytes):\n%s\ngot  (%d bytes):\n%s",
					tt.golden, len(want), want, len(got), got)
			}
		})
	}
}

func TestSnapshotSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		data   pipeline.SpecData
		golden string
	}{
		{
			name:   "with body",
			data:   pipeline.SpecData{Title: "API Auth Flow", Body: "Authentication uses JWT tokens."},
			golden: "spec_with_body",
		},
		{
			name:   "without body",
			data:   pipeline.SpecData{Title: "Empty Spec", Body: ""},
			golden: "spec_without_body",
		},
		{
			name:   "stdin multiline body",
			data:   pipeline.SpecData{Title: "Auth Protocol", Body: "Users authenticate via OAuth2.\nTokens expire after 1 hour."},
			golden: "spec_stdin_multiline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := testutil.NewEnvironment(t)
			seedArtifactTemplatesFromDisk(t, env)

			repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
			got, err := repo.ResolveTemplate("artifacts/spec.md", tt.data)
			if err != nil {
				t.Fatalf("ResolveTemplate: %v", err)
			}

			want := readGolden(t, tt.golden)
			if got != want {
				t.Errorf("snapshot mismatch for %s\nwant (%d bytes):\n%s\ngot  (%d bytes):\n%s",
					tt.golden, len(want), want, len(got), got)
			}
		})
	}
}

func TestSnapshotTask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		data   pipeline.TaskData
		golden string
	}{
		{
			name:   "with body",
			data:   pipeline.TaskData{Title: "Implement auth", Body: "Add JWT middleware to all routes."},
			golden: "task_with_body",
		},
		{
			name:   "minimal",
			data:   pipeline.TaskData{Title: "Fix bug"},
			golden: "task_minimal",
		},
		{
			name: "all optional fields populated",
			data: pipeline.TaskData{
				Title:              "Add caching layer",
				Body:               "Implement Redis-based caching.",
				ID:                 "task-0042",
				Description:        "Full caching implementation",
				Type:               "implementation",
				Class:              "go",
				Deliverables:       []string{"internal/cache/cache.go", "internal/cache/cache_test.go"},
				Constraints:        []string{"Must use Redis", "TTL configurable"},
				References:         []string{"ADR-015"},
				AcceptanceCriteria: []string{"Cache hit rate > 80%", "All tests pass"},
			},
			golden: "task_all_fields",
		},
		{
			name:   "whitespace-only body treated as empty",
			data:   pipeline.TaskData{Title: "Fix bug", Body: "   \n  \t  "},
			golden: "task_minimal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := testutil.NewEnvironment(t)
			seedArtifactTemplatesFromDisk(t, env)

			// Mirror the TrimSpace pre-processing that call sites apply.
			data := tt.data
			if strings.TrimSpace(data.Body) == "" {
				data.Body = ""
			}

			repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
			got, err := repo.ResolveTemplate("artifacts/task.md", data)
			if err != nil {
				t.Fatalf("ResolveTemplate: %v", err)
			}

			want := readGolden(t, tt.golden)
			if got != want {
				t.Errorf("snapshot mismatch for %s\nwant (%d bytes):\n%s\ngot  (%d bytes):\n%s",
					tt.golden, len(want), want, len(got), got)
			}
		})
	}
}

func TestSnapshotAuditTask(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	seedArtifactTemplatesFromDisk(t, env)

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	// Audit-task template has no variables; rendered with nil ctx returns raw content.
	got, err := repo.ResolveTemplate("artifacts/audit-task.md", nil)
	if err != nil {
		t.Fatalf("ResolveTemplate: %v", err)
	}

	want := readGolden(t, "audit_task")
	if got != want {
		t.Errorf("snapshot mismatch for audit_task\nwant (%d bytes):\n%s\ngot  (%d bytes):\n%s",
			len(want), want, len(got), got)
	}
}
