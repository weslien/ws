#!/usr/bin/env bash
# Shared setup for parallel scenario tiers
# Sourced by tier1.sh through tier5.sh
set -uo pipefail  # no -e: test helpers catch failures themselves

WS="${WS:-ws}"
DEMO="/tmp/ws-scenario-repo"
RESULTS_FILE="/tmp/ws-scenario-results.txt"
LOCK_FILE="/tmp/ws-scenario-setup.lock"
PASS=0; FAIL=0

JQ_LAYER() {
  grep '^{' | python3 -c "import json,sys;print(json.load(sys.stdin)['layer'])"
}

t() {
  local num="$1" name="$2" tag="$3"; shift 3
  if output=$("$@" 2>&1); then
    echo "PASS $num $tag $name" >> "$RESULTS_FILE"
    PASS=$((PASS+1))
  else
    echo "FAIL $num $tag $name :: ${output:0:200}" >> "$RESULTS_FILE"
    FAIL=$((FAIL+1))
  fi
}

t_fail() {
  local num="$1" name="$2" tag="$3"; shift 3
  if output=$("$@" 2>&1); then
    echo "FAIL $num $tag $name :: expected failure but succeeded" >> "$RESULTS_FILE"
    FAIL=$((FAIL+1))
  else
    echo "PASS $num $tag $name" >> "$RESULTS_FILE"
    PASS=$((PASS+1))
  fi
}

t_cond() {
  local num="$1" name="$2" tag="$3" cond="$4"
  if eval "$cond" 2>/dev/null; then
    echo "PASS $num $tag $name" >> "$RESULTS_FILE"
    PASS=$((PASS+1))
  else
    echo "FAIL $num $tag $name :: condition false: $cond" >> "$RESULTS_FILE"
    FAIL=$((FAIL+1))
  fi
}

# One-time setup: create demo repo + seed base layer
ensure_setup() {
  exec 9>"$LOCK_FILE"
  flock 9
  if [ ! -d "$DEMO/.git" ]; then
    mkdir -p "$DEMO/src/api" "$DEMO/src/db" "$DEMO/test"
    cat > "$DEMO/go.mod" << 'EOF'
module demo
go 1.23
EOF
    cat > "$DEMO/src/main.go" << 'EOF'
package main
import (
  "net/http"
  "demo/src/api"
)
func main() {
  mux := http.NewServeMux()
  api.RegisterRoutes(mux)
  http.ListenAndServe(":8080", mux)
}
EOF
    cat > "$DEMO/src/api/routes.go" << 'EOF'
package api
import "net/http"
func RegisterRoutes(mux *http.ServeMux) {
  mux.HandleFunc("/add", handleAdd)
  mux.HandleFunc("/subtract", handleSubtract)
}
func handleAdd(w http.ResponseWriter, r *http.Request)      { w.Write([]byte("stub")) }
func handleSubtract(w http.ResponseWriter, r *http.Request)  { w.Write([]byte("stub")) }
EOF
    cat > "$DEMO/src/db/store.go" << 'EOF'
package db
type Store struct{}
func NewStore() *Store { return &Store{} }
func (s *Store) Get(key string) string { return "" }
EOF
    cat > "$DEMO/test/routes_test.go" << 'EOF'
package api
import "testing"
func TestRoutes(t *testing.T) {
  if RegisterRoutes == nil { t.Error("nil") }
}
EOF
    cat > "$DEMO/README.md" << 'EOF'
# Demo API
Calculator with add/subtract endpoints.
EOF
    git -C "$DEMO" init -q
    git -C "$DEMO" config user.email "test@ws.dev"
    git -C "$DEMO" config user.name "WS Test"
    git -C "$DEMO" config commit.gpgsign false
    git -C "$DEMO" add -A
    git -C "$DEMO" commit -q -m "initial"
  fi
  # Ensure base layer exists — keep _base workspace persistent (don't drop)
  if ! $WS status 2>/dev/null | grep -q '_base'; then
    $WS get base:"$DEMO" --name=_base >/dev/null 2>&1
    echo "x" > "$($WS path _base)/src/x.go"
    BASE_LAYER=$($WS keep _base --json 2>/dev/null | JQ_LAYER)
  else
    # _base workspace already exists (created by another tier)
    BASE_LAYER=$($WS status --json 2>/dev/null | python3 -c "
import json,sys
d=json.load(sys.stdin)
for w in d.get('workspaces',d) if isinstance(d,dict) else d:
  if w.get('workspace')=='_base' or w.get('name')=='_base': print(w.get('layer','')); break
" 2>/dev/null || echo "")
  fi
  export BASE_LAYER
  flock -u 9
}
