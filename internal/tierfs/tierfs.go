// Package tierfs provides three-tier file resolution for Wolfcastle's
// base < custom < local override hierarchy. It is the single source of
// truth for tier names and resolution order (ADR-063).
package tierfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TierNames defines the resolution order from lowest to highest priority.
// This slice is the canonical source of truth for tier names (ADR-063).
// All packages that need tier-aware paths should derive them from this
// constant rather than hardcoding their own lists.
var TierNames = []string{"base", "custom", "local"}

// SystemPrefix is the directory under the wolfcastle root that contains
// the three tiers (e.g. ".wolfcastle/system/base").
const SystemPrefix = "system"

// SystemTierPaths returns tier directory paths prefixed with SystemPrefix,
// in resolution order. Example output: ["system/base", "system/custom", "system/local"].
func SystemTierPaths() []string {
	paths := make([]string, len(TierNames))
	for i, name := range TierNames {
		paths[i] = SystemPrefix + "/" + name
	}
	return paths
}

// tiers is an alias kept for internal use within this package.
var tiers = TierNames

// Resolver reads and writes files through the three-tier overlay.
type Resolver interface {
	// Resolve returns content from the highest-priority tier that has
	// the file at relPath. Tiers are checked local -> custom -> base.
	// Returns a wrapped os.ErrNotExist if no tier contains the file.
	Resolve(relPath string) ([]byte, error)

	// ResolveAll collects every .md file in subdir across all tiers.
	// Higher-tier files overwrite lower-tier files with the same name.
	// Keys are filenames, values are file contents.
	ResolveAll(subdir string) (map[string][]byte, error)

	// WriteBase writes data to relPath within the base tier directory,
	// creating parent directories as needed.
	WriteBase(relPath string, data []byte) error

	// BasePath returns the absolute path to subdir within the base tier.
	BasePath(subdir string) string

	// TierDirs returns absolute paths to all tier directories in
	// resolution order (base, custom, local).
	TierDirs() []string
}

// FS implements Resolver over a root directory (e.g. ".wolfcastle/system").
type FS struct {
	root string
}

// New creates a new FS rooted at the given directory.
func New(root string) *FS {
	return &FS{root: root}
}

// Resolve walks tiers from highest priority (local) to lowest (base),
// returning the first match.
func (f *FS) Resolve(relPath string) ([]byte, error) {
	for i := len(tiers) - 1; i >= 0; i-- {
		path := filepath.Join(f.root, tiers[i], relPath)
		data, err := os.ReadFile(path)
		if err == nil {
			return data, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("tierfs: resolve %s in tier %s: %w", relPath, tiers[i], err)
		}
	}
	return nil, fmt.Errorf("tierfs: resolve %s: %w", relPath, os.ErrNotExist)
}

// ResolveAll iterates tiers lowest-to-highest so that higher-tier files
// overwrite lower-tier entries with the same filename.
func (f *FS) ResolveAll(subdir string) (map[string][]byte, error) {
	result := make(map[string][]byte)

	for _, tier := range tiers {
		dir := filepath.Join(f.root, tier, subdir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("tierfs: resolve-all %s in tier %s: %w", subdir, tier, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				return nil, fmt.Errorf("tierfs: resolve-all read %s/%s: %w", subdir, e.Name(), err)
			}
			result[e.Name()] = data
		}
	}

	return result, nil
}

// WriteBase writes data into the base tier, creating directories as needed.
func (f *FS) WriteBase(relPath string, data []byte) error {
	path := filepath.Join(f.root, "base", relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("tierfs: write-base mkdir %s: %w", relPath, err)
	}
	if err := atomicWriteFile(path, data); err != nil {
		return fmt.Errorf("tierfs: write-base %s: %w", relPath, err)
	}
	return nil
}

// atomicWriteFile writes data to path atomically via temp file + rename.
func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".wolfcastle-tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// BasePath returns the absolute path to subdir within the base tier.
func (f *FS) BasePath(subdir string) string {
	return filepath.Join(f.root, "base", subdir)
}

// TierDirs returns absolute paths in resolution order: base, custom, local.
func (f *FS) TierDirs() []string {
	dirs := make([]string, len(tiers))
	for i, t := range tiers {
		dirs[i] = filepath.Join(f.root, t)
	}
	return dirs
}
