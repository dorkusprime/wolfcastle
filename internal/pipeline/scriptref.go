package pipeline

// GenerateScriptReference produces a Markdown document listing every wolfcastle
// command the model can call, with syntax, description, required flags, and
// example usage. This is injected into the model's system prompt per ADR-017.
//
// NOTE: The authoritative copy of the script reference is written directly by
// WriteBasePrompts in the project package (base/prompts/script-reference.md).
// This function exists so that pipeline code can also access the reference at
// runtime without reading from disk — for example, when assembling the system
// prompt for a model invocation.
func GenerateScriptReference() string {
	return scriptReferenceMarkdown
}

const scriptReferenceMarkdown = `# Wolfcastle Script Reference

All commands accept ` + "`--json`" + ` for structured output. Always use ` + "`--json`" + ` when calling Wolfcastle programmatically.

Tree addresses use slash-delimited paths: ` + "`my-project`" + `, ` + "`my-project/sub-module`" + `, ` + "`my-project/sub-module/task-1`" + `.

---

## Task Commands

Manage tasks on leaf nodes. Tasks follow the lifecycle: not_started -> in_progress -> complete (or -> blocked).

### wolfcastle task add

Add a new task to a leaf node.

` + "```" + `
wolfcastle task add "task title" --node <node-address> [--body "detailed description"] [--stdin]
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--node`" + ` | Yes | Target leaf node address |
| ` + "`--body`" + ` | No | Detailed task description/body text |
| ` + "`--stdin`" + ` | No | Read task body from stdin |

**Examples:**
` + "```" + `
wolfcastle task add "Implement user authentication middleware" --node my-project/auth
wolfcastle task add "Add rate limiting" --node my-project/auth --body "Implement token-bucket rate limiting at 100 req/s per user."
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

## Audit Review Commands

Review findings from ` + "`wolfcastle audit run`" + `. Findings are saved as a pending batch; use these commands to approve or reject them individually.

### wolfcastle audit pending

Show pending audit findings awaiting review.

` + "```" + `
wolfcastle audit pending
` + "```" + `

### wolfcastle audit approve

Approve a finding and create a project for it.

` + "```" + `
wolfcastle audit approve <finding-id>
wolfcastle audit approve --all
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--all`" + ` | No | Approve all pending findings |

### wolfcastle audit reject

Reject a finding without creating a project.

` + "```" + `
wolfcastle audit reject <finding-id>
wolfcastle audit reject --all
` + "```" + `

| Flag | Required | Description |
|------|----------|-------------|
| ` + "`--all`" + ` | No | Reject all pending findings |

### wolfcastle audit history

Show past audit review decisions.

` + "```" + `
wolfcastle audit history
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

Find the next actionable task via depth-first traversal of the project tree. Does NOT claim the task. Use ` + "`wolfcastle task claim`" + ` after navigating.

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
`
