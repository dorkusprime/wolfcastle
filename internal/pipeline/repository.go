package pipeline

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// PromptRepository provides three-tier prompt and fragment resolution backed
// by a tierfs.Resolver. It replaces the standalone ResolveFragment,
// ResolveAllFragments, and ResolvePromptTemplate functions with a
// struct-based design.
type PromptRepository struct {
	tiers tierfs.Resolver
}

// DefaultCacheTTL is the TTL applied to the caching resolver that wraps
// tier resolution in production repositories. 30 seconds balances freshness
// with avoiding redundant disk reads during a daemon iteration.
const DefaultCacheTTL = 30 * time.Second

// NewPromptRepository creates a PromptRepository rooted at wolfcastleRoot.
// The tierfs.FS is constructed over the "system" subdirectory and wrapped
// with a CachingResolver for TTL-based read caching.
func NewPromptRepository(wolfcastleRoot string) *PromptRepository {
	fs := tierfs.New(filepath.Join(wolfcastleRoot, tierfs.SystemPrefix))
	return NewPromptRepositoryWithTiers(
		tierfs.NewCachingResolver(fs, DefaultCacheTTL),
	)
}

// NewPromptRepositoryWithTiers creates a PromptRepository with an injected
// resolver, allowing tests to supply fixture-backed implementations.
func NewPromptRepositoryWithTiers(tiers tierfs.Resolver) *PromptRepository {
	return &PromptRepository{tiers: tiers}
}

// Resolve resolves a prompt template by short name and optionally executes
// it as a Go text/template. The name is relative to "prompts/" without the
// .md extension. If ctx is nil, the raw content is returned.
func (r *PromptRepository) Resolve(name string, ctx any) (string, error) {
	path := "prompts/" + name + ".md"
	data, err := r.tiers.Resolve(path)
	if err != nil {
		return "", fmt.Errorf("prompts: resolve %s: %w", name, err)
	}
	raw := string(data)
	if ctx == nil {
		return raw, nil
	}
	tmpl, err := template.New(name).Parse(raw)
	if err != nil {
		return "", fmt.Errorf("prompts: parse template %s: %w", name, err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("prompts: execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

// ResolveTemplate resolves a template file by short name and optionally
// executes it as a Go text/template. The name is relative to "templates/"
// and uses the .tmpl extension. If ctx is non-nil, the content is parsed
// and executed as a template; otherwise the raw content is returned.
func (r *PromptRepository) ResolveTemplate(name string, ctx any) (string, error) {
	path := name + ".tmpl"
	data, err := r.tiers.Resolve(path)
	if err != nil {
		return "", fmt.Errorf("templates: resolve %s: %w", name, err)
	}
	raw := string(data)
	if ctx == nil {
		return raw, nil
	}
	tmpl, err := template.New(name).Parse(raw)
	if err != nil {
		return "", fmt.Errorf("templates: parse template %s: %w", name, err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("templates: execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

// RenderToFile resolves a template by name, executes it with data, and writes
// the result to destPath. Parent directories are created as needed. The file
// is written with 0644 permissions.
func (r *PromptRepository) RenderToFile(tmplName string, data any, destPath string) error {
	content, err := r.ResolveTemplate(tmplName, data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("templates: mkdir for %s: %w", destPath, err)
	}
	if err := os.WriteFile(destPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("templates: write %s: %w", destPath, err)
	}
	return nil
}

// ResolveRaw resolves raw file content by category and filename with no
// template processing.
func (r *PromptRepository) ResolveRaw(category, name string) (string, error) {
	path := category + "/" + name
	data, err := r.tiers.Resolve(path)
	if err != nil {
		return "", fmt.Errorf("prompts: resolve-raw %s/%s: %w", category, name, err)
	}
	return string(data), nil
}

// ListFragments collects all .md fragments in a category across tiers,
// applies include/exclude filtering, and returns their contents sorted
// by filename.
func (r *PromptRepository) ListFragments(category string, include, exclude []string) ([]string, error) {
	files, err := r.tiers.ResolveAll(category)
	if err != nil {
		return nil, fmt.Errorf("prompts: list-fragments %s: %w", category, err)
	}

	excludeSet := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		excludeSet[e] = true
	}

	var names []string
	if len(include) > 0 {
		names = include
	} else {
		names = make([]string, 0, len(files))
		for name := range files {
			names = append(names, name)
		}
		sort.Strings(names)
	}

	var contents []string
	for _, name := range names {
		if excludeSet[name] {
			continue
		}
		data, ok := files[name]
		if !ok {
			if len(include) > 0 {
				return nil, fmt.Errorf("prompts: fragment %q specified in include list not found in any tier", name)
			}
			continue
		}
		contents = append(contents, string(data))
	}

	return contents, nil
}

// WriteBase writes a single file to the base tier, creating parent
// directories as needed.
func (r *PromptRepository) WriteBase(relPath string, data []byte) error {
	if err := r.tiers.WriteBase(relPath, data); err != nil {
		return fmt.Errorf("prompts: write-base %s: %w", relPath, err)
	}
	return nil
}

// WriteAllBase walks an fs.FS and writes each file to the base tier.
// Used during scaffold to seed default prompts from embedded templates.
func (r *PromptRepository) WriteAllBase(templates fs.FS) error {
	return fs.WalkDir(templates, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("prompts: walk %s: %w", path, err)
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(templates, path)
		if err != nil {
			return fmt.Errorf("prompts: read embedded %s: %w", path, err)
		}
		if err := r.tiers.WriteBase(path, data); err != nil {
			return fmt.Errorf("prompts: write-base %s: %w", path, err)
		}
		return nil
	})
}
