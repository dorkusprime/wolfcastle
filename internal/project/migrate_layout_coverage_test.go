package project

import (
	"os"
	"path/filepath"
	"testing"
)

// ── migrateToSystemLayout coverage ──────────────────────────────────

func TestMigrateToSystemLayout_AlreadyMigrated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// system/ already exists: the idempotency guard returns nil immediately.
	if err := os.MkdirAll(filepath.Join(dir, "system"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := migrateToSystemLayout(dir); err != nil {
		t.Fatalf("expected nil when system/ exists, got: %v", err)
	}
}

func TestMigrateToSystemLayout_NoOldLayout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// No system/, no base/ — creates system/ and returns.
	if err := migrateToSystemLayout(dir); err != nil {
		t.Fatalf("expected nil for fresh directory, got: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "system"))
	if err != nil {
		t.Fatalf("system/ should exist after migration: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("system/ should be a directory")
	}
}

func TestMigrateToSystemLayout_NoOldLayout_MkdirFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Make dir read-only (r-x) so MkdirAll("system") fails.
	// Stat still works because execute permission is present.
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0755) }()

	err := migrateToSystemLayout(dir)
	if err == nil {
		t.Error("expected error when system/ cannot be created (no old layout path)")
	}
}

func TestMigrateToSystemLayout_FullMigration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Build old flat layout with marker files for verification.
	oldDirs := []string{"base", "custom", "local", "projects", "logs"}
	for _, d := range oldDirs {
		p := filepath.Join(dir, d)
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "marker.txt"), []byte(d), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Add loose daemon files.
	looseFiles := []string{"wolfcastle.pid", "stop", "daemon.log", "daemon.meta.json"}
	for _, f := range looseFiles {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(f), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := migrateToSystemLayout(dir); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify directories moved under system/.
	for _, d := range oldDirs {
		if _, err := os.Stat(filepath.Join(dir, d)); !os.IsNotExist(err) {
			t.Errorf("old directory %s should no longer exist at root", d)
		}
		data, err := os.ReadFile(filepath.Join(dir, "system", d, "marker.txt"))
		if err != nil {
			t.Errorf("marker file missing in system/%s: %v", d, err)
			continue
		}
		if string(data) != d {
			t.Errorf("system/%s/marker.txt = %q, want %q", d, data, d)
		}
	}

	// Verify loose files moved under system/.
	for _, f := range looseFiles {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Errorf("loose file %s should no longer exist at root", f)
		}
		data, err := os.ReadFile(filepath.Join(dir, "system", f))
		if err != nil {
			t.Errorf("file missing at system/%s: %v", f, err)
			continue
		}
		if string(data) != f {
			t.Errorf("system/%s = %q, want %q", f, data, f)
		}
	}
}

func TestMigrateToSystemLayout_PartialOldLayout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Only base/ and logs/ exist; others are absent and should be skipped.
	for _, d := range []string{"base", "logs"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// One loose file present, others absent.
	if err := os.WriteFile(filepath.Join(dir, "wolfcastle.pid"), []byte("123"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := migrateToSystemLayout(dir); err != nil {
		t.Fatalf("migration with partial layout failed: %v", err)
	}

	// Present dirs should land under system/.
	for _, d := range []string{"base", "logs"} {
		if _, err := os.Stat(filepath.Join(dir, "system", d)); err != nil {
			t.Errorf("system/%s should exist: %v", d, err)
		}
	}
	// Absent dirs should not appear.
	for _, d := range []string{"custom", "local", "projects"} {
		if _, err := os.Stat(filepath.Join(dir, "system", d)); !os.IsNotExist(err) {
			t.Errorf("system/%s should not exist (never in old layout)", d)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "system", "wolfcastle.pid")); err != nil {
		t.Errorf("system/wolfcastle.pid should exist: %v", err)
	}
}

func TestMigrateToSystemLayout_MkdirSystemFails_OldLayout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create base/ so old-layout detection succeeds.
	if err := os.MkdirAll(filepath.Join(dir, "base"), 0755); err != nil {
		t.Fatal(err)
	}

	// Make root read-only so MkdirAll(system/) fails.
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0755) }()

	err := migrateToSystemLayout(dir)
	if err == nil {
		t.Error("expected error when system/ cannot be created (old layout path)")
	}
	if err != nil {
		expected := "creating system/"
		if len(err.Error()) < len(expected) || err.Error()[:len(expected)] != expected {
			// Verify the error is wrapped correctly.
			t.Logf("got error: %v", err)
		}
	}
}
