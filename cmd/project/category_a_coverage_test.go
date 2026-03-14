package project

import (
	"testing"
)

// ---------------------------------------------------------------------------
// create.go: ValidateSlug error (all-symbols name)
// ---------------------------------------------------------------------------

func TestProjectCreate_DigitStartName(t *testing.T) {
	env := newTestEnv(t)

	// "123test" produces slug "123test" which fails ValidateSlug
	// because it must start with a letter
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "123test"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for digit-start project name")
	}
}

func TestProjectCreate_AllDigitsName(t *testing.T) {
	env := newTestEnv(t)

	// "999" produces slug "999" which fails ValidateSlug
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "999"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for all-digits project name")
	}
}
