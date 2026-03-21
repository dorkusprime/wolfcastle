// Package selfupdate handles binary self-update from the release channel.
// When no release channel is configured, the stub updater signals
// unavailability rather than falsely claiming the binary is current.
// The interface is stable and ready for integration.
package selfupdate

import "fmt"

// Result describes the outcome of an update check.
type Result struct {
	// CurrentVersion is the version currently running.
	CurrentVersion string
	// LatestVersion is the latest version available in the release channel.
	LatestVersion string
	// Updated is true if a new binary was installed.
	Updated bool
	// AlreadyCurrent is true if the running version is already the latest.
	AlreadyCurrent bool
	// Unavailable is true when the update mechanism has no release channel
	// to query. A result with Unavailable set will have both Updated and
	// AlreadyCurrent as false; callers should treat this as "unknown" rather
	// than "up to date" or "failed."
	Unavailable bool
}

// Updater checks for and applies binary updates.
type Updater interface {
	// Check queries the release channel for the latest version.
	Check() (*Result, error)
	// Apply downloads and replaces the running binary with the latest version.
	Apply() (*Result, error)
}

// NewUpdater returns an Updater for the given release channel.
// Currently returns a stub that reports updates as unavailable, since
// no release channel exists yet.
func NewUpdater(currentVersion string) Updater {
	return &stubUpdater{version: currentVersion}
}

type stubUpdater struct {
	version string
	// checkFn overrides Check behavior for testing. Nil means use default.
	checkFn func() (*Result, error)
}

func (s *stubUpdater) Check() (*Result, error) {
	if s.checkFn != nil {
		return s.checkFn()
	}
	return &Result{
		CurrentVersion: s.version,
		Unavailable:    true,
	}, nil
}

func (s *stubUpdater) Apply() (*Result, error) {
	result, err := s.Check()
	if err != nil {
		return nil, fmt.Errorf("checking for updates: %w", err)
	}
	if result.Unavailable || result.AlreadyCurrent {
		return result, nil
	}
	// When distribution infrastructure exists, this will:
	// 1. Download the latest release from the configured channel
	// 2. Verify the checksum
	// 3. Replace the running binary
	// 4. Return Updated: true
	return result, nil
}
