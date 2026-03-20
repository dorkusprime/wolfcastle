package cmd

import (
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/clock"
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
	af := env.ToAppFields()

	testApp := &cmdutil.App{
		// Repository fields.
		Config:   af.Config,
		Identity: af.Identity,
		State:    af.State,
		Prompts:  af.Prompts,
		Classes:  af.Classes,
		Daemon:   af.Daemon,
		Git:      af.Git,
		Clock:    clock.New(),

		// Deprecated fields, kept while production code still reads them.
		WolfcastleDir: af.WolfcastleDir,
		Cfg:           af.Cfg,
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
