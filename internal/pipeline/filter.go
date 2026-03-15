package pipeline

import (
	"strings"
)

// FilterScriptReference returns a filtered version of the script reference
// markdown, keeping only command blocks whose names appear in allowed.
// Command blocks are delimited by "### wolfcastle <name>" headers, where
// <name> is matched against the allowed list (e.g. "task add", "status").
//
// The preamble (everything before the first ## or ### header) is always kept.
// Section headers (## lines) are kept only when at least one command beneath
// them survives the filter. An empty allowed list returns the input unchanged.
func FilterScriptReference(content string, allowed []string) string {
	if len(allowed) == 0 {
		return content
	}

	allowedSet := make(map[string]bool, len(allowed))
	for _, cmd := range allowed {
		allowedSet[cmd] = true
	}

	lines := strings.Split(content, "\n")

	// Parse into a sequence of blocks. Each block is either:
	//   - preamble (before the first ## or ###)
	//   - a section header (## line plus any text before the first ### under it)
	//   - a command block (### wolfcastle <name> through end of block)
	type block struct {
		kind    string // "preamble", "section", "command"
		cmdName string // only set for "command" blocks
		lines   []string
	}

	var blocks []block
	var current block
	current.kind = "preamble"

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "### wolfcastle ") {
			// Flush current block
			if len(current.lines) > 0 || current.kind == "preamble" {
				blocks = append(blocks, current)
			}
			// Extract command name: "### wolfcastle task add" -> "task add"
			name := strings.TrimPrefix(trimmed, "### wolfcastle ")
			current = block{kind: "command", cmdName: name, lines: []string{line}}
			continue
		}

		if strings.HasPrefix(trimmed, "## ") {
			// Flush current block
			if len(current.lines) > 0 || current.kind == "preamble" {
				blocks = append(blocks, current)
			}
			current = block{kind: "section", lines: []string{line}}
			continue
		}

		current.lines = append(current.lines, line)
	}
	// Flush final block
	if len(current.lines) > 0 || current.kind == "preamble" {
		blocks = append(blocks, current)
	}

	// Build output: keep preamble, keep commands that match, keep sections
	// only if they have at least one matching command after them.
	var out []string

	for i, b := range blocks {
		switch b.kind {
		case "preamble":
			out = append(out, strings.Join(b.lines, "\n"))

		case "command":
			if allowedSet[b.cmdName] {
				out = append(out, strings.Join(b.lines, "\n"))
			}

		case "section":
			// Look ahead: does any command block between this section and
			// the next section (or end) survive the filter?
			hasChild := false
			for j := i + 1; j < len(blocks); j++ {
				if blocks[j].kind == "section" {
					break
				}
				if blocks[j].kind == "command" && allowedSet[blocks[j].cmdName] {
					hasChild = true
					break
				}
			}
			if hasChild {
				out = append(out, strings.Join(b.lines, "\n"))
			}
		}
	}

	return strings.Join(out, "\n")
}
