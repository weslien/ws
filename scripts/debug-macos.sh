#!/usr/bin/env bash
# Debug script for macOS layer files issue
# Run: bash scripts/debug-macos.sh
set -x  # trace every command

export WS_BACKEND=copy

# Use a fresh WS_HOME to avoid stale state
export WS_HOME="$HOME/.ws-debug"
rm -rf "$WS_HOME"

WS="$(pwd)/ws"

# Setup base repo
DEMO="/tmp/ws-debug-repo"
rm -rf "$DEMO"
mkdir -p "$DEMO/src"
echo 'module demo' > "$DEMO/go.mod"
echo 'package main' > "$DEMO/src/main.go"
cd "$DEMO"
git init -q
git config user.email "test@ws.dev"
git config user.name "WS Test"
git config commit.gpgsign false
git add -A && git commit -q -m "initial"
cd -

echo "=== Step 1: Create base ==="
$WS get base:"$DEMO" --name=_base --force
echo "x" > "$($WS path _base)/src/x.go"
BASE=$($WS keep _base --json 2>/dev/null | grep '^{' | python3 -c "import json,sys;print(json.load(sys.stdin)['layer'])")
echo "BASE=$BASE"
echo "Base layer dir: $(ls -la $WS_HOME/layers/$BASE/)"
echo "Base layer files: $($WS layer files $BASE)"

echo ""
echo "=== Step 2: Scenario 95 (sparse) ==="
$WS get layer:"$BASE" --name=sp95
echo "ws path sp95: $($WS path sp95)"
echo "Workspace exists: $(test -d $($WS path sp95) && echo YES || echo NO)"
echo "Workspace contents:"
find "$($WS path sp95)" -type f | sort

echo 'sparse' > "$($WS path sp95)/src/sparse.go"
echo "After write, workspace contents:"
find "$($WS path sp95)" -type f | sort

L95=$($WS keep sp95 --json 2>/dev/null | grep '^{' | python3 -c "import json,sys;print(json.load(sys.stdin)['layer'])")
echo "L95=$L95"
echo "Layer dir exists: $(test -d $WS_HOME/layers/$L95 && echo YES || echo NO)"
echo "Layer dir contents:"
ls -la "$WS_HOME/layers/$L95/" 2>&1
find "$WS_HOME/layers/$L95" -type f 2>&1 | sort
echo "ws layer files output:"
$WS layer files "$L95" 2>&1
echo "grep test:"
$WS layer files "$L95" 2>&1 | grep -q sparse && echo "MATCH" || echo "NO MATCH"

echo ""
echo "=== Step 3: Scenario 56 (nested dir) ==="
$WS get layer:"$BASE" --name=t56
WSPATH=$($WS path t56)
echo "WSPATH=$WSPATH"
mkdir -p "$WSPATH/src/deep/nested/path"
echo 'package nested' > "$WSPATH/src/deep/nested/path/file.go"
echo "After write, workspace contents:"
find "$WSPATH" -type f | sort

L56=$($WS keep t56 --json 2>/dev/null | grep '^{' | python3 -c "import json,sys;print(json.load(sys.stdin)['layer'])")
echo "L56=$L56"
echo "Layer dir exists: $(test -d $WS_HOME/layers/$L56 && echo YES || echo NO)"
echo "Layer dir contents:"
find "$WS_HOME/layers/$L56" -type f 2>&1 | sort
echo "ws layer files output:"
$WS layer files "$L56" 2>&1
echo "grep test:"
$WS layer files "$L56" 2>&1 | grep -q 'deep/nested/path/file.go' && echo "MATCH" || echo "NO MATCH"

echo ""
echo "=== Step 4: Scenario 96b (audit) ==="
$WS get layer:"$BASE" --name=au96
echo 'audit' > "$($WS path au96)/src/audit.go"
L96=$($WS keep au96 --json 2>/dev/null | grep '^{' | python3 -c "import json,sys;print(json.load(sys.stdin)['layer'])")
echo "L96=$L96"
echo "Layer dir exists: $(test -d $WS_HOME/layers/$L96 && echo YES || echo NO)"
echo "Layer dir contents:"
find "$WS_HOME/layers/$L96" -type f 2>&1 | sort
echo "ws layer files output:"
$WS layer files "$L96" 2>&1
echo "grep test:"
$WS layer files "$L96" 2>&1 | grep -q audit.go && echo "MATCH" || echo "NO MATCH"

echo ""
echo "=== Step 5: layers.json ==="
cat "$WS_HOME/meta/layers.json" 2>&1 | python3 -m json.tool 2>&1

echo ""
echo "=== Step 6: workspaces.json ==="
cat "$WS_HOME/meta/workspaces.json" 2>&1 | python3 -m json.tool 2>&1

# Cleanup
$WS drop _base sp95 t56 au96 2>/dev/null || true
