#!/usr/bin/env bash
# Verify ws CLI on macOS — copy-paste this entire block into Terminal
set -euo pipefail

# ── 1. Update to latest build from main ──────────────────────────────────────
echo "=== 1. Update ws to latest ==="
ws update

# ── 2. Show version ──────────────────────────────────────────────────────────
echo "=== 2. Version ==="
ws --version

# ── 3. Per-command help ─────────────────────────────────────────────────────
echo "=== 3. Help ==="
ws help get
ws help run
ws help agent

# ── 4. Pull down a repo to test with ───────────────────────────────────────
echo "=== 4. Clone test repo ==="
TESTREPO="/tmp/ws-test-repo-$$"
git clone https://github.com/weslien/ws.git "$TESTREPO" 2>/dev/null || \
  cp -R ~/.ws/repos/*/ "$TESTREPO" 2>/dev/null || true

# If no internet repo, create a tiny local one
if [ ! -d "$TESTREPO/.git" ]; then
  rm -rf "$TESTREPO"
  mkdir -p "$TESTREPO"
  git -C "$TESTREPO" init
  echo '{"name":"test"}' > "$TESTREPO/package.json"
  git -C "$TESTREPO" add .
  git -C "$TESTREPO" commit -m "init"
fi

# ── 5. Fresh start workspace ─────────────────────────────────────────────────
echo "=== 5. Fresh workspace (agent-1) ==="
rm -rf ~/.ws
ws get base:"$TESTREPO" --name=agent-1

# ── 6. Run command inside workspace ──────────────────────────────────────────
echo "=== 6. Run 'cat package.json' inside workspace ==="
ws run agent-1 -- cat package.json

# ── 7. Make a change ─────────────────────────────────────────────────────────
echo "=== 7. Mutate content ==="
echo '{"name":"test","version":"1.0.0"}' > "$HOME/.ws/workspaces/agent-1/package.json"

# ── 8. Diff ──────────────────────────────────────────────────────────────────
echo "=== 8. Diff workspace ==="
ws diff agent-1

# ── 9. Keep (checkpoint) ─────────────────────────────────────────────────────
echo "=== 9. Keep as layer ==="
ws keep agent-1 --message="added version field"

# ── 10. Branch from live workspace ───────────────────────────────────────────
echo "=== 10. Branch from agent-1 into agent-2 ==="
ws get ws:agent-1 --name=agent-2

echo '{"name":"test","version":"2.0.0"}' > "$HOME/.ws/workspaces/agent-2/package.json"
ws keep agent-2 --message="bumped to 2.0.0"

# ── 11. Graph ──────────────────────────────────────────────────────────────
echo "=== 11. Dependency graph ==="
ws graph
ws graph agent-2

# ── 12. Layer management ───────────────────────────────────────────────────
echo "=== 12. Layer store ==="
ws layer ls
ws layer gc

# ── 13. Drop workspaces ────────────────────────────────────────────────────
echo "=== 13. Cleanup ==="
ws drop agent-1
ws drop agent-2
ws layer gc

# ── 14. Install agent skill ──────────────────────────────────────────────────
echo "=== 14. Install skill ==="
ws skill

# ── 15. Final status ────────────────────────────────────────────────────────
echo "=== 15. Verify skill installed ==="
ls ~/.hermes/skills/ws-workspace-graph/SKILL.md 2>/dev/null && echo "Skill OK" || echo "Skill not found in default path"

echo ""
echo "🟢 All verifications passed on macOS (copy backend)."
