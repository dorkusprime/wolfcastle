package cmd

import (
	"io/fs"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/project"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

// testEnv sets up a temporary wolfcastle workspace backed by
// testutil.Environment, exposing the fields that cmd/ tests rely on.
type testEnv struct {
	RootDir       string
	WolfcastleDir string
	ProjectsDir   string
	App           *cmdutil.App
	env           *testutil.Environment
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	env := testutil.NewEnvironment(t)
	seedArtifactTemplates(t, env)
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

	// Commands call FindWolfcastleDir, which checks cwd for .wolfcastle.
	t.Chdir(env.ParentDir())

	return &testEnv{
		RootDir:       env.ParentDir(),
		WolfcastleDir: env.Root,
		ProjectsDir:   env.ProjectsDir(),
		App:           testApp,
		env:           env,
	}
}

// createLeafNode creates a leaf node with the given address in the test env.
func (e *testEnv) createLeafNode(t *testing.T, addr, name string) {
	t.Helper()
	e.env.WithProject(name, testutil.Leaf(addr))
}

// loadNodeState is a convenience for loading a node's state.json.
func (e *testEnv) loadNodeState(t *testing.T, addr string) *state.NodeState {
	t.Helper()
	ns, err := e.env.State.ReadNode(addr)
	if err != nil {
		t.Fatalf("loading node state for %s: %v", addr, err)
	}
	return ns
}

// seedArtifactTemplates writes embedded artifact templates (adr, spec, task)
// into the test environment's tier FS so that RenderToFile calls succeed.
func seedArtifactTemplates(t *testing.T, env *testutil.Environment) {
	t.Helper()
	sub, err := fs.Sub(project.Templates, "templates")
	if err != nil {
		t.Fatalf("extracting templates sub-FS: %v", err)
	}
	entries, err := fs.ReadDir(sub, "artifacts")
	if err != nil {
		t.Fatalf("reading artifacts dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := fs.ReadFile(sub, "artifacts/"+e.Name())
		if err != nil {
			t.Fatalf("reading artifact template %s: %v", e.Name(), err)
		}
		env.WithTemplate("artifacts/"+e.Name(), string(data))
	}
}
