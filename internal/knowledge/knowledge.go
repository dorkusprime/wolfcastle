package knowledge

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// FilePath returns the path to the knowledge file for the given namespace.
func FilePath(wolfcastleDir, namespace string) string {
	return filepath.Join(wolfcastleDir, "docs", "knowledge", namespace+".md")
}

// Read returns the contents of the knowledge file for the given namespace.
// If the file does not exist, it returns an empty string and no error.
func Read(wolfcastleDir, namespace string) (string, error) {
	data, err := os.ReadFile(FilePath(wolfcastleDir, namespace))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("reading knowledge file: %w", err)
	}
	return string(data), nil
}

// Append adds a markdown bullet entry to the knowledge file for the given
// namespace, creating the file and parent directories if needed. If the entry
// lacks a "- " prefix, one is added. A trailing newline is ensured.
func Append(wolfcastleDir, namespace, entry string) error {
	p := FilePath(wolfcastleDir, namespace)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("creating knowledge directory: %w", err)
	}

	line := entry
	if !strings.HasPrefix(line, "- ") {
		line = "- " + line
	}
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}

	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening knowledge file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("writing knowledge entry: %w", err)
	}
	return nil
}

// TokenCount estimates the token count for the given content using a simple
// word-based heuristic: words / 0.75. This yields a conservative estimate
// (more tokens than words) consistent with rough LLM tokenization.
func TokenCount(content string) int {
	words := len(strings.Fields(content))
	if words == 0 {
		return 0
	}
	return int(math.Ceil(float64(words) / 0.75))
}

// CheckBudget verifies that appending newEntry to the knowledge file for the
// given namespace would not exceed maxTokens. Returns nil if within budget,
// or a descriptive error if the combined content would exceed the limit.
func CheckBudget(wolfcastleDir, namespace string, maxTokens int, newEntry string) error {
	existing, err := Read(wolfcastleDir, namespace)
	if err != nil {
		return err
	}

	combined := existing + newEntry
	count := TokenCount(combined)
	if count > maxTokens {
		return fmt.Errorf("knowledge file exceeds budget (%d/%d tokens). Run `wolfcastle knowledge prune` to review and consolidate", count, maxTokens)
	}
	return nil
}
