//go:build windows

package invoke

// RestoreTerminal is a no-op on Windows.
func RestoreTerminal() {}
