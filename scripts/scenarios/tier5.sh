#!/usr/bin/env bash
# Tier 5: Advanced Multi-Agent Strategy (scenarios 81-100)
set -uo pipefail  # no -e
source "$(dirname "$0")/common.sh"
DEMO="/tmp/ws-scenario-repo"
ensure_setup

echo "T5: starting"

# 81: Seed-Branch-Consolidate
$WS get layer:"$BASE_LAYER" --name=sbc81-seed >/dev/null 2>&1
echo "x" > "$($WS path sbc81-seed)/src/x.go"
S81=$($WS keep sbc81-seed --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S81" --name=sbc81-a >/dev/null 2>&1
echo 'package main; func A() {}' > "$($WS path sbc81-a)/src/a.go"
LA81=$($WS keep sbc81-a --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S81" --name=sbc81-b >/dev/null 2>&1
echo 'package main; func B() {}' > "$($WS path sbc81-b)/src/b.go"
LB81=$($WS keep sbc81-b --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$LA81" --name=sbc81-final >/dev/null 2>&1
$WS layer copy "$LB81" src/b.go "$($WS path sbc81-final)/src/b.go" 2>/dev/null
t 81 "SBC" "strategy.seed_branch_consolidate" $WS keep sbc81-final --message="consolidated"
$WS drop sbc81-seed sbc81-a sbc81-b sbc81-final 2>/dev/null || true

# 82: Checkpoint recovery
$WS get layer:"$BASE_LAYER" --name=cr82 >/dev/null 2>&1
echo 'package main; func F1() {}' > "$($WS path cr82)/src/f1.go"
S1=$($WS keep cr82 --json 2>/dev/null | JQ_LAYER)
echo 'package main; func F2() {}' > "$($WS path cr82)/src/f2.go"
$WS keep cr82 --message="step2" >/dev/null 2>&1
$WS get layer:"$S1" --name=cr82-rec --force >/dev/null 2>&1
t_cond 82 "Checkpoint rollback" "strategy.checkpoint_rollback" "! $($WS run cr82-rec -- ls src/f2.go 2>&1) | grep -q f2"
$WS drop cr82 cr82-rec 2>/dev/null || true

# 83: Parallel hypotheses
$WS get layer:"$BASE_LAYER" --name=ph83-seed >/dev/null 2>&1
echo "x" > "$($WS path ph83-seed)/src/x.go"
S83=$($WS keep ph83-seed --json 2>/dev/null | JQ_LAYER)
for i in 1 2 3; do
  $WS get layer:"$S83" --name="ph83-h$i" >/dev/null 2>&1
  echo "fix $i" > "$($WS path ph83-h$i)/src/fix.go"
  eval "L83_$i=\$($WS keep ph83-h$i --json 2>/dev/null | JQ_LAYER)"
done
$WS get layer:"$L83_1" --name=ph83-fix --force >/dev/null 2>&1
t 83 "Parallel hypotheses" "strategy.parallel_hypotheses" $WS keep ph83-fix --message="h1 correct"
$WS drop ph83-seed ph83-h1 ph83-h2 ph83-h3 ph83-fix 2>/dev/null || true

# 84: Diamond merge
$WS get layer:"$BASE_LAYER" --name=dm84-base >/dev/null 2>&1
echo "x" > "$($WS path dm84-base)/src/x.go"
B84=$($WS keep dm84-base --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$B84" --name=dm84-a >/dev/null 2>&1
echo 'package main; func A() {}' > "$($WS path dm84-a)/src/a.go"
LA84=$($WS keep dm84-a --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$B84" --name=dm84-b >/dev/null 2>&1
echo 'package main; func B() {}' > "$($WS path dm84-b)/src/b.go"
LB84=$($WS keep dm84-b --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$LA84" --name=dm84-d >/dev/null 2>&1
$WS layer copy "$LB84" src/b.go "$($WS path dm84-d)/src/b.go" 2>/dev/null
t 84 "Diamond merge" "strategy.diamond_merge" $WS keep dm84-d --message="merged A+B"
$WS drop dm84-base dm84-a dm84-b dm84-d 2>/dev/null || true

# 85: Iterative refactoring
$WS get layer:"$BASE_LAYER" --name=ir85 >/dev/null 2>&1
echo 'v1' > "$($WS path ir85)/src/v1.go"
C1=$($WS keep ir85 --json 2>/dev/null | JQ_LAYER)
echo 'v2' > "$($WS path ir85)/src/v2.go"
C2=$($WS keep ir85 --json 2>/dev/null | JQ_LAYER)
echo 'v3' > "$($WS path ir85)/src/v3.go"
C3=$($WS keep ir85 --json 2>/dev/null | JQ_LAYER)
t_cond 85 "Iterative refactor" "strategy.iterative_refactor" "[ '$C1' != '$C2' ] && [ '$C2' != '$C3' ]"
$WS drop ir85 2>/dev/null || true

# 86: Multi-wave pipeline
$WS get layer:"$BASE_LAYER" --name=mw86-seed >/dev/null 2>&1
echo "x" > "$($WS path mw86-seed)/src/x.go"
S86=$($WS keep mw86-seed --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S86" --name=mw86-1a >/dev/null 2>&1
echo 'a' > "$($WS path mw86-1a)/src/a.go"
W1=$($WS keep mw86-1a --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$W1" --name=mw86-2a >/dev/null 2>&1
echo 'b' > "$($WS path mw86-2a)/src/b.go"
W2=$($WS keep mw86-2a --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$W2" --name=mw86-3a >/dev/null 2>&1
echo 'c' > "$($WS path mw86-3a)/src/c.go"
t 86 "Multi-wave" "strategy.multi_wave" $WS keep mw86-3a --message="wave 3"
$WS drop mw86-seed mw86-1a mw86-2a mw86-3a 2>/dev/null || true

# 87: Chain with branch
$WS get layer:"$BASE_LAYER" --name=cb87-0 >/dev/null 2>&1
echo "x" > "$($WS path cb87-0)/src/x.go"
L0=$($WS keep cb87-0 --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$L0" --name=cb87-1 >/dev/null 2>&1
echo 'a' > "$($WS path cb87-1)/src/a.go"
L1=$($WS keep cb87-1 --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$L1" --name=cb87-2a >/dev/null 2>&1
echo 'a2' > "$($WS path cb87-2a)/src/a2.go"
$WS get layer:"$L1" --name=cb87-2b >/dev/null 2>&1
echo 'b2' > "$($WS path cb87-2b)/src/b2.go"
t_cond 87 "Chain+branch" "strategy.chain_branch" "$WS graph | grep -q cb87-2a && $WS graph | grep -q cb87-2b"
$WS drop cb87-0 cb87-1 cb87-2a cb87-2b 2>/dev/null || true

# 88: Selective merge 3
$WS get layer:"$BASE_LAYER" --name=sm88-base >/dev/null 2>&1
echo "x" > "$($WS path sm88-base)/src/x.go"
B88=$($WS keep sm88-base --json 2>/dev/null | JQ_LAYER)
for i in 1 2 3; do
  $WS get layer:"$B88" --name="sm88-b$i" >/dev/null 2>&1
  echo "branch $i" > "$($WS path sm88-b$i)/src/branch${i}.go"
  eval "LB${i}=\$($WS keep sm88-b$i --json 2>/dev/null | JQ_LAYER)"
done
$WS get layer:"$LB1" --name=sm88-m >/dev/null 2>&1
$WS layer copy "$LB2" src/branch2.go "$($WS path sm88-m)/src/branch2.go" 2>/dev/null
$WS layer copy "$LB3" src/branch3.go "$($WS path sm88-m)/src/branch3.go" 2>/dev/null
t 88 "Selective merge 3" "strategy.selective_merge_3" $WS keep sm88-m --message="merged3"
$WS drop sm88-base sm88-b1 sm88-b2 sm88-b3 sm88-m 2>/dev/null || true

# 89: Agent retry
$WS get layer:"$BASE_LAYER" --name=ar89-1 >/dev/null 2>&1
echo 'bad' > "$($WS path ar89-1)/src/bad.go"
$WS keep ar89-1 --message="bad" >/dev/null 2>&1
$WS get layer:"$BASE_LAYER" --name=ar89-1 --force >/dev/null 2>&1
echo 'good' > "$($WS path ar89-1)/src/good.go"
t 89 "Agent retry" "strategy.agent_retry" $WS keep ar89-1 --message="retry"
$WS drop ar89-1 2>/dev/null || true

# 90: Consolidator picks
$WS get layer:"$BASE_LAYER" --name=cp90-seed >/dev/null 2>&1
echo "x" > "$($WS path cp90-seed)/src/x.go"
S90=$($WS keep cp90-seed --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S90" --name=cp90-a >/dev/null 2>&1
echo 'a' > "$($WS path cp90-a)/src/a.go"
LA90=$($WS keep cp90-a --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S90" --name=cp90-b >/dev/null 2>&1
echo 'b' > "$($WS path cp90-b)/src/b.go"
LB90=$($WS keep cp90-b --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$LA90" --name=cp90-final --force >/dev/null 2>&1
t 90 "Consolidator picks" "strategy.consolidator_pick" $WS keep cp90-final --message="picked A"
$WS drop cp90-seed cp90-a cp90-b cp90-final 2>/dev/null || true

# 91: Progressive
$WS get layer:"$BASE_LAYER" --name=pe91 >/dev/null 2>&1
echo 'f1' > "$($WS path pe91)/src/f1.go"
C1=$($WS keep pe91 --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$C1" --name=pe91-v2 --force >/dev/null 2>&1
echo 'f2' > "$($WS path pe91-v2)/src/f2.go"
C2=$($WS keep pe91-v2 --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$C2" --name=pe91-v3 --force >/dev/null 2>&1
echo 'f3' > "$($WS path pe91-v3)/src/f3.go"
t 91 "Progressive" "strategy.progressive" $WS keep pe91-v3 --message="v3"
$WS drop pe91 pe91-v2 pe91-v3 2>/dev/null || true

# 92: A/B test
$WS get layer:"$BASE_LAYER" --name=ab92-seed >/dev/null 2>&1
echo "x" > "$($WS path ab92-seed)/src/x.go"
S92=$($WS keep ab92-seed --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S92" --name=ab92-a >/dev/null 2>&1
echo 'approach A' > "$($WS path ab92-a)/src/approach.go"
LA92=$($WS keep ab92-a --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S92" --name=ab92-b >/dev/null 2>&1
echo 'approach B' > "$($WS path ab92-b)/src/approach.go"
LB92=$($WS keep ab92-b --json 2>/dev/null | JQ_LAYER)
t_cond 92 "A/B test" "strategy.ab_test" "[ '$LA92' != '$LB92' ]"
$WS drop ab92-seed ab92-a ab92-b 2>/dev/null || true

# 93: Canary
$WS get layer:"$BASE_LAYER" --name=can93-canary >/dev/null 2>&1
echo 'canary' > "$($WS path can93-canary)/src/canary.go"
LCAN=$($WS keep can93-canary --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$LCAN" --name=can93-prod >/dev/null 2>&1
t_cond 93 "Canary" "strategy.canary" "$WS run can93-prod -- ls src/canary.go"
$WS drop can93-canary can93-prod 2>/dev/null || true

# 94: Fork-merge-rebase
$WS get layer:"$BASE_LAYER" --name=fmr94-base >/dev/null 2>&1
echo "v1" > "$($WS path fmr94-base)/src/v.go"
B94=$($WS keep fmr94-base --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$B94" --name=fmr94-feature >/dev/null 2>&1
echo 'feature' > "$($WS path fmr94-feature)/src/feature.go"
LF94=$($WS keep fmr94-feature --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$B94" --name=fmr94-rebased >/dev/null 2>&1
$WS layer copy "$LF94" src/feature.go "$($WS path fmr94-rebased)/src/feature.go" 2>/dev/null
t 94 "Fork-merge-rebase" "strategy.fork_merge_rebase" $WS keep fmr94-rebased --message="rebased"
$WS drop fmr94-base fmr94-feature fmr94-rebased 2>/dev/null || true

# 95: Sparse
$WS get layer:"$BASE_LAYER" --name=sp95 >/dev/null 2>&1
echo 'sparse' > "$($WS path sp95)/src/sparse.go"
L95=$($WS keep sp95 --json 2>/dev/null | JQ_LAYER)
t_cond 95 "Sparse" "strategy.sparse" "$WS layer files $L95 | grep -q sparse"
$WS drop sp95 2>/dev/null || true

# 96: Audit
$WS get layer:"$BASE_LAYER" --name=au96 >/dev/null 2>&1
echo 'audit' > "$($WS path au96)/src/audit.go"
L96=$($WS keep au96 --json 2>/dev/null | JQ_LAYER)
t 96 "Audit show" "strategy.audit" $WS layer show "$L96"
t_cond 96b "Audit files" "strategy.audit" "$WS layer files $L96 | grep -q audit.go"
$WS drop au96 2>/dev/null || true

# 97: Cleanup
$WS get layer:"$BASE_LAYER" --name=cl97-seed >/dev/null 2>&1
echo "x" > "$($WS path cl97-seed)/src/x.go"
S97=$($WS keep cl97-seed --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S97" --name=cl97-a >/dev/null 2>&1
$WS get layer:"$S97" --name=cl97-b >/dev/null 2>&1
$WS keep cl97-a --message="a" >/dev/null 2>&1
$WS keep cl97-b --message="b" >/dev/null 2>&1
$WS drop cl97-seed cl97-a cl97-b 2>/dev/null || true
t 97 "Cleanup" "strategy.cleanup" $WS layer gc

# 98: Export for review
$WS get layer:"$BASE_LAYER" --name=er98 >/dev/null 2>&1
echo 'review' > "$($WS path er98)/src/review.go"
mkdir -p /tmp/ws-e98-t5
$WS export er98 /tmp/ws-e98-t5 >/dev/null 2>&1
sleep 1
t_cond 98 "Export for review" "strategy.export_review" "-f /tmp/ws-e98-t5/src/review.go"
rm -rf /tmp/ws-e98-t5
$WS drop er98 2>/dev/null || true

# 99: Full lifecycle
$WS get layer:"$BASE_LAYER" --name=fl99-seed >/dev/null 2>&1
echo "x" > "$($WS path fl99-seed)/src/x.go"
S99=$($WS keep fl99-seed --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S99" --name=fl99-a >/dev/null 2>&1
echo 'package main; func A() {}' > "$($WS path fl99-a)/src/a.go"
LA99=$($WS keep fl99-a --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$S99" --name=fl99-b >/dev/null 2>&1
echo 'package main; func B() {}' > "$($WS path fl99-b)/src/b.go"
LB99=$($WS keep fl99-b --json 2>/dev/null | JQ_LAYER)
$WS get layer:"$LA99" --name=fl99-final >/dev/null 2>&1
$WS layer copy "$LB99" src/b.go "$($WS path fl99-final)/src/b.go" 2>/dev/null
$WS keep fl99-final --message="final" >/dev/null 2>&1
mkdir -p /tmp/ws-e99-t5
$WS export fl99-final /tmp/ws-e99-t5 >/dev/null 2>&1
sleep 1
t_cond 99 "Full lifecycle" "strategy.full_lifecycle" "-f /tmp/ws-e99-t5/src/a.go && -f /tmp/ws-e99-t5/src/b.go"
rm -rf /tmp/ws-e99-t5
$WS drop fl99-seed fl99-a fl99-b fl99-final 2>/dev/null || true

# 100: 10-agent JSON
$WS get layer:"$BASE_LAYER" --name=j100-seed >/dev/null 2>&1
echo "x" > "$($WS path j100-seed)/src/x.go"
S100=$($WS keep j100-seed --json 2>/dev/null | JQ_LAYER)
LAYERS=""
for i in $(seq 1 10); do
  $WS get layer:"$S100" --name="j100-$i" >/dev/null 2>&1
  echo "package main; func F${i}() {}" > "$($WS path j100-$i)/src/f${i}.go"
  LI=$($WS keep "j100-$i" --json 2>/dev/null | JQ_LAYER)
  LAYERS="$LAYERS $LI"
done
$WS get layer:"$S100" --name=j100-con >/dev/null 2>&1
for L in $LAYERS; do
  for f in $($WS layer files "$L" 2>/dev/null | grep '^src/f[0-9]*.go$'); do
    $WS layer copy "$L" "$f" "$($WS path j100-con)/$f" 2>/dev/null
  done
done
LAST=$($WS keep j100-con --message="consolidated 10" --json 2>/dev/null | JQ_LAYER)
$WS drop j100-seed j100-{1..10} j100-con 2>/dev/null || true
FCOUNT2=$($WS layer files "$LAST" 2>/dev/null | grep -c '^src/f[0-9]*.go$')
t_cond 100 "10-agent JSON" "strategy.full_json_10" "[ '$FCOUNT2' -ge 5 ]"

echo "T5: done $PASS pass $FAIL fail"
