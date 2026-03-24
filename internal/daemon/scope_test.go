package daemon

import (
	"strings"
	"testing"
)

func TestValidateScope(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		taskScope   []string
		otherScopes [][]string
		wantIn      []string
		wantUnowned []string
	}{
		{
			name:        "empty status produces no results",
			status:      "",
			taskScope:   []string{"internal/daemon/scope.go"},
			wantIn:      nil,
			wantUnowned: nil,
		},
		{
			name:      "exact file match lands in scope",
			status:    " M internal/daemon/scope.go\n",
			taskScope: []string{"internal/daemon/scope.go"},
			wantIn:    []string{"internal/daemon/scope.go"},
		},
		{
			name:        "file not in any scope is unowned",
			status:      "?? stray.txt\n",
			taskScope:   []string{"internal/daemon/scope.go"},
			otherScopes: [][]string{{"cmd/main.go"}},
			wantUnowned: []string{"stray.txt"},
		},
		{
			name:        "file in another task scope is excluded from both slices",
			status:      " M cmd/main.go\n",
			taskScope:   []string{"internal/daemon/scope.go"},
			otherScopes: [][]string{{"cmd/main.go"}},
			wantIn:      nil,
			wantUnowned: nil,
		},
		{
			name:      "directory prefix scope matches nested files",
			status:    " M internal/daemon/scope.go\n M internal/daemon/iteration.go\n",
			taskScope: []string{"internal/daemon/"},
			wantIn:    []string{"internal/daemon/scope.go", "internal/daemon/iteration.go"},
		},
		{
			name:        "directory prefix in other scope matches nested files",
			status:      " M cmd/server/main.go\n",
			taskScope:   []string{"internal/daemon/"},
			otherScopes: [][]string{{"cmd/"}},
			wantIn:      nil,
			wantUnowned: nil,
		},
		{
			name:        "wolfcastle state files are excluded",
			status:      " M .wolfcastle/state.json\n M internal/daemon/scope.go\n?? .wolfcastle/logs/run.log\n",
			taskScope:   []string{"internal/daemon/scope.go"},
			wantIn:      []string{"internal/daemon/scope.go"},
			wantUnowned: nil,
		},
		{
			name:        "rename paths use destination",
			status:      "R  old.go -> internal/daemon/scope.go\n",
			taskScope:   []string{"internal/daemon/scope.go"},
			wantIn:      []string{"internal/daemon/scope.go"},
			wantUnowned: nil,
		},
		{
			name:        "mixed classification across all three buckets",
			status:      " M internal/daemon/scope.go\n M cmd/main.go\n?? rogue.txt\n M .wolfcastle/system/config.json\n",
			taskScope:   []string{"internal/daemon/scope.go"},
			otherScopes: [][]string{{"cmd/main.go"}},
			wantIn:      []string{"internal/daemon/scope.go"},
			wantUnowned: []string{"rogue.txt"},
		},
		{
			name:        "multiple other scopes checked",
			status:      " M pkg/auth/token.go\n M pkg/cache/lru.go\n",
			taskScope:   []string{"internal/daemon/"},
			otherScopes: [][]string{{"pkg/auth/"}, {"pkg/cache/"}},
			wantIn:      nil,
			wantUnowned: nil,
		},
		{
			name:        "exact scope entry does not match subdirectory",
			status:      " M internal/daemon/scope.go\n",
			taskScope:   []string{"internal/daemon"},
			wantUnowned: []string{"internal/daemon/scope.go"},
		},
		{
			name:        "empty task scope makes all non-wolfcastle files unowned",
			status:      " M foo.go\n M bar.go\n",
			taskScope:   nil,
			otherScopes: nil,
			wantUnowned: []string{"foo.go", "bar.go"},
		},
		{
			name:        "overlapping scopes prefer task scope over other scopes",
			status:      " M shared.go\n",
			taskScope:   []string{"shared.go"},
			otherScopes: [][]string{{"shared.go"}},
			wantIn:      []string{"shared.go"},
			wantUnowned: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIn, gotUnowned := validateScope(tt.status, tt.taskScope, tt.otherScopes)
			if !slicesEqual(gotIn, tt.wantIn) {
				t.Errorf("inScope = %v, want %v", gotIn, tt.wantIn)
			}
			if !slicesEqual(gotUnowned, tt.wantUnowned) {
				t.Errorf("unowned = %v, want %v", gotUnowned, tt.wantUnowned)
			}
		})
	}
}

func TestMatchesScope(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		scope []string
		want  bool
	}{
		{name: "exact match", path: "foo.go", scope: []string{"foo.go"}, want: true},
		{name: "no match", path: "bar.go", scope: []string{"foo.go"}, want: false},
		{name: "dir prefix match", path: "pkg/auth/token.go", scope: []string{"pkg/auth/"}, want: true},
		{name: "dir prefix no match", path: "pkg/cache/lru.go", scope: []string{"pkg/auth/"}, want: false},
		{name: "empty scope", path: "foo.go", scope: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesScope(tt.path, tt.scope); got != tt.want {
				t.Errorf("matchesScope(%q, %v) = %v, want %v", tt.path, tt.scope, got, tt.want)
			}
		})
	}
}

// slicesEqual compares two string slices, treating nil and empty as equivalent.
func slicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}
