#!/usr/bin/env bash
# Parallel 100-scenario runner — 5 tiers in parallel, shared ~/.ws store
set -euo pipefail

RESULTS_FILE="/tmp/ws-scenario-results.txt"
> "$RESULTS_FILE"

DIR="$(dirname "$0")/scenarios"

echo "=== ws 100-Scenario Test Suite (5 parallel tiers) ==="
echo "Version: $(ws --version)"
echo ""

# Run all 5 tiers in parallel
for tier in 1 2 3 4 5; do
  bash "$DIR/tier${tier}.sh" &
done

# Wait for all
wait

echo ""
echo "═══════════════════════════════════════════════════════════"
TOTAL_PASS=$(grep -c '^PASS' "$RESULTS_FILE")
TOTAL_FAIL=$(grep -c '^FAIL' "$RESULTS_FILE")
echo "Results: $TOTAL_PASS PASS, $TOTAL_FAIL FAIL out of $((TOTAL_PASS + TOTAL_FAIL))"
echo "═══════════════════════════════════════════════════════════"
if [ "$TOTAL_FAIL" -gt 0 ]; then
  echo ""
  echo "Failures:"
  grep '^FAIL' "$RESULTS_FILE"
fi
echo ""
echo "Full results: $RESULTS_FILE"
