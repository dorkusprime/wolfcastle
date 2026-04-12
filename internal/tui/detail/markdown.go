package detail

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/x/ansi"
)

// renderMarkdown renders markdown text using glamour. Falls back to
// plain text with basic word wrapping if glamour fails.
func renderMarkdown(text string, width int) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return wrapIndent(text, width, "  ")
	}
	out, err := r.Render(text)
	if err != nil {
		return wrapIndent(text, width, "  ")
	}
	return trimBlankLines(out)
}

// trimBlankLines removes leading and trailing lines that contain only
// whitespace once ANSI escapes are stripped. Glamour's block elements
// emit empty-but-styled pad lines around paragraphs and lists; those
// lines look blank but survive strings.Trim.
func trimBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(ansi.Strip(lines[start])) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(ansi.Strip(lines[end-1])) == "" {
		end--
	}
	return strings.Join(lines[start:end], "\n")
}

// renderMarkdownList renders a slice of strings as a markdown bullet
// list. Each item becomes a `- item` line, then the whole thing is
// rendered through glamour. Item text is escaped so that markdown
// meta characters (underscores in file paths, asterisks, backticks)
// render literally instead of triggering emphasis or code spans.
func renderMarkdownList(items []string, width int) string {
	var md strings.Builder
	for _, item := range items {
		md.WriteString("- ")
		md.WriteString(escapeMarkdown(item))
		md.WriteByte('\n')
	}
	return renderMarkdown(md.String(), width)
}

// escapeMarkdown backslash-escapes the characters goldmark treats as
// markdown syntax, so task field strings containing things like
// file_name.go or a*b render verbatim.
func escapeMarkdown(s string) string {
	const meta = "\\`*_{}[]()#+-.!|<>"
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if strings.ContainsRune(meta, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
