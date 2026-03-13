package project

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Scaffold creates the .wolfcastle/ directory structure for wolfcastle init.
func Scaffold(wolfcastleDir string) error {
	dirs := []string{
		"base/prompts",
		"base/rules",
		"base/audits",
		"custom",
		"local",
		"archive",
		"docs/decisions",
		"docs/specs",
		"logs",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(wolfcastleDir, d), 0755); err != nil {
			return err
		}
	}

	// Write .gitignore
	gitignore := `*
!.gitignore
!config.json
!custom/
!custom/**
!projects/
!projects/**
!archive/
!archive/**
!docs/
!docs/**
`
	if err := os.WriteFile(filepath.Join(wolfcastleDir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		return err
	}

	// Write default config.json
	cfg := map[string]any{}
	cfgData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	cfgData = append(cfgData, '\n')
	if err := os.WriteFile(filepath.Join(wolfcastleDir, "config.json"), cfgData, 0644); err != nil {
		return err
	}

	// Write config.local.json with identity
	identity := detectIdentity()
	localCfg := map[string]any{
		"identity": identity,
	}
	localData, err := json.MarshalIndent(localCfg, "", "  ")
	if err != nil {
		return err
	}
	localData = append(localData, '\n')
	if err := os.WriteFile(filepath.Join(wolfcastleDir, "config.local.json"), localData, 0644); err != nil {
		return err
	}

	// Create engineer namespace directory with empty root index
	ns := identity["user"].(string) + "-" + identity["machine"].(string)
	nsDir := filepath.Join(wolfcastleDir, "projects", ns)
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		return err
	}

	idx := state.NewRootIndex()
	idxData, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	idxData = append(idxData, '\n')
	if err := os.WriteFile(filepath.Join(nsDir, "state.json"), idxData, 0644); err != nil {
		return err
	}

	// Write base prompt files
	WriteBasePrompts(wolfcastleDir)

	return nil
}

func detectIdentity() map[string]any {
	user := "unknown"
	machine := "unknown"

	if u, err := exec.Command("whoami").Output(); err == nil {
		user = strings.TrimSpace(string(u))
	}
	if h, err := os.Hostname(); err == nil {
		// Use short hostname
		if idx := strings.IndexByte(h, '.'); idx > 0 {
			h = h[:idx]
		}
		machine = strings.ToLower(h)
	}

	return map[string]any{
		"user":    user,
		"machine": machine,
	}
}

func WriteBasePrompts(wolfcastleDir string) {
	prompts := map[string]string{
		"base/prompts/script-reference.md": `# Wolfcastle Script Reference

All commands accept ` + "`--json`" + ` for structured output. Always use ` + "`--json`" + ` when calling Wolfcastle programmatically.

Tree addresses use slash-delimited paths: ` + "`my-project`" + `, ` + "`my-project/sub-module`" + `, ` + "`my-project/sub-module/task-1`" + `.

---

## Task Commands

Manage tasks on leaf nodes. Tasks follow the lifecycle: not_started -> in_progress -> complete (or -> blocked).

### wolfcastle task add

Add a new task to a leaf node.

` + "```" + `
wolfcastle task add "description of the task" --node <node-address>
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | Yes | Target leaf node address |

**Example:**
` + "```" + `
wolfcastle task add "Implement user authentication middleware" --node my-project/auth
` + "```" + `

### wolfcastle task claim

Claim a task (transition from not_started to in_progress).

` + "```" + `
wolfcastle task claim --node <node-address/task-id>
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | Yes | Task address: node-path/task-id |

**Example:**
` + "```" + `
wolfcastle task claim --node my-project/auth/task-1
` + "```" + `

### wolfcastle task complete

Complete a task (transition from in_progress to complete). When all tasks on a node are complete, the node itself becomes complete.

` + "```" + `
wolfcastle task complete --node <node-address/task-id>
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | Yes | Task address: node-path/task-id |

**Example:**
` + "```" + `
wolfcastle task complete --node my-project/auth/task-1
` + "```" + `

### wolfcastle task block

Block a task (transition from in_progress to blocked). Requires a reason.

` + "```" + `
wolfcastle task block "reason for blocking" --node <node-address/task-id>
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | Yes | Task address: node-path/task-id |

**Example:**
` + "```" + `
wolfcastle task block "Waiting for database migration to complete" --node my-project/auth/task-2
` + "```" + `

---

## Project Commands

Create and organize project nodes in the work tree.

### wolfcastle project create

Create a new project or sub-project node.

` + "```" + `
wolfcastle project create "Project Name" [--node <parent-address>] [--type <leaf|orchestrator>]
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | No | Parent node address (omit for root-level project) |
| ` + "`--type`" + ` | No | Node type: ` + "`leaf`" + ` (default) or ` + "`orchestrator`" + ` |

Leaf nodes hold tasks. Orchestrator nodes hold child projects.

**Examples:**
` + "```" + `
wolfcastle project create "Authentication Module"
wolfcastle project create "OAuth Provider" --node auth-module --type leaf
wolfcastle project create "API Gateway" --type orchestrator
` + "```" + `

---

## Audit Commands

Record progress and escalate issues through the audit trail.

### wolfcastle audit breadcrumb

Add a breadcrumb entry to a node's audit trail. Use this to record what you did and why.

` + "```" + `
wolfcastle audit breadcrumb "description of what happened" --node <node-address>
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | Yes | Target node address |

**Example:**
` + "```" + `
wolfcastle audit breadcrumb "Refactored auth middleware to use JWT validation" --node my-project/auth
` + "```" + `

### wolfcastle audit escalate

Escalate a gap or issue to the parent node's audit trail. Use when a child node encounters something that the parent orchestrator needs to know about.

` + "```" + `
wolfcastle audit escalate "description of the gap" --node <source-node-address>
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | Yes | Source node address (escalation goes to its parent) |

**Example:**
` + "```" + `
wolfcastle audit escalate "API contract is inconsistent with frontend expectations" --node my-project/api
` + "```" + `

---

## Documentation Commands

### wolfcastle adr create

Create a new Architecture Decision Record.

` + "```" + `
wolfcastle adr create "Decision Title" [--stdin] [--file <path>]
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--stdin`" + ` | No | Read ADR body from stdin |
| ` + "`--file`" + ` | No | Read ADR body from a file |

If neither ` + "`--stdin`" + ` nor ` + "`--file`" + ` is provided, a template is generated.

**Example:**
` + "```" + `
wolfcastle adr create "Use PostgreSQL for primary datastore"
wolfcastle adr create "Switch to gRPC" --file /tmp/adr-body.md
` + "```" + `

---

## Archive Commands

### wolfcastle archive add

Generate an archive entry for a completed node. The node must be in the ` + "`complete`" + ` state.

` + "```" + `
wolfcastle archive add --node <node-address>
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | Yes | Node address to archive |

**Example:**
` + "```" + `
wolfcastle archive add --node my-project/auth
` + "```" + `

---

## Inbox Commands

### wolfcastle inbox add

Add an idea or item to the inbox for later triage.

` + "```" + `
wolfcastle inbox add "description of the idea"
` + "```" + `

No flags required. The item is timestamped and stored as ` + "`new`" + `.

**Example:**
` + "```" + `
wolfcastle inbox add "Consider adding rate limiting to the API gateway"
` + "```" + `

---

## Navigation

### wolfcastle navigate

Find the next actionable task via depth-first traversal of the project tree. Does NOT claim the task — use ` + "`wolfcastle task claim`" + ` after navigating.

` + "```" + `
wolfcastle navigate [--node <scope-address>]
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | No | Scope navigation to a subtree |

**Example:**
` + "```" + `
wolfcastle navigate
wolfcastle navigate --node my-project
` + "```" + `

---

## Spec Commands

Manage specifications linked to project nodes.

### wolfcastle spec create

Create a new spec document, optionally linked to a node.

` + "```" + `
wolfcastle spec create "Spec Title" [--node <node-address>]
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | No | Link the spec to this node |

**Example:**
` + "```" + `
wolfcastle spec create "API Authentication Flow" --node my-project/auth
` + "```" + `

### wolfcastle spec link

Link an existing spec file to a node.

` + "```" + `
wolfcastle spec link "spec-filename.md" --node <node-address>
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | Yes | Target node address |

**Example:**
` + "```" + `
wolfcastle spec link "2026-03-12T10-00Z-api-auth-flow.md" --node my-project/auth
` + "```" + `

### wolfcastle spec list

List specs, optionally filtered by node.

` + "```" + `
wolfcastle spec list [--node <node-address>]
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | No | Filter to specs linked to this node |

**Example:**
` + "```" + `
wolfcastle spec list
wolfcastle spec list --node my-project/auth
` + "```" + `

---

## Status

### wolfcastle status

Show the current state of the project tree.

` + "```" + `
wolfcastle status [--node <scope-address>] [--all]
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | No | Show status for a specific subtree |
| ` + "`--all`" + ` | No | Show status across all engineers |

**Example:**
` + "```" + `
wolfcastle status
wolfcastle status --node my-project
` + "```" + `
`,
		"base/prompts/execute.md": `# Execute Stage

You are Wolfcastle's execution agent. Your job is to complete one task per iteration.

## Phases

### A. Claim
The daemon has already claimed your task. Verify the task details in the iteration context below.

### B. Study
Read relevant code, ADRs, and specs before making changes. Use grep, find, and file reading tools to understand the codebase.

### C. Implement
Make the changes needed to complete the task. Focus on one concern at a time.

### D. Validate
Run any configured validation commands. Fix issues before proceeding.

### E. Record
Write a breadcrumb describing what you did:
` + "```" + `
wolfcastle audit breadcrumb --node <your-node> "description of changes"
` + "```" + `

### F. Commit
Commit your changes with a clear message.

### G. Yield
Output WOLFCASTLE_YIELD on its own line. This is a hard stop — do not continue after yielding.

## Rules
- One task per iteration. No exceptions.
- Commit before yielding.
- Never edit state.json files directly.
- If you cannot complete the task, call wolfcastle task block.
`,
		"base/prompts/expand.md": `# Expand Stage

You are processing inbox items. For each bullet point in the inbox, expand it into a structured project description with:
- Clear title
- Scope description
- Acceptance criteria
- Suggested task breakdown

Output each expanded item as a Markdown section starting with ## .
`,
		"base/prompts/file.md": `# File Stage

You are filing expanded inbox items into the Wolfcastle project tree. For each item:
1. Determine if it fits an existing project or needs a new one
2. Create the project or sub-project using wolfcastle project create
3. Add tasks using wolfcastle task add
4. Each project must end with an audit task
`,
		"base/prompts/summary.md": `# Summary Stage

Write a brief plain-language summary of what this completed node accomplished and why it matters. Focus on outcomes and decisions, not mechanics.
`,
	}

	for path, content := range prompts {
		full := filepath.Join(wolfcastleDir, path)
		os.WriteFile(full, []byte(content), 0644)
	}

	// Write default rule fragments
	rules := map[string]string{
		"base/rules/git-conventions.md": `## Git Conventions
- One logical commit per task
- Write clear commit messages that explain why, not what
- Never commit broken code — validate before committing
- Never force push to main/master
`,
		"base/rules/adr-policy.md": `## ADR Policy
- Check existing ADRs before making architectural decisions
- If you must diverge from an existing ADR, file a new one superseding it
- Use wolfcastle adr create to file new decisions
`,
	}

	for path, content := range rules {
		full := filepath.Join(wolfcastleDir, path)
		os.WriteFile(full, []byte(content), 0644)
	}

	// Write default audit scopes
	audits := map[string]string{
		"base/audits/dry.md": `Identify DRY (Don't Repeat Yourself) violations in the codebase.

Look for:
- Duplicated logic across files or functions
- Copy-pasted code blocks with minor variations
- Repeated patterns that could be abstracted into shared utilities
- Similar data transformations happening in multiple places

For each finding, describe:
1. What is duplicated and where
2. The estimated scope of repetition (how many instances)
3. A suggested approach to eliminate the duplication
`,
		"base/audits/modularity.md": `Identify modularity issues in the codebase.

Look for:
- God objects or files that do too many things
- Tight coupling between modules that should be independent
- Missing abstraction boundaries
- Circular dependencies
- Functions or methods with too many responsibilities

For each finding, describe:
1. What module/file has the issue
2. Why the current structure is problematic
3. A suggested decomposition or refactoring approach
`,
		"base/audits/decomposition.md": `Identify overly complex code that would benefit from decomposition.

Look for:
- Functions longer than ~50 lines
- Deeply nested conditionals (3+ levels)
- Functions with high cyclomatic complexity
- Long parameter lists suggesting missing abstractions
- Switch/case blocks that could be polymorphic

For each finding, describe:
1. The specific function/method and its location
2. Why it is too complex
3. A suggested decomposition into smaller, focused units
`,
		"base/audits/comments.md": `Identify documentation and commenting gaps in the codebase.

Look for:
- Public APIs without documentation
- Complex logic without explanatory comments
- Misleading or outdated comments
- Missing package-level documentation
- Non-obvious "why" decisions that lack explanation

For each finding, describe:
1. What is missing and where
2. Why documentation matters for this specific case
3. What the comment or documentation should say
`,
	}

	for path, content := range audits {
		full := filepath.Join(wolfcastleDir, path)
		os.WriteFile(full, []byte(content), 0644)
	}

	// Write base audit prompt
	auditPrompt := `# Codebase Audit

You are performing a comprehensive codebase audit. For each scope below, analyze the codebase thoroughly and produce actionable findings.

## Output Format

For each finding:
1. **Title** — a short description
2. **Severity** — high, medium, low
3. **Location** — specific files and line ranges
4. **Description** — what the issue is and why it matters
5. **Suggested Fix** — concrete steps to resolve it
6. **Estimated Effort** — small (< 1 hour), medium (1-4 hours), large (4+ hours)

Group findings by scope. Prioritize high-severity items first within each scope.
`
	os.WriteFile(filepath.Join(wolfcastleDir, "base/prompts/audit.md"), []byte(auditPrompt), 0644)

	// Write unblock prompt
	unblockPrompt := `# Unblock Assistant

You are helping the user resolve a blocked task. The diagnostic context below shows what went wrong and what was tried.

Your role:
1. Understand the block reason and failure history
2. Ask clarifying questions to understand the environment
3. Suggest concrete fixes
4. Help the user implement the fix

When the issue is resolved, remind the user to run the unblock command shown in the diagnostic.
`
	os.WriteFile(filepath.Join(wolfcastleDir, "base/prompts/unblock.md"), []byte(unblockPrompt), 0644)
}

// CreateProject creates a new project node in the tree.
func CreateProject(
	idx *state.RootIndex,
	parentAddr string,
	slug string,
	name string,
	nodeType state.NodeType,
	resolver interface{ NodeDir(addr interface{}) string },
) (*state.NodeState, string, error) {
	// Build the new address
	var addr string
	if parentAddr == "" {
		addr = slug
	} else {
		addr = parentAddr + "/" + slug
	}

	// Check for duplicates
	if _, exists := idx.Nodes[addr]; exists {
		return nil, "", fmt.Errorf("node %q already exists", addr)
	}

	// Create node state
	ns := state.NewNodeState(slug, name, nodeType)

	// Add audit task for leaf nodes
	if nodeType == state.NodeLeaf {
		ns.Tasks = []state.Task{
			{
				ID:          "audit",
				Description: "Verify all work in " + name + " is complete and correct",
				State:       state.StatusNotStarted,
			},
		}
	}

	// Update root index
	entry := state.IndexEntry{
		Name:     name,
		Type:     nodeType,
		State:    state.StatusNotStarted,
		Address:  addr,
		Parent:   parentAddr,
		Children: []string{},
	}
	idx.Nodes[addr] = entry

	// Update parent's children list
	if parentAddr != "" {
		if parent, ok := idx.Nodes[parentAddr]; ok {
			parent.Children = append(parent.Children, addr)
			idx.Nodes[parentAddr] = parent
		}
	}

	return ns, addr, nil
}
