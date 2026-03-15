package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestConfigError(t *testing.T) {
	base := fmt.Errorf("missing model")
	err := Config(base)

	if err.Error() != "config: missing model" {
		t.Errorf("Error() = %q", err.Error())
	}
	if !errors.Is(err, base) {
		t.Error("errors.Is should find base error")
	}
	var target *ConfigError
	if !errors.As(err, &target) {
		t.Error("errors.As should match ConfigError")
	}
}

func TestStateError(t *testing.T) {
	base := fmt.Errorf("corrupt json")
	err := State(base)

	if err.Error() != "state: corrupt json" {
		t.Errorf("Error() = %q", err.Error())
	}
	if !errors.Is(err, base) {
		t.Error("errors.Is should find base error")
	}
	var target *StateError
	if !errors.As(err, &target) {
		t.Error("errors.As should match StateError")
	}
}

func TestInvocationError(t *testing.T) {
	base := fmt.Errorf("exit code 1")
	err := Invocation(base)

	if err.Error() != "invocation: exit code 1" {
		t.Errorf("Error() = %q", err.Error())
	}
	if !errors.Is(err, base) {
		t.Error("errors.Is should find base error")
	}
	var target *InvocationError
	if !errors.As(err, &target) {
		t.Error("errors.As should match InvocationError")
	}
}

func TestNavigationError(t *testing.T) {
	base := fmt.Errorf("bad address")
	err := Navigation(base)

	if err.Error() != "navigation: bad address" {
		t.Errorf("Error() = %q", err.Error())
	}
	if !errors.Is(err, base) {
		t.Error("errors.Is should find base error")
	}
	var target *NavigationError
	if !errors.As(err, &target) {
		t.Error("errors.As should match NavigationError")
	}
}

func TestUnwrap(t *testing.T) {
	base := fmt.Errorf("root cause")
	types := []error{
		Config(base),
		State(base),
		Invocation(base),
		Navigation(base),
	}
	for _, err := range types {
		if errors.Unwrap(err) != base {
			t.Errorf("Unwrap(%T) did not return base error", err)
		}
	}
}

func TestErrorsAs_AcrossWrap(t *testing.T) {
	base := fmt.Errorf("disk full")
	wrapped := fmt.Errorf("saving state: %w", State(base))

	var stateErr *StateError
	if !errors.As(wrapped, &stateErr) {
		t.Error("errors.As should find StateError through fmt.Errorf wrapping")
	}
	if !errors.Is(wrapped, base) {
		t.Error("errors.Is should find base error through double wrapping")
	}
}
