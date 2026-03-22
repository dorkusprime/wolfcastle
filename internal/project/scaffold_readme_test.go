package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldService_Init_CreatesScaffoldFiles(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	for tmpl, dest := range scaffoldFiles {
		wantContent, err := Templates.ReadFile(tmpl)
		if err != nil {
			t.Fatalf("reading embedded template %s: %v", tmpl, err)
		}
		data, err := os.ReadFile(filepath.Join(root, dest))
		if err != nil {
			t.Errorf("%s should exist: %v", dest, err)
			continue
		}
		if string(data) != string(wantContent) {
			t.Errorf("%s content mismatch:\ngot:\n%s\nwant:\n%s", dest, string(data), string(wantContent))
		}
	}
}

func TestScaffoldService_Init_READMEsHaveExpectedContent(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path     string
		contains []string
	}{
		{
			path:     "README.md",
			contains: []string{"system/", "docs/", "archive/", "artifacts/"},
		},
		{
			path:     "system/README.md",
			contains: []string{"base/", "custom/", "local/", "three tiers"},
		},
		{
			path:     "system/base/prompts/README.md",
			contains: []string{"system/custom/prompts/", "system/local/prompts/", "override"},
		},
		{
			path:     "docs/README.md",
			contains: []string{"decisions/", "specs/", "ADR"},
		},
		{
			path:     "archive/README.md",
			contains: []string{"Completed projects", "system/projects/"},
		},
	}

	for _, tt := range tests {
		data, err := os.ReadFile(filepath.Join(root, tt.path))
		if err != nil {
			t.Errorf("%s should exist: %v", tt.path, err)
			continue
		}
		content := string(data)
		for _, substr := range tt.contains {
			if !strings.Contains(content, substr) {
				t.Errorf("%s should contain %q", tt.path, substr)
			}
		}
	}
}

func TestScaffoldService_Init_GitignoreTracksREADMEs(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(".gitignore should exist:", err)
	}

	content := string(data)
	// The gitignore uses explicit excludes. Verify key exclusions are present.
	requiredPatterns := []string{
		"system/base/",
		"system/local/",
		"system/logs/",
	}
	for _, pattern := range requiredPatterns {
		if !strings.Contains(content, pattern) {
			t.Errorf(".gitignore should contain %q to exclude non-tracked directories", pattern)
		}
	}
}
