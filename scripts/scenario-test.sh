#!/usr/bin/env bash
# Parallel 100-scenario runner — 5 tiers in parallel, shared ~/.ws store
set -euo pipefail

RESULTS_FILE="/tmp/ws-scenario-results.txt"
> "$RESULTS_FILE"

DIR="$(cd "$(dirname "$0")" && pwd)/scenarios"

# Resolve WS binary: prefer ./ws (just built by task), fall back to PATH
if [ -x "./ws" ]; then
  WS_BIN="$(pwd)/ws"
elif command -v ws >/dev/null 2>&1; then
  WS_BIN="$(command -v ws)"
else
  echo "error: ws binary not found. Run 'task build' first or install ws."
  exit 1
fi

export WS="$WS_BIN"

# Force copy backend — container backend has machine naming restrictions
# (e.g. underscores not allowed) that conflict with test workspace names
export WS_BACKEND=copy

echo "=== ws 100-Scenario Test Suite (5 parallel tiers) ==="
echo "Binary: $WS_BIN"
echo "Version: $($WS_BIN --version)"
echo ""

# Run all 5 tiers in parallel
for tier in 1 2 3 4 5; do
  bash "$DIR/tier${tier}.sh" &
done

# Wait for all
wait

echo ""
echo "═══════════════════════════════════════════════════════════"
TOTAL_PASS=$(grep -c '^PASS' "$RESULTS_FILE" || true)
TOTAL_FAIL=$(grep -c '^FAIL' "$RESULTS_FILE" || true)
echo "Results: $TOTAL_PASS PASS, $TOTAL_FAIL FAIL out of $((TOTAL_PASS + TOTAL_FAIL))"
echo "═══════════════════════════════════════════════════════════"
if [ "$TOTAL_FAIL" -gt 0 ]; then
  echo ""
  echo "Failures:"
  grep '^FAIL' "$RESULTS_FILE"
fi
echo ""
echo "Full results: $RESULTS_FILE"
