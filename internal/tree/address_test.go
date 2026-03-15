package tree

import (
	"testing"
)

func TestParseAddress_ValidSimpleSlug(t *testing.T) {
	t.Parallel()
	addr, err := ParseAddress("my-node")
	if err != nil {
		t.Fatal(err)
	}
	if len(addr.Parts) != 1 || addr.Parts[0] != "my-node" {
		t.Errorf("expected [my-node], got %v", addr.Parts)
	}
	if addr.Raw != "my-node" {
		t.Errorf("expected raw 'my-node', got %q", addr.Raw)
	}
}

func TestParseAddress_ValidMultiSegment(t *testing.T) {
	t.Parallel()
	addr, err := ParseAddress("root/child/leaf")
	if err != nil {
		t.Fatal(err)
	}
	if len(addr.Parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(addr.Parts))
	}
	if addr.Parts[0] != "root" || addr.Parts[1] != "child" || addr.Parts[2] != "leaf" {
		t.Errorf("unexpected parts %v", addr.Parts)
	}
}

func TestValidateSlug_InvalidEmpty(t *testing.T) {
	t.Parallel()
	if err := ValidateSlug(""); err == nil {
		t.Error("expected error for empty slug")
	}
}

func TestValidateSlug_InvalidUppercase(t *testing.T) {
	t.Parallel()
	if err := ValidateSlug("MyNode"); err == nil {
		t.Error("expected error for uppercase")
	}
}

func TestValidateSlug_InvalidConsecutiveHyphens(t *testing.T) {
	t.Parallel()
	if err := ValidateSlug("my--node"); err == nil {
		t.Error("expected error for consecutive hyphens")
	}
}

func TestValidateSlug_InvalidStartingWithNumber(t *testing.T) {
	t.Parallel()
	if err := ValidateSlug("1node"); err == nil {
		t.Error("expected error for starting with number")
	}
}

func TestValidateSlug_InvalidStartingWithHyphen(t *testing.T) {
	t.Parallel()
	if err := ValidateSlug("-node"); err == nil {
		t.Error("expected error for starting with hyphen")
	}
}

func TestValidateSlug_ValidKebabCase(t *testing.T) {
	t.Parallel()
	if err := ValidateSlug("my-cool-node"); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
}

func TestValidateSlug_ValidWithNumbers(t *testing.T) {
	t.Parallel()
	if err := ValidateSlug("node-v2"); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
}

func TestToSlug_ConvertsNamesCorrectly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"My Cool Feature", "my-cool-feature"},
		{"UPPERCASE", "uppercase"},
		{"with  spaces", "with-spaces"},
		{"special!@#chars", "special-chars"},
		{"", "unnamed"},
		{"---", "unnamed"},
		{"Already-Kebab", "already-kebab"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := ToSlug(tt.input)
			if got != tt.want {
				t.Errorf("ToSlug(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitTaskAddress_TaskN(t *testing.T) {
	t.Parallel()
	nodeAddr, taskID, err := SplitTaskAddress("root/leaf/task-0001")
	if err != nil {
		t.Fatal(err)
	}
	if nodeAddr != "root/leaf" {
		t.Errorf("expected 'root/leaf', got %q", nodeAddr)
	}
	if taskID != "task-0001" {
		t.Errorf("expected 'task-1', got %q", taskID)
	}
}

func TestSplitTaskAddress_Audit(t *testing.T) {
	t.Parallel()
	nodeAddr, taskID, err := SplitTaskAddress("root/leaf/audit")
	if err != nil {
		t.Fatal(err)
	}
	if nodeAddr != "root/leaf" {
		t.Errorf("expected 'root/leaf', got %q", nodeAddr)
	}
	if taskID != "audit" {
		t.Errorf("expected 'audit', got %q", taskID)
	}
}

func TestSplitTaskAddress_FailsForNonTaskSuffix(t *testing.T) {
	t.Parallel()
	_, _, err := SplitTaskAddress("root/leaf/something")
	if err == nil {
		t.Error("expected error for non-task suffix")
	}
}

func TestSplitTaskAddress_FailsForTooShort(t *testing.T) {
	t.Parallel()
	_, _, err := SplitTaskAddress("task-0001")
	if err == nil {
		t.Error("expected error for single-segment address")
	}
}

func TestAddress_Parent(t *testing.T) {
	t.Parallel()
	addr := MustParse("root/child/leaf")
	parent := addr.Parent()
	if parent.Raw != "root/child" {
		t.Errorf("expected 'root/child', got %q", parent.Raw)
	}

	// Parent of single-segment is root
	single := MustParse("node")
	p := single.Parent()
	if p.Raw != "" {
		t.Errorf("expected empty parent, got %q", p.Raw)
	}
}

func TestAddress_Child(t *testing.T) {
	t.Parallel()
	addr := MustParse("root")
	child := addr.Child("leaf")
	if child.Raw != "root/leaf" {
		t.Errorf("expected 'root/leaf', got %q", child.Raw)
	}
}

func TestAddress_Leaf(t *testing.T) {
	t.Parallel()
	addr := MustParse("root/child/leaf")
	if addr.Leaf() != "leaf" {
		t.Errorf("expected 'leaf', got %q", addr.Leaf())
	}

	empty := Address{}
	if empty.Leaf() != "" {
		t.Errorf("expected empty leaf, got %q", empty.Leaf())
	}
}

func TestAddress_IsRoot(t *testing.T) {
	t.Parallel()
	addr, _ := ParseAddress("")
	if !addr.IsRoot() {
		t.Error("empty address should be root")
	}

	nonRoot := MustParse("node")
	if nonRoot.IsRoot() {
		t.Error("non-empty address should not be root")
	}
}
