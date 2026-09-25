#!/usr/bin/env bash
# Tier 2: Multi-Agent Coordination (scenarios 21-40)
set -uo pipefail  # no -e
source "$(dirname "$0")/common.sh"
DEMO="/tmp/ws-scenario-repo"
ensure_setup || { echo "T2: setup failed"; exit 1; }

echo "T2: starting"

# 21: Two agents same base
$WS get layer:"$BASE_LAYER" --name=a21a >/dev/null 2>&1
$WS get layer:"$BASE_LAYER" --name=a21b >/dev/null 2>&1
t_cond 21 "Two agents" "coord.parallel_same_base" "$WS status | grep -q a21a && $WS status | grep -q a21b"
$WS drop a21a a21b 2>/dev/null || true

# 22: Three agents from seed
$WS get layer:"$BASE_LAYER" --name=seed22 >/dev/null 2>&1
echo "x" > "$($WS path seed22)/src/x.go"
S22=$($WS keep seed22 --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S22" --name=a22a >/dev/null 2>&1
$WS get layer:"$S22" --name=a22b >/dev/null 2>&1
$WS get layer:"$S22" --name=a22c >/dev/null 2>&1
t_cond 22 "Three from seed" "coord.parallel_seed" "$WS status | grep -q a22a && $WS status | grep -q a22c"
$WS drop seed22 a22a a22b a22c 2>/dev/null || true

# 23: Branch from agent
$WS get layer:"$BASE_LAYER" --name=a23a >/dev/null 2>&1
echo "x" > "$($WS path a23a)/src/x.go"
t 23 "Branch from agent" "coord.sequential_branch" $WS get ws:a23a --name=a23b
$WS drop a23a a23b 2>/dev/null || true

# 24: Branch from kept layer
$WS get layer:"$BASE_LAYER" --name=a24a >/dev/null 2>&1
echo "x" > "$($WS path a24a)/src/x.go"
L24=$($WS keep a24a --json 2>/dev/null | JQ_LAYER)
$WS drop a24a 2>/dev/null || true
t 24 "Layer branch" "coord.layer_branch" $WS get layer:"$L24" --name=a24b
$WS drop a24b 2>/dev/null || true

# 25: Merge two agents
$WS get layer:"$BASE_LAYER" --name=c25s >/dev/null 2>&1
echo "x" > "$($WS path c25s)/src/x.go"
S25=$($WS keep c25s --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S25" --name=c25a >/dev/null 2>&1
echo 'package main; func A() {}' > "$($WS path c25a)/src/a.go"
LA25=$($WS keep c25a --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S25" --name=c25b >/dev/null 2>&1
echo 'package main; func B() {}' > "$($WS path c25b)/src/b.go"
LB25=$($WS keep c25b --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$LA25" --name=c25m >/dev/null 2>&1
$WS layer copy "$LB25" src/b.go "$($WS path c25m)/src/b.go" 2>/dev/null
t 25 "Merge two" "coord.merge_two" $WS keep c25m --message="merged"
$WS drop c25s c25a c25b c25m 2>/dev/null || true

# 26: Merge three agents
$WS get layer:"$BASE_LAYER" --name=c26s >/dev/null 2>&1
echo "x" > "$($WS path c26s)/src/x.go"
S26=$($WS keep c26s --json 2>/dev/null | JQ_LAYER)
for i in a b c; do
  $WS get layer:"$S26" --name="c26_$i" >/dev/null 2>&1
  echo "package main; func ${i^^}() {}" > "$($WS path c26_$i)/src/${i}.go"
  eval "L26_${i}=\$($WS keep c26_$i --json 2>/dev/null | JQ_LAYER)"
done
$WS get layer:"$L26_a" --name=c26m >/dev/null 2>&1
$WS layer copy "$L26_b" src/b.go "$($WS path c26m)/src/b.go" 2>/dev/null
$WS layer copy "$L26_c" src/c.go "$($WS path c26m)/src/c.go" 2>/dev/null
t 26 "Merge three" "coord.merge_three" $WS keep c26m --message="merged3"
$WS drop c26s c26_a c26_b c26_c c26m 2>/dev/null || true

# 27: JSON pipeline
$WS get layer:"$BASE_LAYER" --name=j27 >/dev/null 2>&1
echo "x" > "$($WS path j27)/src/x.go"
J27=$($WS keep j27 --json 2>/dev/null | JQ_LAYER)
t_cond 27 "JSON pipeline" "coord.json_pipeline" "[ -n \"$J27\" ]"
$WS drop j27 2>/dev/null || true

# 28: Status --json
$WS get layer:"$BASE_LAYER" --name=t28 >/dev/null 2>&1
t 28 "Status --json" "coord.status_json" $WS status --json
$WS drop t28 2>/dev/null || true

# 29: Layer ls --json
$WS get layer:"$BASE_LAYER" --name=t29 >/dev/null 2>&1
echo "x" > "$($WS path t29)/src/x.go"
$WS keep t29 --message="t29" >/dev/null 2>&1
t 29 "Layer ls --json" "coord.layer_ls_json" $WS layer ls --json
$WS drop t29 2>/dev/null || true

# 30: Cross-agent visibility
$WS get layer:"$BASE_LAYER" --name=v30a >/dev/null 2>&1
$WS get layer:"$BASE_LAYER" --name=v30b >/dev/null 2>&1
t_cond 30 "Visibility" "coord.visibility" "$WS status | grep -q v30a && $WS status | grep -q v30b"
$WS drop v30a v30b 2>/dev/null || true

# 31: Agent handoff
$WS get layer:"$BASE_LAYER" --name=h31 >/dev/null 2>&1
echo "x" > "$($WS path h31)/src/x.go"
L31=$($WS keep h31 --json 2>/dev/null | JQ_LAYER)
$WS drop h31 2>/dev/null || true
t 31 "Handoff" "coord.handoff" $WS get layer:"$L31" --name=h31b
$WS drop h31b 2>/dev/null || true

# 32: Force re-branch
$WS get layer:"$BASE_LAYER" --name=f32 >/dev/null 2>&1
echo "v1" > "$($WS path f32)/src/v.go"
t 32 "Force re-branch" "coord.force_rebranch" $WS get ws:f32 --name=f32b --force
$WS drop f32 f32b 2>/dev/null || true

# 33: Layer copy merge
$WS get layer:"$BASE_LAYER" --name=lc33s >/dev/null 2>&1
echo "x" > "$($WS path lc33s)/src/x.go"
S33=$($WS keep lc33s --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S33" --name=lc33a >/dev/null 2>&1
echo 'package main; func A() {}' > "$($WS path lc33a)/src/a.go"
LA33=$($WS keep lc33a --json 2>/dev/null | JQ_LAYER)
mkdir -p /tmp/ws-t33-t2
t 33 "Layer copy" "coord.layer_copy_merge" $WS layer copy "$LA33" src/a.go /tmp/ws-t33-t2/a.go
rm -rf /tmp/ws-t33-t2
$WS drop lc33s lc33a 2>/dev/null || true

# 34: Layer diff agents
$WS get layer:"$BASE_LAYER" --name=ld34s >/dev/null 2>&1
echo "x" > "$($WS path ld34s)/src/x.go"
S34=$($WS keep ld34s --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S34" --name=ld34a >/dev/null 2>&1
echo 'a' > "$($WS path ld34a)/src/a.go"
LA34=$($WS keep ld34a --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S34" --name=ld34b >/dev/null 2>&1
echo 'b' > "$($WS path ld34b)/src/b.go"
LB34=$($WS keep ld34b --json 2>/dev/null | JQ_LAYER)
t 34 "Layer diff agents" "coord.layer_diff_agents" $WS layer diff "$LA34" "$LB34"
$WS drop ld34s ld34a ld34b 2>/dev/null || true

# 35: ws diff two workspaces
$WS get layer:"$BASE_LAYER" --name=wd35a >/dev/null 2>&1
$WS get layer:"$BASE_LAYER" --name=wd35b >/dev/null 2>&1
echo 'a' > "$($WS path wd35a)/src/a.go"
echo 'b' > "$($WS path wd35b)/src/b.go"
t 35 "ws diff two" "coord.ws_diff" $WS diff wd35a wd35b
$WS drop wd35a wd35b 2>/dev/null || true

# 36: Graph topology
$WS get layer:"$BASE_LAYER" --name=g36a >/dev/null 2>&1
echo "x" > "$($WS path g36a)/src/x.go"
L36=$($WS keep g36a --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$L36" --name=g36b >/dev/null 2>&1
t 36 "Graph topology" "coord.graph" $WS graph
$WS drop g36a g36b 2>/dev/null || true

# 37: Graph specific ws
$WS get layer:"$BASE_LAYER" --name=g37a >/dev/null 2>&1
echo "x" > "$($WS path g37a)/src/x.go"
L37=$($WS keep g37a --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$L37" --name=g37b >/dev/null 2>&1
t 37 "Graph specific" "coord.graph_ws" $WS graph g37b
$WS drop g37a g37b 2>/dev/null || true

# 38: Keep --json
$WS get layer:"$BASE_LAYER" --name=k38 >/dev/null 2>&1
echo "x" > "$($WS path k38)/src/x.go"
K38=$($WS keep k38 --json 2>/dev/null | JQ_LAYER)
t_cond 38 "Keep --json" "coord.keep_json" "[ -n \"$K38\" ] && [ \"${#K38}\" -eq 16 ]"
$WS drop k38 2>/dev/null || true

# 39: Get --json
G39=$($WS get layer:"$BASE_LAYER" --name=g39 --json 2>/dev/null | JQ_LAYER)
t_cond 39 "Get --json" "coord.get_json" "[ -n \"$G39\" ] && [ \"${#G39}\" -eq 16 ]"
$WS drop g39 2>/dev/null || true

# 40: Export consolidated
$WS get layer:"$BASE_LAYER" --name=e40s >/dev/null 2>&1
echo "x" > "$($WS path e40s)/src/x.go"
S40=$($WS keep e40s --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S40" --name=e40a >/dev/null 2>&1
echo 'package main; func A() {}' > "$($WS path e40a)/src/a.go"
$WS keep e40a --message="e40" >/dev/null 2>&1
mkdir -p /tmp/ws-e40-t2
t 40 "Export consolidated" "coord.export_consolidated" $WS export e40a /tmp/ws-e40-t2
rm -rf /tmp/ws-e40-t2
$WS drop e40s e40a 2>/dev/null || true

echo "T2: done $PASS pass $FAIL fail"
