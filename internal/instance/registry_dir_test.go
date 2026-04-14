package instance

import (
	"path/filepath"
	"testing"
)

// TestRegistryDir_UsesHomeWhenOverrideEmpty exercises the default
// ~/.wolfcastle/instances path. We redirect HOME to a temp dir so the
// call touches no real user state.
func TestRegistryDir_UsesHomeWhenOverrideEmpty(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	savedOverride := RegistryDirOverride
	RegistryDirOverride = ""
	t.Cleanup(func() { RegistryDirOverride = savedOverride })

	dir, err := registryDir()
	if err != nil {
		t.Fatalf("registryDir: %v", err)
	}
	want := filepath.Join(fakeHome, ".wolfcastle", "instances")
	if dir != want {
		t.Errorf("registryDir = %q, want %q", dir, want)
	}
}

// TestList_DefaultRegistryDir verifies List works via the default
// path — ensuring the function handles a missing ~/.wolfcastle/instances
// gracefully as nil, nil.
func TestList_DefaultRegistryDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	savedOverride := RegistryDirOverride
	RegistryDirOverride = ""
	t.Cleanup(func() { RegistryDirOverride = savedOverride })

	entries, err := List()
	if err != nil {
		t.Fatalf("List on fresh home: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries in empty registry, got %+v", entries)
	}
}
