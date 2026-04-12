#!/usr/bin/env bash
#
# Generate TUI screenshots using VHS tape files.
#
# Usage: ./generate.sh [binary-path]
#
# If no binary path is given, builds wolfcastle to a temp location.
# Requires: vhs (brew install vhs)
#
# Architecture: each screenshot group gets its own staging directory with
# a purpose-built .wolfcastle/ state. Daemons are started/stopped outside
# VHS so the header status is deterministic.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TAPE_DIR="$SCRIPT_DIR/tapes"
OUT_DIR="$SCRIPT_DIR"

# ---------------------------------------------------------------------------
# Binary
# ---------------------------------------------------------------------------
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
# Staging directory tracking
# ---------------------------------------------------------------------------
cleanup_dirs=()
DAEMON_PIDS=()  # array of "dir" entries where daemons are running

make_stage() {
    local label="${1:-demo}"
    local d="/tmp/wolfcastle-vhs-${label}"
    rm -rf "$d"
    mkdir -p "$d"
    cleanup_dirs+=("$d")
    echo "$d"
}

# ---------------------------------------------------------------------------
# init_stage: initialize a .wolfcastle/ dir and return the namespace path
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
# create_node: write a node state file inside a namespace directory
# ---------------------------------------------------------------------------
create_node() {
    local ns_dir="$1" addr="$2" name="$3" type="$4" nstate="$5"
    shift 5
    local dir="$ns_dir/$addr"
    mkdir -p "$dir"
    local tasks="${1:-}"
    local children="${2:-}"
    # Build JSON with optional tasks and children fields
    local json="{\n  \"name\": \"$name\",\n  \"type\": \"$type\",\n  \"state\": \"$nstate\""
    if [[ -n "$tasks" ]]; then
        json="$json,\n  \"tasks\": $tasks"
    fi
    if [[ -n "$children" ]]; then
        json="$json,\n  \"children\": $children"
    fi
    json="$json\n}"
    printf "$json" > "$dir/state.json"
}

# ---------------------------------------------------------------------------
# Daemon management
# ---------------------------------------------------------------------------
# configure_stall_model writes a mock model config that sleeps forever.
# The daemon shows "hunting" status without modifying the fake state.
configure_stall_model() {
    local dir="$1"
    local custom_dir="$dir/.wolfcastle/system/custom"
    mkdir -p "$custom_dir"
    cat > "$custom_dir/config.json" << 'CFGEOF'
{
  "models": {
    "fast":  {"command": "sleep", "args": ["3600"]},
    "mid":   {"command": "sleep", "args": ["3600"]},
    "heavy": {"command": "sleep", "args": ["3600"]}
  },
  "daemon": {
    "invocation_timeout_seconds": 3600,
    "stall_timeout_seconds": 3600
  },
  "git": {
    "auto_commit": false,
    "verify_branch": false
  }
}
CFGEOF
}

start_daemon_in() {
    local dir="$1"
    configure_stall_model "$dir"
    (cd "$dir" && wolfcastle start -d 2>/dev/null) || true
    sleep 3
    # Verify it's actually running (daemon field in status JSON)
    if (cd "$dir" && wolfcastle status --json 2>/dev/null | grep -q '"daemon": "hunting"'); then
        DAEMON_PIDS+=("$dir")
        echo "  daemon started in $dir (hunting)"
    else
        # Even if not "hunting" yet, add it so we clean up on exit
        DAEMON_PIDS+=("$dir")
        echo "  daemon started in $dir (may still be initializing)"
    fi
}

stop_daemon_in() {
    local dir="$1"
    (cd "$dir" && wolfcastle stop 2>/dev/null) || true
    for i in 1 2 3 4 5; do
        if ! (cd "$dir" && wolfcastle status --json 2>/dev/null | grep -q '"daemon": "hunting"'); then
            break
        fi
        sleep 1
    done
    # Remove from tracking
    local new=()
    for d in "${DAEMON_PIDS[@]+"${DAEMON_PIDS[@]}"}"; do
        [[ "$d" != "$dir" ]] && new+=("$d")
    done
    DAEMON_PIDS=("${new[@]+"${new[@]}"}")
}

stop_all_daemons() {
    for d in "${DAEMON_PIDS[@]+"${DAEMON_PIDS[@]}"}"; do
        (cd "$d" && wolfcastle stop 2>/dev/null) || true
    done
    sleep 2
    DAEMON_PIDS=()
}

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------
cleanup() {
    stop_all_daemons
    for d in "${cleanup_dirs[@]+"${cleanup_dirs[@]}"}"; do
        [[ -n "$d" ]] && rm -rf "$d"
    done
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Run a single tape with retry logic
# ---------------------------------------------------------------------------
run_tape() {
    local tape="$1" stage="$2" name="$3"
    local min_bytes=20000
    echo "Recording: $name (from $stage)"
    for attempt in 1 2 3 4 5; do
        find "$stage" -name '.lock' -delete 2>/dev/null || true
        (cd "$stage" && vhs "$tape" 2>&1) || {
            echo "  attempt $attempt: vhs failed" >&2
            sleep 2
            continue
        }
        if [[ -f "$stage/$name.png" ]] && [[ $(wc -c < "$stage/$name.png") -gt $min_bytes ]]; then
            mv "$stage/$name.png" "$OUT_DIR/$name.png"
            echo "  -> $OUT_DIR/$name.png ($(wc -c < "$OUT_DIR/$name.png" | tr -d ' ') bytes)"
            return 0
        fi
        local sz=0
        [[ -f "$stage/$name.png" ]] && sz=$(wc -c < "$stage/$name.png" | tr -d ' ')
        echo "  attempt $attempt: screenshot ${sz} bytes (need >${min_bytes}), retrying..." >&2
        sleep 2
    done
    # If we got a file but it's just small, keep it anyway as a fallback
    if [[ -f "$stage/$name.png" ]]; then
        mv "$stage/$name.png" "$OUT_DIR/$name.png"
        echo "  WARNING: $name.png captured but small ($(wc -c < "$OUT_DIR/$name.png" | tr -d ' ') bytes)" >&2
        return 0
    fi
    echo "  FAILED: $name.png not captured after 5 attempts" >&2
    return 1
}


# ===========================================================================
#
#  STAGE 1: MAIN (rich tree with inbox, logs, daemon)
#
#  Used by: tui-full-layout, tui-tree-expanded, tui-dashboard-active,
#           tui-node-detail, tui-task-detail, tui-inbox-modal,
#           tui-inbox-input, tui-log-modal, tui-log-filtered,
#           tui-help-overlay
#
# ===========================================================================
echo ""
echo "=== Setting up MAIN stage ==="
STAGE_MAIN="$(make_stage main)"
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

create_node "$NS_MAIN" "warzone" "warzone" "orchestrator" "in_progress" '' '[
  {"id":"backend","address":"warzone/backend","state":"in_progress"},
  {"id":"frontend","address":"warzone/frontend","state":"in_progress"},
  {"id":"infra","address":"warzone/infra","state":"not_started"}
]'
create_node "$NS_MAIN" "warzone/backend" "backend" "orchestrator" "in_progress" '' '[
  {"id":"api","address":"warzone/backend/api","state":"complete"},
  {"id":"auth","address":"warzone/backend/auth","state":"in_progress"},
  {"id":"database","address":"warzone/backend/database","state":"not_started"}
]'
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
create_node "$NS_MAIN" "warzone/frontend" "frontend" "orchestrator" "in_progress" '' '[
  {"id":"components","address":"warzone/frontend/components","state":"complete"},
  {"id":"routing","address":"warzone/frontend/routing","state":"in_progress"}
]'
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

# Inbox
cat > "$NS_MAIN/inbox.json" << 'INBOXEOF'
{
  "items": [
    {"id":"inbox-001","text":"Add rate limiting to the API","status":"new","created_at":"2026-04-11T10:00:00Z"},
    {"id":"inbox-002","text":"Set up CI/CD pipeline with GitHub Actions","status":"new","created_at":"2026-04-11T10:05:00Z"},
    {"id":"inbox-003","text":"Add OpenTelemetry tracing","status":"filed","created_at":"2026-04-11T09:30:00Z"}
  ]
}
INBOXEOF

# Log data
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
{"type":"intake_item","timestamp":"2026-04-11T08:05:10Z","level":"info","trace":"intake","text":"filed: Add rate limiting to the API -> warzone/backend/api"}
{"type":"intake_end","timestamp":"2026-04-11T08:05:15Z","level":"info","trace":"intake","text":"intake complete: 1 filed, 2 deferred"}
LOGEOF

echo "  Main stage:     $STAGE_MAIN"
echo "  Main namespace: $NS_MAIN"


# ===========================================================================
#
#  STAGE 2: WELCOME (no .wolfcastle/, daemon runs in MAIN stage)
#
#  Used by: tui-welcome-sessions
#
# ===========================================================================
echo ""
echo "=== Setting up WELCOME stage ==="
STAGE_WELCOME="$(make_stage welcome)"
# No .wolfcastle/ here. Just subdirectories for the directory browser.
mkdir -p "$STAGE_WELCOME/my-saas-app/.wolfcastle"
mkdir -p "$STAGE_WELCOME/design-system"
mkdir -p "$STAGE_WELCOME/internal-tools"
mkdir -p "$STAGE_WELCOME/docs"
echo "  Welcome stage:  $STAGE_WELCOME"


# ===========================================================================
#
#  STAGE 3: ALL-COMPLETE (every node complete, no daemon)
#
#  Used by: tui-all-complete
#
# ===========================================================================
echo ""
echo "=== Setting up ALL-COMPLETE stage ==="
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

create_node "$NS_COMPLETE" "warzone" "warzone" "orchestrator" "complete" '' '[
  {"id":"backend","address":"warzone/backend","state":"complete"},
  {"id":"frontend","address":"warzone/frontend","state":"complete"},
  {"id":"infra","address":"warzone/infra","state":"complete"}
]'
create_node "$NS_COMPLETE" "warzone/backend" "backend" "orchestrator" "complete" '' '[
  {"id":"api","address":"warzone/backend/api","state":"complete"},
  {"id":"auth","address":"warzone/backend/auth","state":"complete"},
  {"id":"database","address":"warzone/backend/database","state":"complete"}
]'
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
create_node "$NS_COMPLETE" "warzone/frontend" "frontend" "orchestrator" "complete" '' '[
  {"id":"components","address":"warzone/frontend/components","state":"complete"},
  {"id":"routing","address":"warzone/frontend/routing","state":"complete"}
]'
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

echo "  Complete stage:  $STAGE_COMPLETE"


# ===========================================================================
#
#  STAGE 4: ALL-BLOCKED (every node blocked, no daemon)
#
#  Used by: tui-all-blocked
#
# ===========================================================================
echo ""
echo "=== Setting up ALL-BLOCKED stage ==="
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

create_node "$NS_BLOCKED" "warzone" "warzone" "orchestrator" "blocked" '' '[
  {"id":"backend","address":"warzone/backend","state":"blocked"},
  {"id":"frontend","address":"warzone/frontend","state":"blocked"},
  {"id":"infra","address":"warzone/infra","state":"blocked"}
]'
create_node "$NS_BLOCKED" "warzone/backend" "backend" "orchestrator" "blocked" '' '[
  {"id":"api","address":"warzone/backend/api","state":"blocked"},
  {"id":"auth","address":"warzone/backend/auth","state":"blocked"},
  {"id":"database","address":"warzone/backend/database","state":"blocked"}
]'
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
create_node "$NS_BLOCKED" "warzone/frontend" "frontend" "orchestrator" "blocked" '' '[
  {"id":"components","address":"warzone/frontend/components","state":"blocked"},
  {"id":"routing","address":"warzone/frontend/routing","state":"blocked"}
]'
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

echo "  Blocked stage:   $STAGE_BLOCKED"


# ===========================================================================
#
#  STAGE 5: TASK-BLOCKED (one blocked task with rich failure data, no daemon)
#
#  Used by: tui-task-blocked
#
# ===========================================================================
echo ""
echo "=== Setting up TASK-BLOCKED stage ==="
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

create_node "$NS_TASK_BLOCKED" "warzone" "warzone" "orchestrator" "blocked" '' '[
  {"id":"payments","address":"warzone/payments","state":"blocked"}
]'
create_node "$NS_TASK_BLOCKED" "warzone/payments" "payments" "leaf" "blocked" '[
  {"id":"task-1","title":"Integrate Stripe payment flow","state":"blocked","class":"coding/go","block_reason":"Waiting for Stripe webhook secret from ops team","failure_count":3,"last_failure_type":"dependency"},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'

echo "  Task-blocked stage: $STAGE_TASK_BLOCKED"


# ===========================================================================
#
#  STAGE 6: DAEMON-START (valid state, no daemon, for START DAEMON modal)
#
#  Used by: tui-daemon-start
#
# ===========================================================================
echo ""
echo "=== Setting up DAEMON-START stage ==="
STAGE_DAEMON_START="$(make_stage daemon-start)"
NS_DAEMON_START="$(init_stage "$STAGE_DAEMON_START")"

cat > "$NS_DAEMON_START/state.json" << 'STATEEOF'
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
      "children": ["warzone/backend"]
    },
    "warzone/backend": {
      "name": "backend",
      "type": "leaf",
      "state": "in_progress",
      "address": "warzone/backend",
      "parent": "warzone"
    }
  }
}
STATEEOF

create_node "$NS_DAEMON_START" "warzone" "warzone" "orchestrator" "in_progress" '' '[
  {"id":"backend","address":"warzone/backend","state":"in_progress"}
]'
create_node "$NS_DAEMON_START" "warzone/backend" "backend" "leaf" "in_progress" '[
  {"id":"task-1","title":"Implement REST endpoints","state":"in_progress","class":"coding/go"},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'

echo "  Daemon-start stage: $STAGE_DAEMON_START"


# ===========================================================================
#
#  STAGE 7: SEARCH (purpose-built tree for search highlighting)
#
#  Used by: tui-search-active
#
#  Tree designed so "auth" matches:
#    warzone       olive (ancestor)
#    backend       olive (ancestor, expanded)
#      api         unhighlighted
#      auth        YELLOW (literal leaf match, expanded)
#        -> PKCE   YELLOW (literal task match, contains "auth")
#        -> Session unhighlighted task
#        -> Audit  unhighlighted
#      database    unhighlighted
#    frontend      olive (ancestor, COLLAPSED, contains auth-gateway)
#    infra         unhighlighted
#
# ===========================================================================
echo ""
echo "=== Setting up SEARCH stage ==="
STAGE_SEARCH="$(make_stage search)"
NS_SEARCH="$(init_stage "$STAGE_SEARCH")"

cat > "$NS_SEARCH/state.json" << 'STATEEOF'
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
      "children": ["warzone/frontend/components", "warzone/frontend/auth-gateway"]
    },
    "warzone/frontend/components": {
      "name": "components",
      "type": "leaf",
      "state": "complete",
      "address": "warzone/frontend/components",
      "parent": "warzone/frontend"
    },
    "warzone/frontend/auth-gateway": {
      "name": "auth-gateway",
      "type": "leaf",
      "state": "not_started",
      "address": "warzone/frontend/auth-gateway",
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

create_node "$NS_SEARCH" "warzone" "warzone" "orchestrator" "in_progress" '' '[
  {"id":"backend","address":"warzone/backend","state":"in_progress"},
  {"id":"frontend","address":"warzone/frontend","state":"in_progress"},
  {"id":"infra","address":"warzone/infra","state":"not_started"}
]'
create_node "$NS_SEARCH" "warzone/backend" "backend" "orchestrator" "in_progress" '' '[
  {"id":"api","address":"warzone/backend/api","state":"complete"},
  {"id":"auth","address":"warzone/backend/auth","state":"in_progress"},
  {"id":"database","address":"warzone/backend/database","state":"not_started"}
]'
create_node "$NS_SEARCH" "warzone/backend/api" "api" "leaf" "complete" '[
  {"id":"task-1","title":"Implement REST endpoints","state":"complete"},
  {"id":"audit","title":"Audit","state":"complete","is_audit":true}
]'
create_node "$NS_SEARCH" "warzone/backend/auth" "auth" "leaf" "in_progress" '[
  {"id":"task-1","title":"Add OAuth2 PKCE auth flow","state":"in_progress","description":"Implement authorization code flow with PKCE"},
  {"id":"task-2","title":"Session token rotation","state":"not_started"},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'
create_node "$NS_SEARCH" "warzone/backend/database" "database" "leaf" "not_started" '[
  {"id":"task-1","title":"Schema migration framework","state":"not_started"},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'
create_node "$NS_SEARCH" "warzone/frontend" "frontend" "orchestrator" "in_progress" '' '[
  {"id":"components","address":"warzone/frontend/components","state":"complete"},
  {"id":"auth-gateway","address":"warzone/frontend/auth-gateway","state":"not_started"}
]'
create_node "$NS_SEARCH" "warzone/frontend/components" "components" "leaf" "complete" '[
  {"id":"task-1","title":"Build component library","state":"complete"},
  {"id":"audit","title":"Audit","state":"complete","is_audit":true}
]'
create_node "$NS_SEARCH" "warzone/frontend/auth-gateway" "auth-gateway" "leaf" "not_started" '[
  {"id":"task-1","title":"Auth gateway integration","state":"not_started"},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'
create_node "$NS_SEARCH" "warzone/infra" "infra" "leaf" "not_started" '[
  {"id":"task-1","title":"Terraform deployment","state":"not_started"},
  {"id":"audit","title":"Audit","state":"not_started","is_audit":true}
]'

echo "  Search stage:    $STAGE_SEARCH"

echo ""
echo "All 7 stages ready."
echo ""

# ===========================================================================
#
#  TAPE EXECUTION
#
#  Group tapes by their staging directory and daemon requirements to
#  minimize daemon start/stop cycles.
#
# ===========================================================================

SUCCESS=0
FAILED=0

# ---------------------------------------------------------------------------
# Group 1: MAIN stage tapes (daemon running in STAGE_MAIN)
# ---------------------------------------------------------------------------
echo "=== Starting daemon for MAIN group ==="
start_daemon_in "$STAGE_MAIN"

# ---------------------------------------------------------------------------
# Run welcome tape first while the daemon is freshly registered in the
# instance registry (the welcome screen's sessions panel needs it).
# ---------------------------------------------------------------------------
echo ""
echo "=== WELCOME tape (daemon fresh in MAIN for sessions) ==="
tape="$TAPE_DIR/tui-welcome-sessions.tape"
if run_tape "$tape" "$STAGE_WELCOME" "tui-welcome-sessions"; then
    SUCCESS=$((SUCCESS + 1))
else
    FAILED=$((FAILED + 1))
fi

MAIN_TAPES=(
    tui-full-layout
    tui-tree-expanded
    tui-dashboard-active
    tui-node-detail
    tui-task-detail
    tui-inbox-modal
    tui-inbox-input
    tui-log-modal
    tui-log-filtered
    tui-help-overlay
)

for name in "${MAIN_TAPES[@]}"; do
    tape="$TAPE_DIR/$name.tape"
    if [[ ! -f "$tape" ]]; then
        echo "WARNING: tape not found: $tape" >&2
        FAILED=$((FAILED + 1))
        continue
    fi
    if run_tape "$tape" "$STAGE_MAIN" "$name"; then
        SUCCESS=$((SUCCESS + 1))
    else
        FAILED=$((FAILED + 1))
    fi
done

# ---------------------------------------------------------------------------
# Stop the main daemon before running no-daemon groups
# ---------------------------------------------------------------------------
echo ""
echo "=== Stopping MAIN daemon ==="
stop_daemon_in "$STAGE_MAIN"

# ---------------------------------------------------------------------------
# Group 3: ALL-COMPLETE (no daemon = "standing down")
# ---------------------------------------------------------------------------
echo ""
echo "=== ALL-COMPLETE tape (no daemon) ==="
tape="$TAPE_DIR/tui-all-complete.tape"
if run_tape "$tape" "$STAGE_COMPLETE" "tui-all-complete"; then
    SUCCESS=$((SUCCESS + 1))
else
    FAILED=$((FAILED + 1))
fi

# ---------------------------------------------------------------------------
# Group 4: ALL-BLOCKED (no daemon = "standing down")
# ---------------------------------------------------------------------------
echo ""
echo "=== ALL-BLOCKED tape (no daemon) ==="
tape="$TAPE_DIR/tui-all-blocked.tape"
if run_tape "$tape" "$STAGE_BLOCKED" "tui-all-blocked"; then
    SUCCESS=$((SUCCESS + 1))
else
    FAILED=$((FAILED + 1))
fi

# ---------------------------------------------------------------------------
# Group 5: TASK-BLOCKED (no daemon)
# ---------------------------------------------------------------------------
echo ""
echo "=== TASK-BLOCKED tape (no daemon) ==="
tape="$TAPE_DIR/tui-task-blocked.tape"
if run_tape "$tape" "$STAGE_TASK_BLOCKED" "tui-task-blocked"; then
    SUCCESS=$((SUCCESS + 1))
else
    FAILED=$((FAILED + 1))
fi

# ---------------------------------------------------------------------------
# Group 6: DAEMON-START (no daemon, shows START DAEMON modal)
# ---------------------------------------------------------------------------
echo ""
echo "=== DAEMON-START tape (no daemon) ==="
tape="$TAPE_DIR/tui-daemon-start.tape"
if run_tape "$tape" "$STAGE_DAEMON_START" "tui-daemon-start"; then
    SUCCESS=$((SUCCESS + 1))
else
    FAILED=$((FAILED + 1))
fi

# ---------------------------------------------------------------------------
# Group 7: SEARCH (needs its own daemon)
# ---------------------------------------------------------------------------
echo ""
echo "=== Starting daemon for SEARCH group ==="
start_daemon_in "$STAGE_SEARCH"

tape="$TAPE_DIR/tui-search-active.tape"
if run_tape "$tape" "$STAGE_SEARCH" "tui-search-active"; then
    SUCCESS=$((SUCCESS + 1))
else
    FAILED=$((FAILED + 1))
fi

stop_daemon_in "$STAGE_SEARCH"

# ===========================================================================
# Summary
# ===========================================================================
echo ""
echo "Done: $SUCCESS succeeded, $FAILED failed (16 expected)"
echo "Screenshots written to: $OUT_DIR/"
ls -1 "$OUT_DIR"/*.png 2>/dev/null || echo "(no screenshots generated)"
