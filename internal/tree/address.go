// Package tree provides address parsing, slug validation, and filesystem
// resolution for Wolfcastle's hierarchical project tree. Addresses are
// slash-separated kebab-case paths (e.g., "project/submodule/leaf") that
// map to per-node directories on disk.
package tree

import (
	"fmt"
	"strings"
	"unicode"
)

// Address represents a parsed tree address.
type Address struct {
	Parts []string
	Raw   string
}

// ParseAddress parses a slash-separated tree address.
func ParseAddress(s string) (Address, error) {
	if s == "" {
		return Address{Raw: ""}, nil
	}
	parts := strings.Split(s, "/")
	for _, p := range parts {
		if err := ValidateSlug(p); err != nil {
			return Address{}, fmt.Errorf("invalid address %q: %w", s, err)
		}
	}
	return Address{Parts: parts, Raw: s}, nil
}

// ValidateSlug checks that a slug is valid kebab-case.
func ValidateSlug(s string) error {
	if s == "" {
		return fmt.Errorf("slug cannot be empty")
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return fmt.Errorf("slug %q cannot start or end with a hyphen", s)
	}
	for i, r := range s {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' {
			if i > 0 && s[i-1] == '-' {
				return fmt.Errorf("slug %q has consecutive hyphens", s)
			}
			continue
		}
		if unicode.IsUpper(r) {
			return fmt.Errorf("slug %q contains uppercase character", s)
		}
		return fmt.Errorf("slug %q contains invalid character %q", s, r)
	}
	// Must start with a letter
	if s[0] < 'a' || s[0] > 'z' {
		return fmt.Errorf("slug %q must start with a letter", s)
	}
	return nil
}

// IsRoot returns true if the address is empty (root level).
func (a Address) IsRoot() bool {
	return len(a.Parts) == 0
}

// Parent returns the parent address, or empty if at root.
func (a Address) Parent() Address {
	if len(a.Parts) <= 1 {
		return Address{Raw: ""}
	}
	parts := a.Parts[:len(a.Parts)-1]
	return Address{Parts: parts, Raw: strings.Join(parts, "/")}
}

// Leaf returns the last segment of the address.
func (a Address) Leaf() string {
	if len(a.Parts) == 0 {
		return ""
	}
	return a.Parts[len(a.Parts)-1]
}

// Child returns a new address with the given slug appended.
func (a Address) Child(slug string) Address {
	parts := make([]string, len(a.Parts)+1)
	copy(parts, a.Parts)
	parts[len(a.Parts)] = slug
	return Address{Parts: parts, Raw: strings.Join(parts, "/")}
}

// String returns the raw address string.
func (a Address) String() string {
	return a.Raw
}

// ToSlug converts a name to a valid kebab-case slug.
func ToSlug(name string) string {
	var b strings.Builder
	prev := '-'
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			prev = r
		} else if prev != '-' {
			b.WriteRune('-')
			prev = '-'
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "unnamed"
	}
	return s
}

// MustParse parses an address or panics.
func MustParse(s string) Address {
	a, err := ParseAddress(s)
	if err != nil {
		panic(err)
	}
	return a
}

// SplitTaskAddress splits "node/path/task-N" into node address and task ID.
func SplitTaskAddress(s string) (nodeAddr string, taskID string, err error) {
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("task address %q must have at least node/task-id", s)
	}
	last := parts[len(parts)-1]
	if !strings.HasPrefix(last, "task-") && last != "audit" {
		return "", "", fmt.Errorf("task address %q: last segment must be task-N or audit", s)
	}
	nodeAddr = strings.Join(parts[:len(parts)-1], "/")
	return nodeAddr, last, nil
}
