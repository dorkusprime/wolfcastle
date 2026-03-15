// Package errors defines typed error categories for the Wolfcastle daemon.
// The daemon inspects error types to decide retry vs abort behavior:
// InvocationError is retryable, StateError and NavigationError are fatal,
// ConfigError prevents startup.
package errors

import "fmt"

// ConfigError indicates a configuration problem (missing model, invalid
// field, unresolvable prompt template). These prevent the daemon from
// starting or a stage from running.
type ConfigError struct {
	Err error
}

func (e *ConfigError) Error() string { return fmt.Sprintf("config: %v", e.Err) }
func (e *ConfigError) Unwrap() error { return e.Err }

// StateError indicates corrupted or inconsistent state on disk (broken
// JSON, invalid state transitions, multiple in-progress tasks). These
// are fatal: the daemon should stop rather than risk further corruption.
type StateError struct {
	Err error
}

func (e *StateError) Error() string { return fmt.Sprintf("state: %v", e.Err) }
func (e *StateError) Unwrap() error { return e.Err }

// InvocationError indicates a model invocation failure (process exit,
// timeout, retry exhaustion). These are retryable: the daemon logs the
// error and moves to the next iteration.
type InvocationError struct {
	Err error
}

func (e *InvocationError) Error() string { return fmt.Sprintf("invocation: %v", e.Err) }
func (e *InvocationError) Unwrap() error { return e.Err }

// NavigationError indicates a failure in tree traversal (address parsing,
// root index loading, FindNextTask). These are fatal when they indicate
// structural problems, retryable when caused by transient I/O.
type NavigationError struct {
	Err error
}

func (e *NavigationError) Error() string { return fmt.Sprintf("navigation: %v", e.Err) }
func (e *NavigationError) Unwrap() error { return e.Err }

// Config wraps an error as a ConfigError.
func Config(err error) error { return &ConfigError{Err: err} }

// State wraps an error as a StateError.
func State(err error) error { return &StateError{Err: err} }

// Invocation wraps an error as an InvocationError.
func Invocation(err error) error { return &InvocationError{Err: err} }

// Navigation wraps an error as a NavigationError.
func Navigation(err error) error { return &NavigationError{Err: err} }
