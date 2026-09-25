#!/usr/bin/env bash
# Tier 4: Stress & Scale (scenarios 61-80)
set -uo pipefail  # no -e
source "$(dirname "$0")/common.sh"
DEMO="/tmp/ws-scenario-repo"
ensure_setup || { echo "T4: setup failed"; exit 1; }

echo "T4: starting"

# 61: 20 workspaces from same seed
$WS get layer:"$BASE_LAYER" --name=seed61 >/dev/null 2>&1
echo "x" > "$($WS path seed61)/src/x.go"
S61=$($WS keep seed61 --json 2>/dev/null | JQ_LAYER)
for i in $(seq 1 20); do
  $WS get layer:"$S61" --name="s61-$i" >/dev/null 2>&1
done
t_cond 61 "20 workspaces" "stress.20_ws" "[ \$($WS status | grep -c 's61-') -eq 20 ]"
$WS drop seed61 s61-{1..20} 2>/dev/null || true

# 62: 50 create-keep-drop cycles
$WS get layer:"$BASE_LAYER" --name=churn62-seed >/dev/null 2>&1
echo "x" > "$($WS path churn62-seed)/src/x.go"
S62=$($WS keep churn62-seed --json 2>/dev/null | JQ_LAYER)
for i in $(seq 1 50); do
  $WS get layer:"$S62" --name="churn62-$i" >/dev/null 2>&1
  echo "c$i" > "$($WS path churn62-$i)/src/c.go"
  $WS keep "churn62-$i" --message="churn $i" >/dev/null 2>&1
  $WS drop "churn62-$i" 2>/dev/null || true
done
$WS drop churn62-seed 2>/dev/null || true
t 62 "50 churn" "stress.churn_50" $WS layer gc

# 63: 100 files
$WS get layer:"$BASE_LAYER" --name=t63 >/dev/null 2>&1
mkdir -p "$($WS path t63)/src/generated"
for i in $(seq 1 100); do
  echo "package generated; func F${i}() {}" > "$($WS path t63)/src/generated/file_${i}.go"
done
L63=$($WS keep t63 --json 2>/dev/null | JQ_LAYER)
FCOUNT=$($WS layer files "$L63" 2>/dev/null | grep -c generated)
t_cond 63 "100 files" "stress.100_files" "[ '$FCOUNT' -eq 100 ]"
$WS drop t63 2>/dev/null || true

# 64: 1MB file
$WS get layer:"$BASE_LAYER" --name=t64 >/dev/null 2>&1
head -c 1048576 /dev/zero | tr '\0' 'x' > "$($WS path t64)/src/large.txt"
t 64 "1MB file" "stress.1mb_file" $WS keep t64 --message="1mb"
$WS drop t64 2>/dev/null || true

# 65: 10-level chain
$WS get layer:"$BASE_LAYER" --name=chain65-0 >/dev/null 2>&1
echo "x" > "$($WS path chain65-0)/src/c0.go"
PREV=$($WS keep chain65-0 --json 2>/dev/null | JQ_LAYER)
for i in $(seq 1 9); do
  $WS get layer:"$PREV" --name="chain65-$i" >/dev/null 2>&1
  echo "c$i" > "$($WS path chain65-$i)/src/c${i}.go"
  PREV=$($WS keep "chain65-$i" --json 2>/dev/null | JQ_LAYER)
done
t 65 "10-level chain" "stress.chain_10" $WS graph chain65-9
$WS drop chain65-{0..9} 2>/dev/null || true

# 66: 5-level chain
$WS get layer:"$BASE_LAYER" --name=chain66-0 >/dev/null 2>&1
echo "x" > "$($WS path chain66-0)/src/c.go"
PREV=$($WS keep chain66-0 --json 2>/dev/null | JQ_LAYER)
for i in $(seq 1 4); do
  $WS get layer:"$PREV" --name="chain66-$i" >/dev/null 2>&1
  echo "c$i" > "$($WS path chain66-$i)/src/c${i}.go"
  PREV=$($WS keep "chain66-$i" --json 2>/dev/null | JQ_LAYER)
done
t 66 "5-level chain" "stress.chain_5" $WS graph chain66-4
$WS drop chain66-{0..4} 2>/dev/null || true

# 67: Diamond
$WS get layer:"$BASE_LAYER" --name=d67-base >/dev/null 2>&1
echo "x" > "$($WS path d67-base)/src/x.go"
B67=$($WS keep d67-base --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$B67" --name=d67-b >/dev/null 2>&1
echo 'b' > "$($WS path d67-b)/src/b.go"
LB67=$($WS keep d67-b --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$B67" --name=d67-c >/dev/null 2>&1
echo 'c' > "$($WS path d67-c)/src/c.go"
LC67=$($WS keep d67-c --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$LB67" --name=d67-d >/dev/null 2>&1
$WS layer copy "$LC67" src/c.go "$($WS path d67-d)/src/c.go" 2>/dev/null
t 67 "Diamond" "stress.diamond" $WS keep d67-d --message="diamond"
$WS drop d67-base d67-b d67-c d67-d 2>/dev/null || true

# 68: Fan-out 10
$WS get layer:"$BASE_LAYER" --name=fo68-seed >/dev/null 2>&1
echo "x" > "$($WS path fo68-seed)/src/x.go"
S68=$($WS keep fo68-seed --json 2>/dev/null | JQ_LAYER)
for i in $(seq 1 10); do
  $WS get layer:"$S68" --name="fo68-$i" >/dev/null 2>&1
done
$WS drop fo68-seed 2>/dev/null || true
t_cond 68 "Fan-out 10" "stress.fanout_10" "[ $($WS status | grep -c 'fo68-') -eq 10 ]"
$WS drop fo68-seed fo68-{1..10} 2>/dev/null || true

# 69: GC after 20 workspaces
$WS get layer:"$BASE_LAYER" --name=gc69-seed >/dev/null 2>&1
echo "x" > "$($WS path gc69-seed)/src/x.go"
S69=$($WS keep gc69-seed --json 2>/dev/null | JQ_LAYER)
for i in $(seq 1 20); do
  $WS get layer:"$S69" --name="gc69-$i" >/dev/null 2>&1
  echo "x$i" > "$($WS path gc69-$i)/src/x.go"
  $WS keep "gc69-$i" --message="gc $i" >/dev/null 2>&1
  $WS drop "gc69-$i" 2>/dev/null || true
done
$WS drop gc69-seed 2>/dev/null || true
t 69 "GC after 20" "stress.gc_20" $WS layer gc

# 70: Rapid create-drop
for i in $(seq 1 20); do
  $WS get layer:"$BASE_LAYER" --name="rd70-$i" >/dev/null 2>&1
  $WS drop "rd70-$i" 2>/dev/null || true
done
t_cond 70 "Rapid create-drop" "stress.rapid_create_drop" "[ \$($WS layer ls | grep -c 'rd70-') -eq 0 ]"

# 71: Multiple keeps on same workspace
$WS get layer:"$BASE_LAYER" --name=mk71 >/dev/null 2>&1
echo "v1" > "$($WS path mk71)/src/v.go"
K1=$($WS keep mk71 --json 2>/dev/null | JQ_LAYER)
echo "v2" > "$($WS path mk71)/src/v2.go"
K2=$($WS keep mk71 --json 2>/dev/null | JQ_LAYER)
t_cond 71 "Multi keep" "stress.multi_keep" "[ '$K1' != '$K2' ]"
$WS drop mk71 2>/dev/null || true

# 72: Fork from deep layer
$WS get layer:"$BASE_LAYER" --name=deep72-0 >/dev/null 2>&1
echo "x" > "$($WS path deep72-0)/src/x.go"
PREV=$($WS keep deep72-0 --json 2>/dev/null | JQ_LAYER)
for i in $(seq 1 5); do
  $WS get layer:"$PREV" --name="deep72-$i" >/dev/null 2>&1
  echo "c$i" > "$($WS path deep72-$i)/src/c.go"
  PREV=$($WS keep "deep72-$i" --json 2>/dev/null | JQ_LAYER)
done
$WS drop deep72-{0..5} 2>/dev/null || true
t 72 "Fork deep" "stress.deep_fork" $WS get layer:"$PREV" --name=deep72-fork
$WS drop deep72-fork 2>/dev/null || true

# 73: 10 agents same file
$WS get layer:"$BASE_LAYER" --name=same73-seed >/dev/null 2>&1
echo "x" > "$($WS path same73-seed)/src/x.go"
S73=$($WS keep same73-seed --json 2>/dev/null | JQ_LAYER)
for i in $(seq 1 10); do
  $WS get layer:"$S73" --name="same73-$i" >/dev/null 2>&1
  echo "version $i" > "$($WS path same73-$i)/src/x.go"
  $WS keep "same73-$i" --message="v$i" >/dev/null 2>&1
done
LCOUNT=$($WS layer ls | wc -l)
t_cond 73 "10 agents same file" "stress.same_file_10" "[ '$LCOUNT' -ge 12 ]"
$WS drop same73-seed same73-{1..10} 2>/dev/null || true

# 74: 5 agents diff files
$WS get layer:"$BASE_LAYER" --name=diff74-seed >/dev/null 2>&1
echo "x" > "$($WS path diff74-seed)/src/x.go"
S74=$($WS keep diff74-seed --json 2>/dev/null | JQ_LAYER)
for i in $(seq 1 5); do
  $WS get layer:"$S74" --name="diff74-$i" >/dev/null 2>&1
  echo "package main; func F${i}() {}" > "$($WS path diff74-$i)/src/f${i}.go"
  $WS keep "diff74-$i" --message="f$i" >/dev/null 2>&1
done
LCOUNT=$($WS layer ls | wc -l)
t_cond 74 "5 agents diff files" "stress.diff_files_5" "[ '$LCOUNT' -ge 7 ]"
$WS drop diff74-seed diff74-{1..5} 2>/dev/null || true

# 75: Deep layer diff
$WS get layer:"$BASE_LAYER" --name=dl75-0 >/dev/null 2>&1
echo "a" > "$($WS path dl75-0)/src/a.go"
L0=$($WS keep dl75-0 --json 2>/dev/null | JQ_LAYER)
PREV=$L0
for i in $(seq 1 5); do
  $WS get layer:"$PREV" --name="dl75-$i" >/dev/null 2>&1
  echo "c$i" > "$($WS path dl75-$i)/src/c${i}.go"
  PREV=$($WS keep "dl75-$i" --json 2>/dev/null | JQ_LAYER)
done
t 75 "Deep diff" "stress.deep_diff" $WS layer diff "$L0" "$PREV"
$WS drop dl75-{0..5} 2>/dev/null || true

# 76: Export large
$WS get layer:"$BASE_LAYER" --name=el76 >/dev/null 2>&1
mkdir -p "$($WS path el76)/src/generated"
for i in $(seq 1 50); do
  echo "package generated; func F${i}() {}" > "$($WS path el76)/src/generated/file_${i}.go"
done
mkdir -p /tmp/ws-e76-t4
t 76 "Export large" "stress.export_large" $WS export el76 /tmp/ws-e76-t4
rm -rf /tmp/ws-e76-t4
$WS drop el76 2>/dev/null || true

# 77: 20 layers with GC
$WS get layer:"$BASE_LAYER" --name=l77-seed >/dev/null 2>&1
echo "x" > "$($WS path l77-seed)/src/x.go"
S77=$($WS keep l77-seed --json 2>/dev/null | JQ_LAYER)
for i in $(seq 1 20); do
  $WS get layer:"$S77" --name="l77-$i" >/dev/null 2>&1
  echo "v$i" > "$($WS path l77-$i)/src/v.go"
  $WS keep "l77-$i" --message="l77 $i" >/dev/null 2>&1
  $WS drop "l77-$i" 2>/dev/null || true
done
$WS drop l77-seed 2>/dev/null || true
t 77 "20 layers GC" "stress.20_layers_gc" $WS layer gc

# 78: Alternating keep/skip
$WS get layer:"$BASE_LAYER" --name=alt78-0 >/dev/null 2>&1
echo "a" > "$($WS path alt78-0)/src/a.go"
K0=$($WS keep alt78-0 --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$K0" --name=alt78-1 >/dev/null 2>&1
K1=$($WS keep alt78-1 --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$K1" --name=alt78-2 >/dev/null 2>&1
echo "b" > "$($WS path alt78-2)/src/b.go"
K2=$($WS keep alt78-2 --json 2>/dev/null | JQ_LAYER)
t_cond 78 "Alternating keep/skip" "stress.alternating_keep" "[ '$K0' = '$K1' ] && [ '$K1' != '$K2' ]"
$WS drop alt78-{0..2} 2>/dev/null || true

# 79: Base dedup 10
$WS get base:"$DEMO" --name=dd79-1 >/dev/null 2>&1
$WS drop dd79-1 2>/dev/null || true
for i in $(seq 1 10); do
  $WS get base:"$DEMO" --name="dd79-$i" >/dev/null 2>&1
  $WS drop "dd79-$i" 2>/dev/null || true
done
BASE_COUNT=$($WS layer ls --json 2>/dev/null | python3 -c "import json,sys;d=json.load(sys.stdin);print(len(set(l['hash'] for l in d if 'base:' in l.get('message',''))))" 2>/dev/null || echo 1)
t_cond 79 "Base dedup 10" "stress.dedup_10" "[ '$BASE_COUNT' -eq 1 ]"

# 80: Content dedup
$WS get layer:"$BASE_LAYER" --name=cd80a >/dev/null 2>&1
$WS get layer:"$BASE_LAYER" --name=cd80b >/dev/null 2>&1
echo 'package main; func Same() {}' > "$($WS path cd80a)/src/same.go"
echo 'package main; func Same() {}' > "$($WS path cd80b)/src/same.go"
KA=$($WS keep cd80a --json 2>/dev/null | JQ_LAYER)
KB=$($WS keep cd80b --json 2>/dev/null | JQ_LAYER)
t_cond 80 "Content dedup" "stress.content_dedup" "[ '$KA' = '$KB' ]"
$WS drop cd80a cd80b 2>/dev/null || true

echo "T4: done $PASS pass $FAIL fail"
