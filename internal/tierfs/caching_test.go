package tierfs

import (
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// spyResolver records calls to Resolve and ResolveAll so tests can
// verify that the caching layer avoids redundant disk hits.
type spyResolver struct {
	inner        Resolver
	resolveCount atomic.Int64
	allCount     atomic.Int64
}

func (s *spyResolver) Resolve(relPath string) ([]byte, error) {
	s.resolveCount.Add(1)
	return s.inner.Resolve(relPath)
}

func (s *spyResolver) ResolveAll(subdir string) (map[string][]byte, error) {
	s.allCount.Add(1)
	return s.inner.ResolveAll(subdir)
}

func (s *spyResolver) WriteBase(relPath string, data []byte) error {
	return s.inner.WriteBase(relPath, data)
}

func (s *spyResolver) BasePath(subdir string) string {
	return s.inner.BasePath(subdir)
}

func (s *spyResolver) TierDirs() []string {
	return s.inner.TierDirs()
}

func TestDefaultTierTTLs_ReturnsPositiveDurations(t *testing.T) {
	t.Parallel()
	ttls := DefaultTierTTLs()
	if ttls.Base <= 0 {
		t.Errorf("Base TTL should be positive, got %v", ttls.Base)
	}
	if ttls.Custom <= 0 {
		t.Errorf("Custom TTL should be positive, got %v", ttls.Custom)
	}
	if ttls.Local <= 0 {
		t.Errorf("Local TTL should be positive, got %v", ttls.Local)
	}
	// Base should be much longer than custom/local
	if ttls.Base <= ttls.Custom {
		t.Errorf("Base TTL (%v) should be longer than Custom TTL (%v)", ttls.Base, ttls.Custom)
	}
}

func TestCachingResolver_CachedReadSkipsDisk(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/base/f.md", "hello")

	spy := &spyResolver{inner: New(root)}
	cr := NewCachingResolver(spy, 10*time.Second)

	got1, err := cr.Resolve("f.md")
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	got2, err := cr.Resolve("f.md")
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}

	if string(got1) != "hello" || string(got2) != "hello" {
		t.Fatalf("unexpected content: %q / %q", got1, got2)
	}
	if spy.resolveCount.Load() != 1 {
		t.Errorf("expected 1 disk read, got %d", spy.resolveCount.Load())
	}
}

func TestCachingResolver_ResolveAllCached(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/base/prompts/a.md", "alpha")
	writeFile(t, root+"/base/prompts/b.md", "beta")

	spy := &spyResolver{inner: New(root)}
	cr := NewCachingResolver(spy, 10*time.Second)

	got1, err := cr.ResolveAll("prompts")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	got2, err := cr.ResolveAll("prompts")
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if len(got1) != 2 || len(got2) != 2 {
		t.Fatalf("unexpected lengths: %d / %d", len(got1), len(got2))
	}
	if spy.allCount.Load() != 1 {
		t.Errorf("expected 1 disk read, got %d", spy.allCount.Load())
	}
}

func TestCachingResolver_ExpiredEntriesReRead(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/base/f.md", "v1")

	spy := &spyResolver{inner: New(root)}
	cr := NewCachingResolver(spy, 50*time.Millisecond)

	// Use a controllable clock to avoid real sleeps.
	now := time.Now()
	cr.now = func() time.Time { return now }

	_, _ = cr.Resolve("f.md")
	if spy.resolveCount.Load() != 1 {
		t.Fatalf("expected 1 read after first call, got %d", spy.resolveCount.Load())
	}

	// Still within TTL.
	now = now.Add(10 * time.Millisecond)
	_, _ = cr.Resolve("f.md")
	if spy.resolveCount.Load() != 1 {
		t.Fatalf("expected cache hit, got %d reads", spy.resolveCount.Load())
	}

	// Advance past TTL.
	now = now.Add(100 * time.Millisecond)

	// Update the file on disk so we can verify the re-read picks it up.
	writeFile(t, root+"/base/f.md", "v2")

	got, err := cr.Resolve("f.md")
	if err != nil {
		t.Fatalf("resolve after expiry: %v", err)
	}
	if string(got) != "v2" {
		t.Errorf("expected v2 after expiry, got %q", got)
	}
	if spy.resolveCount.Load() != 2 {
		t.Errorf("expected 2 reads after expiry, got %d", spy.resolveCount.Load())
	}
}

func TestCachingResolver_ResolveAllExpiry(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/base/rules/a.md", "orig")

	spy := &spyResolver{inner: New(root)}
	cr := NewCachingResolver(spy, 50*time.Millisecond)

	now := time.Now()
	cr.now = func() time.Time { return now }

	_, _ = cr.ResolveAll("rules")
	if spy.allCount.Load() != 1 {
		t.Fatalf("expected 1 read, got %d", spy.allCount.Load())
	}

	// Advance past TTL.
	now = now.Add(100 * time.Millisecond)

	_, _ = cr.ResolveAll("rules")
	if spy.allCount.Load() != 2 {
		t.Errorf("expected 2 reads after expiry, got %d", spy.allCount.Load())
	}
}

func TestCachingResolver_InvalidateClearsEntry(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/base/f.md", "cached")

	spy := &spyResolver{inner: New(root)}
	cr := NewCachingResolver(spy, 10*time.Minute)

	_, _ = cr.Resolve("f.md")
	if spy.resolveCount.Load() != 1 {
		t.Fatal("expected 1 read")
	}

	cr.Invalidate("f.md")

	_, _ = cr.Resolve("f.md")
	if spy.resolveCount.Load() != 2 {
		t.Errorf("expected re-read after invalidation, got %d", spy.resolveCount.Load())
	}
}

func TestCachingResolver_InvalidateAllClearsEverything(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/base/a.md", "a")
	writeFile(t, root+"/base/b.md", "b")
	writeFile(t, root+"/base/prompts/c.md", "c")

	spy := &spyResolver{inner: New(root)}
	cr := NewCachingResolver(spy, 10*time.Minute)

	_, _ = cr.Resolve("a.md")
	_, _ = cr.Resolve("b.md")
	_, _ = cr.ResolveAll("prompts")

	cr.InvalidateAll()

	_, _ = cr.Resolve("a.md")
	_, _ = cr.Resolve("b.md")
	_, _ = cr.ResolveAll("prompts")

	if spy.resolveCount.Load() != 4 {
		t.Errorf("expected 4 resolve calls (2 before + 2 after), got %d", spy.resolveCount.Load())
	}
	if spy.allCount.Load() != 2 {
		t.Errorf("expected 2 resolveAll calls, got %d", spy.allCount.Load())
	}
}

func TestCachingResolver_CachesNotFoundErrors(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(root+"/base", 0o755)

	spy := &spyResolver{inner: New(root)}
	cr := NewCachingResolver(spy, 10*time.Minute)

	_, err1 := cr.Resolve("missing.md")
	_, err2 := cr.Resolve("missing.md")

	if !errors.Is(err1, os.ErrNotExist) || !errors.Is(err2, os.ErrNotExist) {
		t.Fatalf("expected ErrNotExist both times, got %v / %v", err1, err2)
	}
	if spy.resolveCount.Load() != 1 {
		t.Errorf("expected 1 disk read for missing file, got %d", spy.resolveCount.Load())
	}
}

func TestCachingResolver_WriteBaseInvalidatesCache(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/base/f.md", "original")

	spy := &spyResolver{inner: New(root)}
	cr := NewCachingResolver(spy, 10*time.Minute)

	got1, _ := cr.Resolve("f.md")
	if string(got1) != "original" {
		t.Fatalf("got %q, want original", got1)
	}

	if err := cr.WriteBase("f.md", []byte("updated")); err != nil {
		t.Fatalf("write: %v", err)
	}

	got2, _ := cr.Resolve("f.md")
	if string(got2) != "updated" {
		t.Errorf("expected updated after write, got %q", got2)
	}
	if spy.resolveCount.Load() != 2 {
		t.Errorf("expected 2 reads (initial + post-write), got %d", spy.resolveCount.Load())
	}
}

func TestCachingResolver_ZeroTTLNeverExpires(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/base/f.md", "forever")

	spy := &spyResolver{inner: New(root)}
	cr := NewCachingResolver(spy, 0)

	now := time.Now()
	cr.now = func() time.Time { return now }

	_, _ = cr.Resolve("f.md")

	// Advance time significantly.
	now = now.Add(999 * time.Hour)

	_, _ = cr.Resolve("f.md")
	if spy.resolveCount.Load() != 1 {
		t.Errorf("zero TTL should never expire, got %d reads", spy.resolveCount.Load())
	}
}

func TestCachingResolver_PassthroughMethods(t *testing.T) {
	root := t.TempDir()
	inner := New(root)
	cr := NewCachingResolver(inner, time.Minute)

	if cr.BasePath("prompts") != inner.BasePath("prompts") {
		t.Error("BasePath mismatch")
	}

	dirs := cr.TierDirs()
	innerDirs := inner.TierDirs()
	if len(dirs) != len(innerDirs) {
		t.Fatal("TierDirs length mismatch")
	}
	for i := range dirs {
		if dirs[i] != innerDirs[i] {
			t.Errorf("TierDirs[%d]: %q != %q", i, dirs[i], innerDirs[i])
		}
	}
}
