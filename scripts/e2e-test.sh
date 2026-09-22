#!/usr/bin/env bash
# End-to-end lifecycle test for ws CLI
# Works on Linux (overlayfs) and macOS (copy backend)
set -e

WS_BIN="${WS_BIN:-ws}"
TESTREPO="/tmp/ws-test-repo-$$"
PASS=0
FAIL=0

ok()   { echo "  ✓ $1"; PASS=$((PASS+1)); }
fail() { echo "  ✗ $1"; FAIL=$((FAIL+1)); }
section() { echo ""; echo "── $1 ──"; }

cleanup() {
  for ws in agent-1 agent-2 merger; do
    "$WS_BIN" drop "$ws" 2>/dev/null || true
  done
  rm -rf "$TESTREPO" 2>/dev/null || true
  rm -rf ~/.ws 2>/dev/null || true
}
trap cleanup EXIT

section "Version"
"$WS_BIN" --version && ok "version" || fail "version"

section "Help"
"$WS_BIN" help get >/dev/null && ok "help get" || fail "help get"
"$WS_BIN" help run >/dev/null && ok "help run" || fail "help run"
"$WS_BIN" help agent >/dev/null && ok "help agent" || fail "help agent"
"$WS_BIN" help status >/dev/null && ok "help status" || fail "help status"
"$WS_BIN" help layer >/dev/null && ok "help layer" || fail "help layer"

section "Setup test repo"
rm -rf "$TESTREPO"
mkdir -p "$TESTREPO"
git -C "$TESTREPO" init
echo '{"name":"test"}' > "$TESTREPO/package.json"
git -C "$TESTREPO" add .
git -C "$TESTREPO" commit -m "init" --no-gpg-sign 2>/dev/null
ok "test repo created"

section "Fresh workspace"
rm -rf ~/.ws
"$WS_BIN" get base:"$TESTREPO" --name=agent-1 && ok "get base:" || fail "get base:"

section "Run inside workspace"
"$WS_BIN" run agent-1 -- cat package.json && ok "run" || fail "run"

section "Mutate + diff"
echo '{"name":"test","version":"1.0.0"}' > "$HOME/.ws/workspaces/agent-1/package.json"
"$WS_BIN" diff agent-1 && ok "diff" || fail "diff"

section "Keep"
"$WS_BIN" keep agent-1 --message="added version" && ok "keep" || fail "keep"
LAYER_A=$("$WS_BIN" layer ls | grep "added version" | awk '{print $1}')
echo "  layer: $LAYER_A"

section "Branch from live workspace"
"$WS_BIN" get ws:agent-1 --name=agent-2 && ok "get ws:" || fail "get ws:"
echo '{"name":"test","version":"2.0.0"}' > "$HOME/.ws/workspaces/agent-2/package.json"
"$WS_BIN" keep agent-2 --message="bumped to 2.0.0" && ok "keep agent-2" || fail "keep agent-2"
LAYER_B=$("$WS_BIN" layer ls | grep "bumped to 2.0.0" | awk '{print $1}')
echo "  layer: $LAYER_B"

section "Graph"
"$WS_BIN" graph && ok "graph" || fail "graph"
"$WS_BIN" graph agent-2 && ok "graph agent-2" || fail "graph agent-2"

section "Status"
"$WS_BIN" status && ok "status" || fail "status"

section "Layer operations"
"$WS_BIN" layer ls && ok "layer ls" || fail "layer ls"
"$WS_BIN" layer show "$LAYER_A" && ok "layer show" || fail "layer show"
"$WS_BIN" layer diff "$LAYER_A" "$LAYER_B" && ok "layer diff" || fail "layer diff"
"$WS_BIN" layer cat "$LAYER_A" package.json && ok "layer cat" || fail "layer cat"
"$WS_BIN" layer path "$LAYER_A" && ok "layer path" || fail "layer path"
"$WS_BIN" layer files "$LAYER_A" && ok "layer files" || fail "layer files"

section "Force replace"
"$WS_BIN" get base:"$TESTREPO" --name=agent-1 --force && ok "get --force" || fail "get --force"

section "Merge workflow"
BASE=$("$WS_BIN" layer ls | grep "base:" | head -1 | awk '{print $1}')
"$WS_BIN" get layer:"$BASE" --name=merger && ok "get layer: for merge" || fail "get layer: for merge"
"$WS_BIN" layer cat "$LAYER_A" package.json > /tmp/merge-ours.json
"$WS_BIN" layer cat "$LAYER_B" package.json > /tmp/merge-theirs.json
"$WS_BIN" layer cat "$BASE" package.json > /tmp/merge-base.json
cp /tmp/merge-theirs.json "$HOME/.ws/workspaces/merger/package.json"
"$WS_BIN" keep merger --message="manual merge" && ok "keep merger" || fail "keep merger"

section "Cleanup"
"$WS_BIN" drop agent-1 && ok "drop agent-1" || fail "drop agent-1"
"$WS_BIN" drop agent-2 && ok "drop agent-2" || fail "drop agent-2"
"$WS_BIN" drop merger && ok "drop merger" || fail "drop merger"
"$WS_BIN" layer gc && ok "layer gc" || fail "layer gc"

section "Skill"
"$WS_BIN" skill && ok "skill install" || fail "skill install"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  PASS: $PASS  FAIL: $FAIL"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
echo "  All tests passed."
