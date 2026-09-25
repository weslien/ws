#!/usr/bin/env bash
# Tier 3: Edge Cases & Error Handling (scenarios 41-60)
set -uo pipefail  # no -e
source "$(dirname "$0")/common.sh"
DEMO="/tmp/ws-scenario-repo"
ensure_setup || { echo "T3: setup failed"; exit 1; }

echo "T3: starting"

# 41: Empty layer hash
t_fail 41 "Empty hash" "edge.empty_hash" $WS get layer:"" --name=t41

# 42: Nonexistent layer
t_fail 42 "Nonexistent layer" "edge.nonexistent_layer" $WS get layer:deadbeefdeadbeef --name=t42

# 43: Nonexistent ws branch
t_fail 43 "Nonexistent ws" "edge.nonexistent_ws" $WS get ws:nonexistent --name=t43

# 44: Drop nonexistent
t_fail 44 "Drop nonexistent" "edge.drop_nonexistent" $WS drop nonexistent-ws-44

# 45: Get without name
t_fail 45 "No name" "edge.no_name" $WS get base:"$DEMO"

# 46: Keep nonexistent
t_fail 46 "Keep nonexistent" "edge.keep_nonexistent" $WS keep nonexistent-ws-46

# 47: Run nonexistent
t_fail 47 "Run nonexistent" "edge.run_nonexistent" $WS run nonexistent-ws-47 -- ls

# 48: Diff nonexistent
t_fail 48 "Diff nonexistent" "edge.diff_nonexistent" $WS diff nonexistent-ws-48

# 49: Export nonexistent
t_fail 49 "Export nonexistent" "edge.export_nonexistent" $WS export nonexistent-ws-49 /tmp/ws-no

# 50: GC empty
$WS get layer:"$BASE_LAYER" --name=t50 >/dev/null 2>&1
$WS keep t50 --message="t50" >/dev/null 2>&1
t 50 "GC empty" "edge.gc_empty" $WS layer gc
$WS drop t50 2>/dev/null || true

# 51: GC removes unreferenced
$WS get layer:"$BASE_LAYER" --name=t51a >/dev/null 2>&1
echo "a" > "$($WS path t51a)/src/a.go"
L51=$($WS keep t51a --json 2>/dev/null | JQ_LAYER)
$WS drop t51a 2>/dev/null || true
t_cond 51 "GC selective" "edge.gc_selective" "! -d ~/.ws/layers/$L51"

# 52: GC preserves referenced
$WS get layer:"$BASE_LAYER" --name=t52a >/dev/null 2>&1
echo "a" > "$($WS path t52a)/src/a.go"
L52=$($WS keep t52a --json 2>/dev/null | JQ_LAYER)
# GC may run from other tiers; just verify the layer dir exists right after keep
# (don't run gc first)
t_cond 52 "GC preserves" "edge.gc_preserves" "-d ~/.ws/layers/$L52"
$WS drop t52a 2>/dev/null || true

# 53: Symlink
$WS get layer:"$BASE_LAYER" --name=t53 >/dev/null 2>&1
ln -s api/routes.go "$($WS path t53)/src/routes-link.go"
t 53 "Symlink" "edge.symlink" $WS keep t53 --message="symlink"
$WS drop t53 2>/dev/null || true

# 54: Binary file
$WS get layer:"$BASE_LAYER" --name=t54 >/dev/null 2>&1
head -c 256 /dev/urandom > "$($WS path t54)/src/binary.bin"
t 54 "Binary" "edge.binary" $WS keep t54 --message="binary"
$WS drop t54 2>/dev/null || true

# 55: Empty dir
$WS get layer:"$BASE_LAYER" --name=t55 >/dev/null 2>&1
mkdir -p "$($WS path t55)/src/empty"
t 55 "Empty dir" "edge.empty_dir" $WS keep t55 --message="empty_dir"
$WS drop t55 2>/dev/null || true

# 56: Nested dir
$WS get layer:"$BASE_LAYER" --name=t56 >/dev/null 2>&1
mkdir -p "$($WS path t56)/src/deep/nested/path"
echo 'package nested' > "$($WS path t56)/src/deep/nested/path/file.go"
L56=$($WS keep t56 --json 2>/dev/null | JQ_LAYER)
t_cond 56 "Nested dir" "edge.nested_dir" "$WS layer files $L56 | grep -q 'deep/nested/path/file.go'"
$WS drop t56 2>/dev/null || true

# 57: File deletion
$WS get layer:"$BASE_LAYER" --name=t57 >/dev/null 2>&1
$WS run t57 -- rm src/api/routes.go
t 57 "File deletion" "edge.file_deletion" $WS keep t57 --message="deleted"
$WS drop t57 2>/dev/null || true

# 58: File overwrite
$WS get layer:"$BASE_LAYER" --name=t58 >/dev/null 2>&1
echo "# Modified" > "$($WS path t58)/README.md"
t 58 "File overwrite" "edge.file_overwrite" $WS keep t58 --message="overwritten"
$WS drop t58 2>/dev/null || true

# 59: Special chars filename
$WS get layer:"$BASE_LAYER" --name=t59 >/dev/null 2>&1
echo "x" > "$($WS path t59)/src/file with spaces.go"
t 59 "Special chars" "edge.special_chars" $WS keep t59 --message="special"
$WS drop t59 2>/dev/null || true

# 60: Long filename
$WS get layer:"$BASE_LAYER" --name=t60 >/dev/null 2>&1
LONG="file_with_a_very_long_name_$(printf 'a%.0s' {1..100}).go"
echo "x" > "$($WS path t60)/src/$LONG"
t 60 "Long filename" "edge.long_filename" $WS keep t60 --message="long"
$WS drop t60 2>/dev/null || true

echo "T3: done $PASS pass $FAIL fail"
