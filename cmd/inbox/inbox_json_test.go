package inbox

import (
	"testing"
)

func TestInboxAdd_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{"inbox", "add", "test idea"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("inbox add (json) failed: %v", err)
	}
}

func TestInboxList_JSONOutput_Empty(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{"inbox", "list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("inbox list (json) failed: %v", err)
	}
}

func TestInboxList_JSONOutput_WithItems(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"inbox", "add", "idea one"})
	_ = env.RootCmd.Execute()

	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{"inbox", "list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("inbox list (json) failed: %v", err)
	}
}

func TestInboxClear_JSONOutput(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"inbox", "add", "idea"})
	_ = env.RootCmd.Execute()

	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{"inbox", "clear", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("inbox clear (json) failed: %v", err)
	}
}
