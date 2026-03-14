// Package selfupdate handles binary self-update from the release channel.
// The actual download/replace logic is stubbed until production distribution
// infrastructure is in place. The interface is stable and ready for integration.
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
}

// Updater checks for and applies binary updates.
type Updater interface {
	// Check queries the release channel for the latest version.
	Check() (*Result, error)
	// Apply downloads and replaces the running binary with the latest version.
	Apply() (*Result, error)
}

// NewUpdater returns an Updater for the given release channel.
// Currently returns a stub that reports the binary is already current.
func NewUpdater(currentVersion string) Updater {
	return &stubUpdater{version: currentVersion}
}

type stubUpdater struct {
	version string
}

func (s *stubUpdater) Check() (*Result, error) {
	return &Result{
		CurrentVersion: s.version,
		LatestVersion:  s.version,
		AlreadyCurrent: true,
	}, nil
}

func (s *stubUpdater) Apply() (*Result, error) {
	result, err := s.Check()
	if err != nil {
		return nil, fmt.Errorf("checking for updates: %w", err)
	}
	if result.AlreadyCurrent {
		return result, nil
	}
	// When distribution infrastructure exists, this will:
	// 1. Download the latest release from the configured channel
	// 2. Verify the checksum
	// 3. Replace the running binary
	// 4. Return Updated: true
	return result, nil
}
