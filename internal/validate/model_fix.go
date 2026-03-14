package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// modelFixResponse is the expected JSON output from the model.
type modelFixResponse struct {
	Resolution string `json:"resolution"`
	Reason     string `json:"reason"`
}

// TryModelAssistedFix invokes the configured doctor model to resolve an ambiguous issue.
// Returns true if the fix was applied successfully.
func TryModelAssistedFix(ctx context.Context, model config.ModelDef, issue Issue, projectsDir string) (bool, error) {
	if issue.Node == "" {
		return false, fmt.Errorf("model-assisted fix requires a node address")
	}

	prompt := fmt.Sprintf(`You are Wolfcastle Doctor, a structural repair agent.
An ambiguous state conflict has been found.

Node: %s
Issue: %s (%s)
Description: %s

The valid states are: not_started, in_progress, complete, blocked.

Output a JSON object with your resolution:
{"resolution": "not_started|in_progress|complete|blocked", "reason": "explanation"}

Output ONLY the JSON object, nothing else.`, issue.Node, issue.Category, issue.FixType, issue.Description)

	result, err := invoke.Invoke(ctx, model, prompt, projectsDir)
	if err != nil {
		return false, fmt.Errorf("model invocation failed: %w", err)
	}

	var resp modelFixResponse
	if err := json.Unmarshal([]byte(result.Stdout), &resp); err != nil {
		return false, fmt.Errorf("parsing model response: %w", err)
	}

	// Validate the resolution
	normalized, ok := NormalizeStateValue(resp.Resolution)
	if !ok {
		return false, fmt.Errorf("model returned invalid resolution: %q", resp.Resolution)
	}

	// Apply the fix
	a, err := tree.ParseAddress(issue.Node)
	if err != nil {
		return false, err
	}
	statePath := filepath.Join(projectsDir, filepath.Join(a.Parts...), "state.json")
	ns, err := state.LoadNodeState(statePath)
	if err != nil {
		return false, err
	}

	ns.State = normalized
	if err := state.SaveNodeState(statePath, ns); err != nil {
		return false, err
	}

	return true, nil
}
