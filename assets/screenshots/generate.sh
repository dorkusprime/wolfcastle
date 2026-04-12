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
    local label="${1:-demo}"
    local d="/tmp/wolfcastle-${label}"
    rm -rf "$d"
    mkdir -p "$d"
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
STAGE_MAIN="$(make_stage demo)"
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
      "state": "in_progress",
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
      "state": "in_progress",
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
  {"id":"task-1","title":"Add OAuth2 PKCE flow","state":"in_progress","class":"coding/go","description":"Implement the authorization code flow with PKCE for public clients. The auth service needs a /authorize endpoint that generates code challenges, a /token endpoint that verifies code verifiers, and JWKS rotation for signing tokens.","scope":["internal/auth/","internal/token/","cmd/auth/"],"deliverables":["PKCE authorize endpoint","Token exchange endpoint","JWKS key rotation"],"acceptance_criteria":["All OAuth2 PKCE conformance tests pass","Token lifetime configurable via config","Refresh token rotation on every use"]},
  {"id":"task-2","title":"Session token rotation","state":"not_started","class":"coding/go","description":"Rotate session tokens on each request to prevent replay attacks"},
  {"id":"task-3","title":"Rate limit auth endpoints","state":"not_started","class":"coding/go","description":"Add per-IP rate limiting to /authorize and /token"},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'
create_node "$NS_MAIN" "warzone/backend/database" "database" "leaf" "not_started" '[
  {"id":"task-1","title":"Schema migration framework","state":"not_started","class":"coding/go"},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'
create_node "$NS_MAIN" "warzone/frontend" "frontend" "orchestrator" "in_progress"
create_node "$NS_MAIN" "warzone/frontend/components" "components" "leaf" "complete" '[
  {"id":"task-1","title":"Build component library","state":"complete","class":"coding/react"},
  {"id":"audit","title":"Audit","state":"complete","is_audit":true}
]'
create_node "$NS_MAIN" "warzone/frontend/routing" "routing" "leaf" "in_progress" '[
  {"id":"task-1","title":"Implement client-side routing","state":"in_progress","class":"coding/react"},
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

# Write realistic log data so the log stream has content.
LOG_DIR="$STAGE_MAIN/.wolfcastle/system/logs"
mkdir -p "$LOG_DIR"
cat > "$LOG_DIR/0001-exec-20260411T08-00Z.jsonl" << 'LOGEOF'
{"type":"daemon_lifecycle","timestamp":"2026-04-11T08:00:01Z","level":"info","trace":"exec","event":"start","text":"daemon started on branch main"}
{"type":"iteration_header","timestamp":"2026-04-11T08:00:02Z","level":"info","trace":"exec","iteration":1,"node":"warzone/backend/api","task":"task-1","text":"claiming task: Implement REST endpoints"}
{"type":"stage_start","timestamp":"2026-04-11T08:00:03Z","level":"info","trace":"exec","stage":"execute","node":"warzone/backend/api","task":"task-1"}
{"type":"model_invoke","timestamp":"2026-04-11T08:00:04Z","level":"debug","trace":"exec","stage":"execute","text":"invoking model: claude -p --model claude-sonnet-4-20250514","node":"warzone/backend/api"}
{"type":"breadcrumb","timestamp":"2026-04-11T08:01:30Z","level":"info","trace":"exec","node":"warzone/backend/api","task":"task-1","text":"implemented GET /api/donuts, POST /api/donuts, DELETE /api/donuts/:id"}
{"type":"breadcrumb","timestamp":"2026-04-11T08:01:45Z","level":"info","trace":"exec","node":"warzone/backend/api","task":"task-1","text":"added integration tests for all CRUD endpoints, 100% pass rate"}
{"type":"marker","timestamp":"2026-04-11T08:02:00Z","level":"info","trace":"exec","marker":"WOLFCASTLE_COMPLETE","node":"warzone/backend/api","task":"task-1","text":"task complete"}
{"type":"commit","timestamp":"2026-04-11T08:02:05Z","level":"info","trace":"exec","text":"committed: wolfcastle: warzone/backend/api/task-1 complete","node":"warzone/backend/api"}
{"type":"stage_end","timestamp":"2026-04-11T08:02:06Z","level":"info","trace":"exec","stage":"execute","duration_ms":123000,"exit_code":0}
{"type":"iteration_header","timestamp":"2026-04-11T08:02:10Z","level":"info","trace":"exec","iteration":2,"node":"warzone/backend/auth","task":"task-1","text":"claiming task: Add OAuth2 PKCE flow"}
{"type":"stage_start","timestamp":"2026-04-11T08:02:11Z","level":"info","trace":"exec","stage":"execute","node":"warzone/backend/auth","task":"task-1"}
{"type":"model_invoke","timestamp":"2026-04-11T08:02:12Z","level":"debug","trace":"exec","stage":"execute","text":"invoking model: claude -p --model claude-sonnet-4-20250514","node":"warzone/backend/auth"}
{"type":"breadcrumb","timestamp":"2026-04-11T08:04:00Z","level":"info","trace":"exec","node":"warzone/backend/auth","task":"task-1","text":"scaffolded /authorize and /token endpoints with PKCE challenge verification"}
{"type":"validation","timestamp":"2026-04-11T08:04:15Z","level":"warn","trace":"exec","node":"warzone/backend/auth","text":"test coverage at 78%, below 80% threshold"}
{"type":"breadcrumb","timestamp":"2026-04-11T08:04:30Z","level":"info","trace":"exec","node":"warzone/backend/auth","task":"task-1","text":"added JWKS rotation with configurable key lifetime (default 24h)"}
{"type":"validation","timestamp":"2026-04-11T08:04:45Z","level":"info","trace":"exec","node":"warzone/backend/auth","text":"test coverage now 91%, all OAuth2 conformance tests pass"}
{"type":"marker","timestamp":"2026-04-11T08:05:00Z","level":"info","trace":"exec","marker":"WOLFCASTLE_YIELD","node":"warzone/backend/auth","task":"task-1","text":"yielding for next iteration"}
{"type":"stage_end","timestamp":"2026-04-11T08:05:01Z","level":"info","trace":"exec","stage":"execute","duration_ms":170000,"exit_code":0}
{"type":"intake_start","timestamp":"2026-04-11T08:05:05Z","level":"info","trace":"intake","text":"processing inbox: 3 items pending"}
{"type":"intake_item","timestamp":"2026-04-11T08:05:10Z","level":"info","trace":"intake","text":"filed: Add rate limiting to the API → warzone/backend/api"}
{"type":"intake_end","timestamp":"2026-04-11T08:05:15Z","level":"info","trace":"intake","text":"intake complete: 1 filed, 2 deferred"}
LOGEOF

# Fake daemon instance setup. We start a background sleep process and
# register/deregister the instance around tape runs so only the main
# stage tapes see "hunting" in the header.
sleep 9999 &
FAKE_DAEMON_PID=$!
INSTANCE_DIR="$HOME/.wolfcastle/instances"
mkdir -p "$INSTANCE_DIR"
RESOLVED_STAGE="$(cd "$STAGE_MAIN" && pwd -P)"
INSTANCE_SLUG="$(echo "$RESOLVED_STAGE" | tr '/' '-' | sed 's/^-//')"
INSTANCE_FILE="$INSTANCE_DIR/${INSTANCE_SLUG}.json"

register_fake_daemon() {
    cat > "$INSTANCE_FILE" << EOF
{
  "pid": $FAKE_DAEMON_PID,
  "worktree": "$RESOLVED_STAGE",
  "branch": "main",
  "started_at": "2026-04-11T08:00:00Z"
}
EOF
}

deregister_fake_daemon() {
    rm -f "$INSTANCE_FILE"
}

# Clean up on exit.
cleanup() {
    kill $FAKE_DAEMON_PID 2>/dev/null
    deregister_fake_daemon
    for d in "${cleanup_dirs[@]}"; do
        [[ -n "$d" ]] && rm -rf "$d"
    done
}
trap cleanup EXIT

echo "Main staging:    $STAGE_MAIN"
echo "Main namespace:  $NS_MAIN"
echo "Fake daemon PID: $FAKE_DAEMON_PID"

# ---------------------------------------------------------------------------
# STAGE_COMPLETE: all nodes in "complete" state
# ---------------------------------------------------------------------------
STAGE_COMPLETE="$(make_stage complete)"
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
STAGE_BLOCKED="$(make_stage blocked)"
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
# STAGE_WELCOME: directory with subdirectories but no .wolfcastle/ (welcome screen)
# ---------------------------------------------------------------------------
STAGE_WELCOME="$(make_stage welcome)"
# Create realistic-looking project directories so the browser has content.
mkdir -p "$STAGE_WELCOME/my-saas-app"
mkdir -p "$STAGE_WELCOME/my-saas-app/.wolfcastle"
mkdir -p "$STAGE_WELCOME/internal-tools"
mkdir -p "$STAGE_WELCOME/design-system"
mkdir -p "$STAGE_WELCOME/docs"
echo "Welcome staging:  $STAGE_WELCOME"

# ---------------------------------------------------------------------------
# STAGE_TASK_BLOCKED: small tree with one blocked task showing rich data
# ---------------------------------------------------------------------------
STAGE_TASK_BLOCKED="$(make_stage task-blocked)"
NS_TASK_BLOCKED="$(init_stage "$STAGE_TASK_BLOCKED")"

cat > "$NS_TASK_BLOCKED/state.json" << 'STATEEOF'
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
      "children": ["warzone/payments"]
    },
    "warzone/payments": {
      "name": "payments",
      "type": "leaf",
      "state": "blocked",
      "address": "warzone/payments",
      "parent": "warzone"
    }
  }
}
STATEEOF

create_node "$NS_TASK_BLOCKED" "warzone" "warzone" "orchestrator" "blocked"
create_node "$NS_TASK_BLOCKED" "warzone/payments" "payments" "leaf" "blocked" '[
  {"id":"task-1","title":"Integrate Stripe payment flow","state":"blocked","class":"coding/go","block_reason":"Waiting for Stripe webhook secret from ops team","failure_count":3,"last_failure_type":"dependency"},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'

echo "Task-blocked staging: $STAGE_TASK_BLOCKED"

echo ""

# ---------------------------------------------------------------------------
# Tape routing: map tape names to their staging directories
# ---------------------------------------------------------------------------
stage_for_tape() {
    case "$1" in
        tui-all-complete)     echo "$STAGE_COMPLETE" ;;
        tui-all-blocked)      echo "$STAGE_BLOCKED" ;;
        tui-welcome-sessions) echo "$STAGE_WELCOME" ;;
        tui-task-blocked)     echo "$STAGE_TASK_BLOCKED" ;;
        *)                    echo "$STAGE_MAIN" ;;
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

    # Clean stale lock files from previous VHS runs.
    find "$stage" -name '.lock' -delete 2>/dev/null

    # Register the fake daemon only when running main-stage tapes.
    if [[ "$stage" == "$STAGE_MAIN" ]]; then
        register_fake_daemon
    else
        deregister_fake_daemon
    fi

    echo "Recording: $name (from $stage)"

    # VHS + Bubbletea alt-screen capture is non-deterministic.
    # Retry up to 3 times if the screenshot is too small (blank capture).
    for attempt in 1 2 3; do
        find "$stage" -name '.lock' -delete 2>/dev/null
        (cd "$stage" && vhs "$tape" 2>&1) || {
            echo "  attempt $attempt: vhs failed" >&2
            continue
        }
        if [[ -f "$stage/$name.png" ]] && [[ $(wc -c < "$stage/$name.png") -gt 20000 ]]; then
            break
        fi
        echo "  attempt $attempt: screenshot too small, retrying..." >&2
        sleep 1
    done

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
