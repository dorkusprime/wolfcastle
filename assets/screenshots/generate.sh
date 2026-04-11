#!/usr/bin/env bash
#
# Generate TUI screenshots using VHS tape files.
#
# Usage: ./generate.sh [binary-path]
#
# If no binary path is given, builds wolfcastle to a temp location.
# Requires: vhs (brew install vhs)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TAPE_DIR="$SCRIPT_DIR/tapes"
OUT_DIR="$SCRIPT_DIR"

# Build or locate the binary.
if [[ -n "${1:-}" ]]; then
    WOLFCASTLE="$1"
else
    echo "Building wolfcastle..."
    WOLFCASTLE="$(mktemp -d)/wolfcastle"
    (cd "$REPO_ROOT" && go build -o "$WOLFCASTLE" .)
fi

if ! command -v vhs &>/dev/null; then
    echo "error: vhs not found. Install with: brew install vhs" >&2
    exit 1
fi

export PATH="$(dirname "$WOLFCASTLE"):$PATH"

# ---------------------------------------------------------------------------
# Staging directory helpers
# ---------------------------------------------------------------------------

cleanup_dirs=("")
make_stage() {
    local d
    d="$(mktemp -d)"
    cleanup_dirs+=("$d")
    echo "$d"
}

cleanup() {
    for d in "${cleanup_dirs[@]}"; do
        [[ -n "$d" ]] && rm -rf "$d"
    done
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Helper: create a node state file inside a namespace directory
# ---------------------------------------------------------------------------
create_node() {
    local ns_dir="$1" addr="$2" name="$3" type="$4" nstate="$5"
    shift 5
    local dir="$ns_dir/$addr"
    mkdir -p "$dir"
    local tasks="${1:-}"
    if [[ -n "$tasks" ]]; then
        cat > "$dir/state.json" << EOF
{
  "name": "$name",
  "type": "$type",
  "state": "$nstate",
  "tasks": $tasks
}
EOF
    else
        cat > "$dir/state.json" << EOF
{
  "name": "$name",
  "type": "$type",
  "state": "$nstate"
}
EOF
    fi
}

# ---------------------------------------------------------------------------
# Helper: initialize a stage dir and return the namespace directory path
# ---------------------------------------------------------------------------
init_stage() {
    local stage="$1"
    (cd "$stage" && wolfcastle init >/dev/null 2>&1)
    (cd "$stage" && wolfcastle project create demo-app >/dev/null 2>&1 || true)
    local ns_dir
    ns_dir=$(find "$stage/.wolfcastle/system/projects" -mindepth 1 -maxdepth 1 -type d | head -1)
    if [[ -z "$ns_dir" ]]; then
        echo "error: could not find namespace directory in $stage" >&2
        exit 1
    fi
    echo "$ns_dir"
}

# ---------------------------------------------------------------------------
# STAGE_MAIN: the primary staging directory with a realistic mixed-state tree
# ---------------------------------------------------------------------------
STAGE_MAIN="$(make_stage)"
NS_MAIN="$(init_stage "$STAGE_MAIN")"

cat > "$NS_MAIN/state.json" << 'STATEEOF'
{
  "version": 1,
  "root_id": "warzone",
  "root_name": "warzone",
  "root": ["warzone"],
  "root_state": "in_progress",
  "nodes": {
    "warzone": {
      "name": "warzone",
      "type": "orchestrator",
      "state": "in_progress",
      "address": "warzone",
      "children": ["warzone/backend", "warzone/frontend", "warzone/infra"]
    },
    "warzone/backend": {
      "name": "backend",
      "type": "orchestrator",
      "state": "in_progress",
      "address": "warzone/backend",
      "parent": "warzone",
      "children": ["warzone/backend/api", "warzone/backend/auth", "warzone/backend/database"]
    },
    "warzone/backend/api": {
      "name": "api",
      "type": "leaf",
      "state": "complete",
      "address": "warzone/backend/api",
      "parent": "warzone/backend"
    },
    "warzone/backend/auth": {
      "name": "auth",
      "type": "leaf",
      "state": "in_progress",
      "address": "warzone/backend/auth",
      "parent": "warzone/backend"
    },
    "warzone/backend/database": {
      "name": "database",
      "type": "leaf",
      "state": "not_started",
      "address": "warzone/backend/database",
      "parent": "warzone/backend"
    },
    "warzone/frontend": {
      "name": "frontend",
      "type": "orchestrator",
      "state": "blocked",
      "address": "warzone/frontend",
      "parent": "warzone",
      "children": ["warzone/frontend/components", "warzone/frontend/routing"]
    },
    "warzone/frontend/components": {
      "name": "components",
      "type": "leaf",
      "state": "complete",
      "address": "warzone/frontend/components",
      "parent": "warzone/frontend"
    },
    "warzone/frontend/routing": {
      "name": "routing",
      "type": "leaf",
      "state": "blocked",
      "address": "warzone/frontend/routing",
      "parent": "warzone/frontend"
    },
    "warzone/infra": {
      "name": "infra",
      "type": "leaf",
      "state": "not_started",
      "address": "warzone/infra",
      "parent": "warzone"
    }
  }
}
STATEEOF

create_node "$NS_MAIN" "warzone" "warzone" "orchestrator" "in_progress"
create_node "$NS_MAIN" "warzone/backend" "backend" "orchestrator" "in_progress"
create_node "$NS_MAIN" "warzone/backend/api" "api" "leaf" "complete" '[
  {"id":"task-1","title":"Implement REST endpoints","state":"complete","class":"coding/go","description":"Build CRUD endpoints for the donut API"},
  {"id":"audit","title":"Audit","state":"complete","is_audit":true}
]'
create_node "$NS_MAIN" "warzone/backend/auth" "auth" "leaf" "in_progress" '[
  {"id":"task-1","title":"Add OAuth2 PKCE flow","state":"in_progress","class":"coding/go","description":"Implement the authorization code flow with PKCE for public clients"},
  {"id":"task-2","title":"Session token rotation","state":"not_started","class":"coding/go"},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'
create_node "$NS_MAIN" "warzone/backend/database" "database" "leaf" "not_started" '[
  {"id":"task-1","title":"Schema migration framework","state":"not_started","class":"coding/go"},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'
create_node "$NS_MAIN" "warzone/frontend" "frontend" "orchestrator" "blocked"
create_node "$NS_MAIN" "warzone/frontend/components" "components" "leaf" "complete" '[
  {"id":"task-1","title":"Build component library","state":"complete","class":"coding/react"},
  {"id":"audit","title":"Audit","state":"complete","is_audit":true}
]'
create_node "$NS_MAIN" "warzone/frontend/routing" "routing" "leaf" "blocked" '[
  {"id":"task-1","title":"Implement client-side routing","state":"blocked","class":"coding/react","block_reason":"Waiting for auth API to expose public endpoints","failure_count":3,"last_failure_type":"dependency"},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'
create_node "$NS_MAIN" "warzone/infra" "infra" "leaf" "not_started" '[
  {"id":"task-1","title":"Terraform AWS deployment","state":"not_started","class":"coding/terraform"},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'

cat > "$NS_MAIN/inbox.json" << 'INBOXEOF'
{
  "items": [
    {"id":"inbox-001","text":"Add rate limiting to the API","status":"new","created_at":"2026-04-11T10:00:00Z"},
    {"id":"inbox-002","text":"Set up CI/CD pipeline with GitHub Actions","status":"new","created_at":"2026-04-11T10:05:00Z"},
    {"id":"inbox-003","text":"Add OpenTelemetry tracing","status":"filed","created_at":"2026-04-11T09:30:00Z"}
  ]
}
INBOXEOF

echo "Main staging:    $STAGE_MAIN"
echo "Main namespace:  $NS_MAIN"

# ---------------------------------------------------------------------------
# STAGE_COMPLETE: all nodes in "complete" state
# ---------------------------------------------------------------------------
STAGE_COMPLETE="$(make_stage)"
NS_COMPLETE="$(init_stage "$STAGE_COMPLETE")"

cat > "$NS_COMPLETE/state.json" << 'STATEEOF'
{
  "version": 1,
  "root_id": "warzone",
  "root_name": "warzone",
  "root": ["warzone"],
  "root_state": "complete",
  "nodes": {
    "warzone": {
      "name": "warzone",
      "type": "orchestrator",
      "state": "complete",
      "address": "warzone",
      "children": ["warzone/backend", "warzone/frontend", "warzone/infra"]
    },
    "warzone/backend": {
      "name": "backend",
      "type": "orchestrator",
      "state": "complete",
      "address": "warzone/backend",
      "parent": "warzone",
      "children": ["warzone/backend/api", "warzone/backend/auth", "warzone/backend/database"]
    },
    "warzone/backend/api": {
      "name": "api",
      "type": "leaf",
      "state": "complete",
      "address": "warzone/backend/api",
      "parent": "warzone/backend"
    },
    "warzone/backend/auth": {
      "name": "auth",
      "type": "leaf",
      "state": "complete",
      "address": "warzone/backend/auth",
      "parent": "warzone/backend"
    },
    "warzone/backend/database": {
      "name": "database",
      "type": "leaf",
      "state": "complete",
      "address": "warzone/backend/database",
      "parent": "warzone/backend"
    },
    "warzone/frontend": {
      "name": "frontend",
      "type": "orchestrator",
      "state": "complete",
      "address": "warzone/frontend",
      "parent": "warzone",
      "children": ["warzone/frontend/components", "warzone/frontend/routing"]
    },
    "warzone/frontend/components": {
      "name": "components",
      "type": "leaf",
      "state": "complete",
      "address": "warzone/frontend/components",
      "parent": "warzone/frontend"
    },
    "warzone/frontend/routing": {
      "name": "routing",
      "type": "leaf",
      "state": "complete",
      "address": "warzone/frontend/routing",
      "parent": "warzone/frontend"
    },
    "warzone/infra": {
      "name": "infra",
      "type": "leaf",
      "state": "complete",
      "address": "warzone/infra",
      "parent": "warzone"
    }
  }
}
STATEEOF

create_node "$NS_COMPLETE" "warzone" "warzone" "orchestrator" "complete"
create_node "$NS_COMPLETE" "warzone/backend" "backend" "orchestrator" "complete"
create_node "$NS_COMPLETE" "warzone/backend/api" "api" "leaf" "complete" '[
  {"id":"task-1","title":"Implement REST endpoints","state":"complete","class":"coding/go"},
  {"id":"audit","title":"Audit","state":"complete","is_audit":true}
]'
create_node "$NS_COMPLETE" "warzone/backend/auth" "auth" "leaf" "complete" '[
  {"id":"task-1","title":"Add OAuth2 PKCE flow","state":"complete","class":"coding/go"},
  {"id":"audit","title":"Audit","state":"complete","is_audit":true}
]'
create_node "$NS_COMPLETE" "warzone/backend/database" "database" "leaf" "complete" '[
  {"id":"task-1","title":"Schema migration framework","state":"complete","class":"coding/go"},
  {"id":"audit","title":"Audit","state":"complete","is_audit":true}
]'
create_node "$NS_COMPLETE" "warzone/frontend" "frontend" "orchestrator" "complete"
create_node "$NS_COMPLETE" "warzone/frontend/components" "components" "leaf" "complete" '[
  {"id":"task-1","title":"Build component library","state":"complete","class":"coding/react"},
  {"id":"audit","title":"Audit","state":"complete","is_audit":true}
]'
create_node "$NS_COMPLETE" "warzone/frontend/routing" "routing" "leaf" "complete" '[
  {"id":"task-1","title":"Implement client-side routing","state":"complete","class":"coding/react"},
  {"id":"audit","title":"Audit","state":"complete","is_audit":true}
]'
create_node "$NS_COMPLETE" "warzone/infra" "infra" "leaf" "complete" '[
  {"id":"task-1","title":"Terraform AWS deployment","state":"complete","class":"coding/terraform"},
  {"id":"audit","title":"Audit","state":"complete","is_audit":true}
]'

echo "Complete staging: $STAGE_COMPLETE"

# ---------------------------------------------------------------------------
# STAGE_BLOCKED: all nodes in "blocked" state
# ---------------------------------------------------------------------------
STAGE_BLOCKED="$(make_stage)"
NS_BLOCKED="$(init_stage "$STAGE_BLOCKED")"

cat > "$NS_BLOCKED/state.json" << 'STATEEOF'
{
  "version": 1,
  "root_id": "warzone",
  "root_name": "warzone",
  "root": ["warzone"],
  "root_state": "blocked",
  "nodes": {
    "warzone": {
      "name": "warzone",
      "type": "orchestrator",
      "state": "blocked",
      "address": "warzone",
      "children": ["warzone/backend", "warzone/frontend", "warzone/infra"]
    },
    "warzone/backend": {
      "name": "backend",
      "type": "orchestrator",
      "state": "blocked",
      "address": "warzone/backend",
      "parent": "warzone",
      "children": ["warzone/backend/api", "warzone/backend/auth", "warzone/backend/database"]
    },
    "warzone/backend/api": {
      "name": "api",
      "type": "leaf",
      "state": "blocked",
      "address": "warzone/backend/api",
      "parent": "warzone/backend"
    },
    "warzone/backend/auth": {
      "name": "auth",
      "type": "leaf",
      "state": "blocked",
      "address": "warzone/backend/auth",
      "parent": "warzone/backend"
    },
    "warzone/backend/database": {
      "name": "database",
      "type": "leaf",
      "state": "blocked",
      "address": "warzone/backend/database",
      "parent": "warzone/backend"
    },
    "warzone/frontend": {
      "name": "frontend",
      "type": "orchestrator",
      "state": "blocked",
      "address": "warzone/frontend",
      "parent": "warzone",
      "children": ["warzone/frontend/components", "warzone/frontend/routing"]
    },
    "warzone/frontend/components": {
      "name": "components",
      "type": "leaf",
      "state": "blocked",
      "address": "warzone/frontend/components",
      "parent": "warzone/frontend"
    },
    "warzone/frontend/routing": {
      "name": "routing",
      "type": "leaf",
      "state": "blocked",
      "address": "warzone/frontend/routing",
      "parent": "warzone/frontend"
    },
    "warzone/infra": {
      "name": "infra",
      "type": "leaf",
      "state": "blocked",
      "address": "warzone/infra",
      "parent": "warzone"
    }
  }
}
STATEEOF

create_node "$NS_BLOCKED" "warzone" "warzone" "orchestrator" "blocked"
create_node "$NS_BLOCKED" "warzone/backend" "backend" "orchestrator" "blocked"
create_node "$NS_BLOCKED" "warzone/backend/api" "api" "leaf" "blocked" '[
  {"id":"task-1","title":"Implement REST endpoints","state":"blocked","class":"coding/go","block_reason":"External API unavailable","failure_count":5},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'
create_node "$NS_BLOCKED" "warzone/backend/auth" "auth" "leaf" "blocked" '[
  {"id":"task-1","title":"Add OAuth2 PKCE flow","state":"blocked","class":"coding/go","block_reason":"Identity provider down","failure_count":2},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'
create_node "$NS_BLOCKED" "warzone/backend/database" "database" "leaf" "blocked" '[
  {"id":"task-1","title":"Schema migration framework","state":"blocked","class":"coding/go","block_reason":"Migration lock held by another process","failure_count":1},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'
create_node "$NS_BLOCKED" "warzone/frontend" "frontend" "orchestrator" "blocked"
create_node "$NS_BLOCKED" "warzone/frontend/components" "components" "leaf" "blocked" '[
  {"id":"task-1","title":"Build component library","state":"blocked","class":"coding/react","block_reason":"Design system not finalized","failure_count":4},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'
create_node "$NS_BLOCKED" "warzone/frontend/routing" "routing" "leaf" "blocked" '[
  {"id":"task-1","title":"Implement client-side routing","state":"blocked","class":"coding/react","block_reason":"Waiting for auth API","failure_count":3},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'
create_node "$NS_BLOCKED" "warzone/infra" "infra" "leaf" "blocked" '[
  {"id":"task-1","title":"Terraform AWS deployment","state":"blocked","class":"coding/terraform","block_reason":"AWS credentials expired","failure_count":7},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'

echo "Blocked staging:  $STAGE_BLOCKED"

# ---------------------------------------------------------------------------
# STAGE_WELCOME: empty directory, no .wolfcastle/ (welcome screen)
# ---------------------------------------------------------------------------
STAGE_WELCOME="$(make_stage)"
echo "Welcome staging:  $STAGE_WELCOME"

echo ""

# ---------------------------------------------------------------------------
# Tape routing: map tape names to their staging directories
# ---------------------------------------------------------------------------
stage_for_tape() {
    case "$1" in
        tui-all-complete)    echo "$STAGE_COMPLETE" ;;
        tui-all-blocked)     echo "$STAGE_BLOCKED" ;;
        tui-welcome-sessions) echo "$STAGE_WELCOME" ;;
        *)                   echo "$STAGE_MAIN" ;;
    esac
}

# ---------------------------------------------------------------------------
# Run tapes
# ---------------------------------------------------------------------------
SUCCESS=0
FAILED=0
SKIPPED=0

for tape in "$TAPE_DIR"/*.tape; do
    name="$(basename "$tape" .tape)"

    # Pick the right staging directory.
    stage="$(stage_for_tape "$name")"

    echo "Recording: $name (from $stage)"
    (cd "$stage" && vhs "$tape" 2>&1) || {
        echo "  FAILED: $name" >&2
        FAILED=$((FAILED + 1))
        continue
    }

    # VHS writes the screenshot relative to cwd (the staging dir).
    if [[ -f "$stage/$name.png" ]]; then
        mv "$stage/$name.png" "$OUT_DIR/$name.png"
        echo "  -> $OUT_DIR/$name.png"
        SUCCESS=$((SUCCESS + 1))
    else
        echo "  WARNING: $name.png not found in $stage" >&2
        SKIPPED=$((SKIPPED + 1))
    fi
done

echo ""
echo "Done: $SUCCESS succeeded, $FAILED failed, $SKIPPED missing"
echo "Screenshots written to: $OUT_DIR/"
ls -1 "$OUT_DIR"/*.png 2>/dev/null || echo "(no screenshots generated)"
