package config

import (
	"fmt"
	"strings"
)

// ParsePath splits a dot-delimited path into segments, validating that no
// segment is empty and no segment contains array indexing syntax (brackets).
func ParsePath(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	segments := strings.Split(path, ".")
	for _, seg := range segments {
		if seg == "" {
			return nil, fmt.Errorf("path %q contains an empty segment", path)
		}
		if strings.ContainsAny(seg, "[]") {
			return nil, fmt.Errorf("path segment %q contains array indexing syntax; arrays are not supported", seg)
		}
	}
	return segments, nil
}

// GetPath walks a dot-delimited path through nested maps, returning the value
// and true if found, or (nil, false) if any segment is missing or a non-map
// intermediate is encountered.
func GetPath(m map[string]any, path string) (any, bool) {
	segments, err := ParsePath(path)
	if err != nil {
		return nil, false
	}
	var current any = m
	for _, seg := range segments {
		cm, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = cm[seg]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// SetPath walks a dot-delimited path through nested maps, creating intermediate
// maps as needed, and sets the final key to value. Returns an error if an
// intermediate value exists but is not a map.
func SetPath(m map[string]any, path string, value any) error {
	segments, err := ParsePath(path)
	if err != nil {
		return err
	}
	current := m
	for _, seg := range segments[:len(segments)-1] {
		next, exists := current[seg]
		if !exists {
			child := make(map[string]any)
			current[seg] = child
			current = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("path segment %q exists but is %T, not a map", seg, next)
		}
		current = child
	}
	current[segments[len(segments)-1]] = value
	return nil
}

// DeletePath walks a dot-delimited path and sets the final key to nil,
// using null-deletion semantics compatible with DeepMerge. Returns an error
// if an intermediate segment exists but is not a map.
func DeletePath(m map[string]any, path string) error {
	segments, err := ParsePath(path)
	if err != nil {
		return err
	}
	current := m
	for _, seg := range segments[:len(segments)-1] {
		next, exists := current[seg]
		if !exists {
			// Path doesn't exist; nothing to delete.
			return nil
		}
		child, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("path segment %q exists but is %T, not a map", seg, next)
		}
		current = child
	}
	current[segments[len(segments)-1]] = nil
	return nil
}
