#!/usr/bin/env bash
# Tier 1: Basic Lifecycle (scenarios 1-20)
set -uo pipefail  # no -e
source "$(dirname "$0")/common.sh"
DEMO="/tmp/ws-scenario-repo"
ensure_setup || { echo "T1: setup failed"; exit 1; }

echo "T1: starting"

# 1: Create from base repo
t 1 "Create from base" "basic.clone" $WS get base:"$DEMO" --name=t1
$WS drop t1 2>/dev/null || true

# 2: Create from dir
t 2 "Create from dir" "basic.dir" $WS get dir:"$DEMO" --name=t2
$WS drop t2 2>/dev/null || true

# 3: Run command
$WS get layer:"$BASE_LAYER" --name=t3 >/dev/null 2>&1
t 3 "Run command" "basic.run" $WS run t3 -- ls
$WS drop t3 2>/dev/null || true

# 4: Diff workspace
$WS get layer:"$BASE_LAYER" --name=t4 >/dev/null 2>&1
echo "test" > "$($WS path t4)/src/new.go"
t 4 "Diff workspace" "basic.diff" $WS diff t4
$WS drop t4 2>/dev/null || true

# 5: Keep workspace
$WS get layer:"$BASE_LAYER" --name=t5 >/dev/null 2>&1
echo "test" > "$($WS path t5)/src/new.go"
t 5 "Keep" "basic.keep" $WS keep t5 --message="test5"
$WS drop t5 2>/dev/null || true

# 6: Keep with message
$WS get layer:"$BASE_LAYER" --name=t6 >/dev/null 2>&1
echo "test" > "$($WS path t6)/src/new.go"
t 6 "Keep message" "basic.keep_message" $WS keep t6 --message="msg6"
$WS drop t6 2>/dev/null || true

# 7: Drop
$WS get layer:"$BASE_LAYER" --name=t7 >/dev/null 2>&1
t 7 "Drop" "basic.drop" $WS drop t7

# 8: Drop multiple
$WS get layer:"$BASE_LAYER" --name=t8a >/dev/null 2>&1
$WS get layer:"$BASE_LAYER" --name=t8b >/dev/null 2>&1
t 8 "Drop multi" "basic.drop_multi" $WS drop t8a t8b

# 9: Keep unchanged (dedup)
$WS get layer:"$BASE_LAYER" --name=t9 >/dev/null 2>&1
t 9 "Keep unchanged" "basic.keep_unchanged" $WS keep t9 --message="unchanged"
$WS drop t9 2>/dev/null || true

# 10: Fork from kept layer
$WS get layer:"$BASE_LAYER" --name=t10a >/dev/null 2>&1
echo "x" > "$($WS path t10a)/src/x.go"
L10=$($WS keep t10a --json 2>/dev/null | JQ_LAYER)
t 10 "Fork layer" "basic.fork_layer" $WS get layer:"$L10" --name=t10b
$WS drop t10a t10b 2>/dev/null || true

# 11: Branch from workspace
$WS get layer:"$BASE_LAYER" --name=t11a >/dev/null 2>&1
echo "y" > "$($WS path t11a)/src/y.go"
t 11 "Branch ws" "basic.branch_ws" $WS get ws:t11a --name=t11b
$WS drop t11a t11b 2>/dev/null || true

# 12: Layer list
$WS get layer:"$BASE_LAYER" --name=t12 >/dev/null 2>&1
echo "z" > "$($WS path t12)/src/z.go"
$WS keep t12 --message="t12" >/dev/null 2>&1
t 12 "Layer ls" "basic.layer_ls" $WS layer ls
$WS drop t12 2>/dev/null || true

# 13: Layer show
$WS get layer:"$BASE_LAYER" --name=t13 >/dev/null 2>&1
echo "s" > "$($WS path t13)/src/s.go"
L13=$($WS keep t13 --json 2>/dev/null | JQ_LAYER)
t 13 "Layer show" "basic.layer_show" $WS layer show "$L13"
$WS drop t13 2>/dev/null || true

# 14: Layer cat
$WS get layer:"$BASE_LAYER" --name=t14 >/dev/null 2>&1
echo 'package main; func T14() {}' > "$($WS path t14)/src/t14.go"
L14=$($WS keep t14 --json 2>/dev/null | JQ_LAYER)
t 14 "Layer cat" "basic.layer_cat" $WS layer cat "$L14" src/t14.go
$WS drop t14 2>/dev/null || true

# 15: Layer files
$WS get layer:"$BASE_LAYER" --name=t15 >/dev/null 2>&1
echo 'x' > "$($WS path t15)/src/t15.go"
L15=$($WS keep t15 --json 2>/dev/null | JQ_LAYER)
t 15 "Layer files" "basic.layer_files" $WS layer files "$L15"
$WS drop t15 2>/dev/null || true

# 16: Layer path
$WS get layer:"$BASE_LAYER" --name=t16 >/dev/null 2>&1
echo 'x' > "$($WS path t16)/src/x.go"
L16=$($WS keep t16 --json 2>/dev/null | JQ_LAYER)
t 16 "Layer path" "basic.layer_path" $WS layer path "$L16"
$WS drop t16 2>/dev/null || true

# 17: ws path
$WS get layer:"$BASE_LAYER" --name=t17 >/dev/null 2>&1
t 17 "ws path" "basic.ws_path" $WS path t17
$WS drop t17 2>/dev/null || true

# 18: Export
$WS get layer:"$BASE_LAYER" --name=t18 >/dev/null 2>&1
mkdir -p /tmp/ws-e18-t1
t 18 "Export" "basic.export" $WS export t18 /tmp/ws-e18-t1
rm -rf /tmp/ws-e18-t1
$WS drop t18 2>/dev/null || true

# 19: Export excludes .git
$WS get base:"$DEMO" --name=t19 >/dev/null 2>&1
mkdir -p /tmp/ws-e19-t1
$WS export t19 /tmp/ws-e19-t1 >/dev/null 2>&1
t_cond 19 "Export no .git" "basic.export_nogit" "! -d /tmp/ws-e19-t1/.git"
rm -rf /tmp/ws-e19-t1
$WS drop t19 2>/dev/null || true

# 20: Status
$WS get layer:"$BASE_LAYER" --name=t20a >/dev/null 2>&1
$WS get layer:"$BASE_LAYER" --name=t20b >/dev/null 2>&1
t 20 "Status" "basic.status" $WS status
$WS drop t20a t20b 2>/dev/null || true

echo "T1: done $PASS pass $FAIL fail"
