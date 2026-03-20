package tierfs

import (
	"sync"
	"time"
)

// TierTTLs holds per-tier TTL durations, keyed by tier name.
// A zero or negative TTL means the cached entry never expires.
type TierTTLs struct {
	Base   time.Duration
	Custom time.Duration
	Local  time.Duration
}

// DefaultTierTTLs returns sensible defaults: base never expires (set to 24h
// as a practical "infinite"), custom and local expire after 30 seconds.
func DefaultTierTTLs() TierTTLs {
	return TierTTLs{
		Base:   24 * time.Hour,
		Custom: 30 * time.Second,
		Local:  30 * time.Second,
	}
}

// cacheEntry stores a resolved value alongside its expiry time.
type cacheEntry struct {
	data    []byte
	err     error
	expires time.Time
}

// resolveAllEntry stores a cached ResolveAll result.
type resolveAllEntry struct {
	data    map[string][]byte
	err     error
	expires time.Time
}

// CachingResolver wraps a Resolver with TTL-based caching for Resolve
// and ResolveAll calls. Write operations and metadata queries pass
// through to the underlying resolver and invalidate relevant cache
// entries.
type CachingResolver struct {
	inner Resolver
	ttl   time.Duration // single TTL used for all cached entries
	now   func() time.Time

	mu       sync.RWMutex
	cache    map[string]cacheEntry
	allCache map[string]resolveAllEntry
}

// NewCachingResolver creates a CachingResolver wrapping inner with the
// given TTL applied to all cache entries. For per-tier TTLs, the
// outermost resolver determines caching granularity; since tierfs.FS
// already handles tier priority internally, a single TTL governs how
// long a resolved result (from whichever tier won) stays cached.
func NewCachingResolver(inner Resolver, ttl time.Duration) *CachingResolver {
	return &CachingResolver{
		inner:    inner,
		ttl:      ttl,
		now:      time.Now,
		cache:    make(map[string]cacheEntry),
		allCache: make(map[string]resolveAllEntry),
	}
}

// Resolve returns cached content if a valid entry exists, otherwise
// delegates to the inner resolver and caches the result.
func (c *CachingResolver) Resolve(relPath string) ([]byte, error) {
	now := c.now()

	c.mu.RLock()
	entry, ok := c.cache[relPath]
	c.mu.RUnlock()

	if ok && (c.ttl <= 0 || now.Before(entry.expires)) {
		if entry.err != nil {
			return nil, entry.err
		}
		// Return a copy so callers can't mutate the cached slice.
		cp := make([]byte, len(entry.data))
		copy(cp, entry.data)
		return cp, nil
	}

	data, err := c.inner.Resolve(relPath)

	c.mu.Lock()
	c.cache[relPath] = cacheEntry{
		data:    data,
		err:     err,
		expires: now.Add(c.ttl),
	}
	c.mu.Unlock()

	return data, err
}

// ResolveAll returns cached results if valid, otherwise delegates to
// the inner resolver.
func (c *CachingResolver) ResolveAll(subdir string) (map[string][]byte, error) {
	now := c.now()

	c.mu.RLock()
	entry, ok := c.allCache[subdir]
	c.mu.RUnlock()

	if ok && (c.ttl <= 0 || now.Before(entry.expires)) {
		if entry.err != nil {
			return nil, entry.err
		}
		// Return a shallow copy of the map.
		cp := make(map[string][]byte, len(entry.data))
		for k, v := range entry.data {
			cp[k] = v
		}
		return cp, nil
	}

	data, err := c.inner.ResolveAll(subdir)

	c.mu.Lock()
	c.allCache[subdir] = resolveAllEntry{
		data:    data,
		err:     err,
		expires: now.Add(c.ttl),
	}
	c.mu.Unlock()

	return data, err
}

// WriteBase delegates to the inner resolver and invalidates the cache
// for the written path.
func (c *CachingResolver) WriteBase(relPath string, data []byte) error {
	err := c.inner.WriteBase(relPath, data)
	if err == nil {
		c.Invalidate(relPath)
	}
	return err
}

// BasePath delegates directly to the inner resolver (no caching needed).
func (c *CachingResolver) BasePath(subdir string) string {
	return c.inner.BasePath(subdir)
}

// TierDirs delegates directly to the inner resolver.
func (c *CachingResolver) TierDirs() []string {
	return c.inner.TierDirs()
}

// Invalidate removes the cache entry for a specific path, affecting
// both the Resolve and ResolveAll caches.
func (c *CachingResolver) Invalidate(relPath string) {
	c.mu.Lock()
	delete(c.cache, relPath)
	// Clear all ResolveAll entries since we can't cheaply determine
	// which subdirectory caches might contain this path.
	c.allCache = make(map[string]resolveAllEntry)
	c.mu.Unlock()
}

// InvalidateAll clears the entire cache.
func (c *CachingResolver) InvalidateAll() {
	c.mu.Lock()
	c.cache = make(map[string]cacheEntry)
	c.allCache = make(map[string]resolveAllEntry)
	c.mu.Unlock()
}
