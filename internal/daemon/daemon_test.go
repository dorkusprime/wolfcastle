package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/inbox"
)

// --- parseExpandedSections ---

func TestParseExpandedSections_MultipleSections(t *testing.T) {
	t.Parallel()
	input := `Some preamble text
## Item 1
Content for item one
More content
## Item 2
Content for item two
## Item 3
Content for item three`

	sections := parseExpandedSections(input)
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}
	if sections[0] != "## Item 1\nContent for item one\nMore content" {
		t.Errorf("unexpected section 0: %q", sections[0])
	}
	if sections[1] != "## Item 2\nContent for item two" {
		t.Errorf("unexpected section 1: %q", sections[1])
	}
	if sections[2] != "## Item 3\nContent for item three" {
		t.Errorf("unexpected section 2: %q", sections[2])
	}
}

func TestParseExpandedSections_NoSections(t *testing.T) {
	t.Parallel()
	input := "Just some plain text without any headings"
	sections := parseExpandedSections(input)
	if len(sections) != 0 {
		t.Errorf("expected 0 sections, got %d", len(sections))
	}
}

func TestParseExpandedSections_SingleSection(t *testing.T) {
	t.Parallel()
	input := "## Only Section\nSome content here"
	sections := parseExpandedSections(input)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0] != "## Only Section\nSome content here" {
		t.Errorf("unexpected section: %q", sections[0])
	}
}

func TestParseExpandedSections_EmptyInput(t *testing.T) {
	t.Parallel()
	sections := parseExpandedSections("")
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty input, got %d", len(sections))
	}
}

func TestParseExpandedSections_ConsecutiveHeadings(t *testing.T) {
	t.Parallel()
	input := "## First\n## Second\nContent"
	sections := parseExpandedSections(input)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if sections[0] != "## First" {
		t.Errorf("unexpected section 0: %q", sections[0])
	}
	if sections[1] != "## Second\nContent" {
		t.Errorf("unexpected section 1: %q", sections[1])
	}
}

func TestParseExpandedSections_PreambleOnly(t *testing.T) {
	t.Parallel()
	input := "Some text\nMore text\nNo headings"
	sections := parseExpandedSections(input)
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for preamble-only input, got %d", len(sections))
	}
}

func TestParseExpandedSections_HeadingAtEnd(t *testing.T) {
	t.Parallel()
	input := "preamble\n## Trailing"
	sections := parseExpandedSections(input)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0] != "## Trailing" {
		t.Errorf("unexpected section: %q", sections[0])
	}
}

// --- dedupPipe ---

func TestDedupPipe_BasicDedup(t *testing.T) {
	t.Parallel()
	result := dedupPipe("a|b|a|c|b")
	if len(result) != 3 {
		t.Fatalf("expected 3 unique items, got %d: %v", len(result), result)
	}
	expected := []string{"a", "b", "c"}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("index %d: expected %q, got %q", i, v, result[i])
		}
	}
}

func TestDedupPipe_EmptyParts(t *testing.T) {
	t.Parallel()
	result := dedupPipe("a||b|||c")
	if len(result) != 3 {
		t.Fatalf("expected 3 items (empty parts skipped), got %d: %v", len(result), result)
	}
}

func TestDedupPipe_WhitespaceHandling(t *testing.T) {
	t.Parallel()
	result := dedupPipe("  a  | b |  a  | c ")
	if len(result) != 3 {
		t.Fatalf("expected 3 items (whitespace trimmed and deduped), got %d: %v", len(result), result)
	}
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestDedupPipe_EmptyString(t *testing.T) {
	t.Parallel()
	result := dedupPipe("")
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestDedupPipe_SingleItem(t *testing.T) {
	t.Parallel()
	result := dedupPipe("only")
	if len(result) != 1 || result[0] != "only" {
		t.Errorf("expected [only], got %v", result)
	}
}

func TestDedupPipe_AllWhitespace(t *testing.T) {
	t.Parallel()
	result := dedupPipe("  |  |  ")
	if len(result) != 0 {
		t.Errorf("expected empty result for all-whitespace, got %v", result)
	}
}

// --- checkInboxState ---

func TestCheckInboxState_MissingFile(t *testing.T) {
	t.Parallel()
	d := &Daemon{}
	hasNew, hasExpanded := d.checkInboxState("/nonexistent/path/inbox.json")
	if hasNew || hasExpanded {
		t.Error("expected false, false for missing file")
	}
}

func TestCheckInboxState_EmptyInbox(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")

	inboxData := &inbox.File{Items: []inbox.Item{}}
	if err := inbox.Save(inboxPath, inboxData); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{}
	hasNew, hasExpanded := d.checkInboxState(inboxPath)
	if hasNew || hasExpanded {
		t.Error("expected false, false for empty inbox")
	}
}

func TestCheckInboxState_NewItemsOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")

	inboxData := &inbox.File{
		Items: []inbox.Item{
			{Timestamp: "2026-03-14T00:00:00Z", Text: "new thing", Status: "new"},
			{Timestamp: "2026-03-14T00:01:00Z", Text: "filed thing", Status: "filed"},
		},
	}
	if err := inbox.Save(inboxPath, inboxData); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{}
	hasNew, hasExpanded := d.checkInboxState(inboxPath)
	if !hasNew {
		t.Error("expected hasNew=true")
	}
	if hasExpanded {
		t.Error("expected hasExpanded=false")
	}
}

func TestCheckInboxState_ExpandedItemsOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")

	inboxData := &inbox.File{
		Items: []inbox.Item{
			{Timestamp: "2026-03-14T00:00:00Z", Text: "expanded thing", Status: "expanded", Expanded: "details"},
		},
	}
	if err := inbox.Save(inboxPath, inboxData); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{}
	hasNew, hasExpanded := d.checkInboxState(inboxPath)
	if hasNew {
		t.Error("expected hasNew=false")
	}
	if !hasExpanded {
		t.Error("expected hasExpanded=true")
	}
}

func TestCheckInboxState_BothNewAndExpanded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")

	inboxData := &inbox.File{
		Items: []inbox.Item{
			{Timestamp: "2026-03-14T00:00:00Z", Text: "new thing", Status: "new"},
			{Timestamp: "2026-03-14T00:01:00Z", Text: "expanded thing", Status: "expanded"},
		},
	}
	if err := inbox.Save(inboxPath, inboxData); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{}
	hasNew, hasExpanded := d.checkInboxState(inboxPath)
	if !hasNew {
		t.Error("expected hasNew=true")
	}
	if !hasExpanded {
		t.Error("expected hasExpanded=true")
	}
}

func TestCheckInboxState_AllFiled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")

	inboxData := &inbox.File{
		Items: []inbox.Item{
			{Timestamp: "2026-03-14T00:00:00Z", Text: "filed thing", Status: "filed"},
			{Timestamp: "2026-03-14T00:01:00Z", Text: "also filed", Status: "filed"},
		},
	}
	if err := inbox.Save(inboxPath, inboxData); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{}
	hasNew, hasExpanded := d.checkInboxState(inboxPath)
	if hasNew || hasExpanded {
		t.Error("expected false, false when all items are filed")
	}
}

func TestCheckInboxState_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inboxPath := filepath.Join(dir, "inbox.json")

	if err := os.WriteFile(inboxPath, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{}
	hasNew, hasExpanded := d.checkInboxState(inboxPath)
	if hasNew || hasExpanded {
		t.Error("expected false, false for invalid JSON")
	}
}
