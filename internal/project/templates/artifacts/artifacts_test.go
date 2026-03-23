package artifacts_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"text/template"
)

// ADR data mirrors the fields from cmd/adr_create.go
type ADRData struct {
	Title string
	Date  string
	Body  string
}

// SpecData mirrors the fields from cmd/spec.go
type SpecData struct {
	Title string
	Body  string
}

// TaskData mirrors the fields from cmd/task/add.go
type TaskData struct {
	Title string
	Body  string
}

// buildADROriginal reproduces cmd/adr_create.go:64-75
func buildADROriginal(title, date, body string) string {
	var content strings.Builder
	fmt.Fprintf(&content, "# %s\n\n", title)
	content.WriteString("## Status\nAccepted\n\n")
	fmt.Fprintf(&content, "## Date\n%s\n\n", date)

	if body != "" {
		content.WriteString(body)
	} else {
		content.WriteString("## Context\n\n[Why was this decision needed?]\n\n")
		content.WriteString("## Decision\n\n[What was decided?]\n\n")
		content.WriteString("## Consequences\n\n[What follows from this decision?]\n")
	}
	return content.String()
}

// buildSpecOriginal reproduces cmd/spec.go:81-86
func buildSpecOriginal(title, body string) string {
	if body != "" {
		return fmt.Sprintf("# %s\n\n%s\n", title, body)
	}
	return fmt.Sprintf("# %s\n\n[Spec content goes here.]\n", title)
}

// buildTaskOriginal reproduces cmd/task/add.go:192-200
func buildTaskOriginal(title, body string) string {
	var sb strings.Builder
	sb.WriteString("# " + title + "\n")
	if strings.TrimSpace(body) != "" {
		sb.WriteString("\n" + body + "\n")
	}
	return sb.String()
}

func TestADRTemplateEquivalence(t *testing.T) {
	tmpl := template.Must(template.ParseFiles("adr.md.tmpl"))

	tests := []struct {
		name  string
		title string
		date  string
		body  string
	}{
		{name: "with body", title: "Use PostgreSQL", date: "2026-03-22", body: "Custom body content here."},
		{name: "without body (default template)", title: "Switch to gRPC", date: "2026-01-15", body: ""},
		{name: "multiline body", title: "Cache Strategy", date: "2026-06-01", body: "## Context\nWe need caching.\n\n## Decision\nUse Redis.\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := buildADROriginal(tt.title, tt.date, tt.body)

			var buf bytes.Buffer
			err := tmpl.Execute(&buf, ADRData{Title: tt.title, Date: tt.date, Body: tt.body})
			if err != nil {
				t.Fatalf("template execution failed: %v", err)
			}

			if buf.String() != expected {
				t.Errorf("output mismatch\nexpected (%d bytes): %q\ngot      (%d bytes): %q",
					len(expected), expected, buf.Len(), buf.String())
			}
		})
	}
}

func TestSpecTemplateEquivalence(t *testing.T) {
	tmpl := template.Must(template.ParseFiles("spec.md.tmpl"))

	tests := []struct {
		name  string
		title string
		body  string
	}{
		{name: "with body", title: "API Auth Flow", body: "Authentication uses JWT tokens."},
		{name: "without body (placeholder)", title: "Empty Spec", body: ""},
		{name: "multiline body", title: "Data Model", body: "Users have roles.\nRoles have permissions."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := buildSpecOriginal(tt.title, tt.body)

			var buf bytes.Buffer
			err := tmpl.Execute(&buf, SpecData{Title: tt.title, Body: tt.body})
			if err != nil {
				t.Fatalf("template execution failed: %v", err)
			}

			if buf.String() != expected {
				t.Errorf("output mismatch\nexpected (%d bytes): %q\ngot      (%d bytes): %q",
					len(expected), expected, buf.Len(), buf.String())
			}
		})
	}
}

func TestTaskTemplateEquivalence(t *testing.T) {
	tmpl := template.Must(template.ParseFiles("task.md.tmpl"))

	tests := []struct {
		name  string
		title string
		body  string
	}{
		{name: "with body", title: "Implement auth", body: "Add JWT middleware to all routes."},
		{name: "without body", title: "Fix bug", body: ""},
		{name: "whitespace-only body", title: "Clean up", body: "   \n  \t  "},
		{name: "multiline body", title: "Add caching", body: "Layer 1: in-memory.\nLayer 2: Redis."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := buildTaskOriginal(tt.title, tt.body)

			// Mirror the TrimSpace check the call site would do
			body := tt.body
			if strings.TrimSpace(body) == "" {
				body = ""
			}

			var buf bytes.Buffer
			err := tmpl.Execute(&buf, TaskData{Title: tt.title, Body: body})
			if err != nil {
				t.Fatalf("template execution failed: %v", err)
			}

			if buf.String() != expected {
				t.Errorf("output mismatch\nexpected (%d bytes): %q\ngot      (%d bytes): %q",
					len(expected), expected, buf.Len(), buf.String())
			}
		})
	}
}
