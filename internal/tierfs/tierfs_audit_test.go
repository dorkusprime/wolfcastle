package tierfs

import (
	"testing"
)

func TestSystemTierPaths_ReturnsCorrectPaths(t *testing.T) {
	t.Parallel()
	got := SystemTierPaths()

	if len(got) != len(TierNames) {
		t.Fatalf("SystemTierPaths returned %d paths, want %d", len(got), len(TierNames))
	}

	want := []string{"system/base", "system/custom", "system/local"}
	for i, p := range got {
		if p != want[i] {
			t.Errorf("SystemTierPaths()[%d] = %q, want %q", i, p, want[i])
		}
	}
}
