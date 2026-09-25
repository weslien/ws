#!/usr/bin/env python3
"""ws strategy selector — Phase 1 rule-based System1 model.

Given a task description, extracts features and selects a ws strategy.
Outputs the strategy tag and a command template for the LLM to fill in.

Usage:
    echo "Fix the bug by trying 3 approaches in parallel" | python3 strategy-selector.py
    python3 strategy-selector.py "Build a feature with 5 agents in 3 waves"
"""

import json
import re
import sys
import os


def extract_features(task: str) -> dict:
    """Extract features from a natural language task description."""
    task_lower = task.lower()
    features = {
        "agent_count": 1,
        "dependency_type": "none",
        "has_merge": False,
        "merge_granularity": "full",
        "has_checkpoint": False,
        "has_rollback": False,
        "has_export": False,
        "task_type": "feature",
        "isolation_level": "workspace",
        "retry_count": 0,
        "has_hypotheses": False,
        "wave_count": 1,
        "provenance_needed": False,
        "source_type": "base",
    }

    # Agent count
    for n in [10, 9, 8, 7, 6, 5, 4, 3, 2]:
        if re.search(rf"\b{n}\b.*agent|\bagent.*\b{n}\b|parallel.*{n}|{n}.*parallel", task_lower):
            features["agent_count"] = n
            break
    # "from agent-A and agent-B" implies 2 agents even without a number
    agent_refs = len(re.findall(r"agent-[a-z]", task_lower))
    if agent_refs > features["agent_count"]:
        features["agent_count"] = agent_refs
    if "parallel" in task_lower or "concurrent" in task_lower:
        if features["agent_count"] == 1:
            features["agent_count"] = 3  # default parallel

    # Dependency type
    if "diamond" in task_lower or ("independent" in task_lower and "merge" in task_lower):
        features["dependency_type"] = "diamond"
    elif "sequential" in task_lower or "after" in task_lower or "depends on" in task_lower or "wave" in task_lower:
        features["dependency_type"] = "sequential"
    elif features["agent_count"] > 1:
        features["dependency_type"] = "parallel"

    # Checkpoint
    if "checkpoint" in task_lower or "save" in task_lower or "progress" in task_lower:
        features["has_checkpoint"] = True

    # Rollback
    if "roll back" in task_lower or "rollback" in task_lower or "undo" in task_lower or "redo" in task_lower:
        features["has_rollback"] = True
        features["has_checkpoint"] = True  # rollback implies checkpoint
        if "redo" in task_lower or "retry" in task_lower:
            features["retry_count"] = 1

    # Merge — also detect "from agent-" as selective merge
    if "merge" in task_lower or "combine" in task_lower or "consolidate" in task_lower or "pick" in task_lower:
        features["has_merge"] = True
    if "selective" in task_lower or "take only" in task_lower or "specific file" in task_lower or "from agent-" in task_lower:
        features["merge_granularity"] = "selective"
        features["has_merge"] = True

    # Export
    if "export" in task_lower or "review" in task_lower or "commit" in task_lower or "deploy" in task_lower:
        features["has_export"] = True

    # Task type
    if "bug" in task_lower or "fix" in task_lower:
        features["task_type"] = "bugfix"
    elif "test" in task_lower:
        features["task_type"] = "test"
    elif "refactor" in task_lower:
        features["task_type"] = "refactor"
    elif "audit" in task_lower or "provenance" in task_lower or "trace" in task_lower:
        features["task_type"] = "audit"

    # Hypotheses
    if "hypothes" in task_lower or "approach" in task_lower or "try" in task_lower or "compare" in task_lower:
        features["has_hypotheses"] = True

    # Wave count
    if "wave" in task_lower:
        for n in [3, 2]:
            if f"{n} wave" in task_lower or f"wave {n}" in task_lower:
                features["wave_count"] = n
                break

    # Provenance
    if "audit" in task_lower or "provenance" in task_lower or "trace" in task_lower:
        features["provenance_needed"] = True

    # Source type
    if "directory" in task_lower or "folder" in task_lower or "local" in task_lower:
        features["source_type"] = "dir"
    elif "checkpoint" in task_lower or "saved" in task_lower or "from there" in task_lower:
        features["source_type"] = "layer"
    elif "another agent" in task_lower or "agent-" in task_lower:
        features["source_type"] = "ws"

    return features


def select_strategy(f: dict) -> str:
    """Phase 1 rule-based strategy selection."""
    # Single agent
    if f["agent_count"] == 1:
        if f["has_export"] and f["has_merge"]:
            return "strategy.full_lifecycle"
        if f["has_checkpoint"] and f["has_rollback"]:
            return "strategy.checkpoint_rollback"
        if f["provenance_needed"]:
            return "strategy.audit"
        if f["has_export"]:
            return "basic.export"
        if f["source_type"] == "dir":
            return "basic.dir"
        if f["source_type"] == "layer":
            return "basic.fork_layer"
        if f["source_type"] == "ws":
            return "basic.branch_ws"
        if f["retry_count"] > 0:
            return "strategy.agent_retry"
        return "basic.clone"

    # Multi-agent
    if f["has_hypotheses"]:
        return "strategy.parallel_hypotheses"

    if f["dependency_type"] == "diamond":
        return "strategy.diamond_merge"

    if f["dependency_type"] == "sequential" and f["wave_count"] > 1:
        return "strategy.multi_wave"

    if f["has_merge"] and f["merge_granularity"] == "selective":
        return "strategy.selective_merge_3"

    if f["has_merge"]:
        if f["agent_count"] <= 2:
            return "coord.merge_two"
        return "coord.merge_three"

    if f["dependency_type"] == "sequential":
        return "coord.sequential_branch"

    if f["has_export"]:
        return "strategy.full_lifecycle"

    return "strategy.seed_branch_consolidate"


# Strategy → command template mapping
STRATEGIES = {
    "basic.clone": [
        "ws get base:$REPO --name=$TASK",
        "ws run $TASK -- $BUILD_CMD",
        "ws diff $TASK",
        "ws keep $TASK --message='$MSG' --json",
    ],
    "basic.dir": [
        "ws get dir:$PATH --name=$TASK",
        "ws run $TASK -- $BUILD_CMD",
        "ws keep $TASK --message='$MSG' --json",
    ],
    "basic.fork_layer": [
        "ws get layer:$LAYER --name=$TASK",
        "ws run $TASK -- $BUILD_CMD",
        "ws keep $TASK --message='$MSG' --json",
    ],
    "basic.branch_ws": [
        "ws get ws:$SOURCE_WS --name=$TASK",
        "ws run $TASK -- $BUILD_CMD",
        "ws keep $TASK --message='$MSG' --json",
    ],
    "basic.export": [
        "ws export $TASK $DEST",
    ],
    "strategy.seed_branch_consolidate": [
        "ws get base:$REPO --name=seed",
        "ws keep seed --json",
        "# Branch N agents from seed",
        "for i in $(seq 1 $N); do ws get layer:$SEED --name=agent-$i; done",
        "# Agents work independently",
        "ws keep agent-1 --json  # → $LAYER_1",
        "# Consolidator: fork from best, copy files from others",
        "ws get layer:$BEST --name=final --force",
        "ws layer copy $OTHER_LAYER $FILE $(ws path final)/$FILE",
        "ws keep final --message='consolidated'",
        "ws export final $DEST",
        "ws drop seed agent-{1..$N} final",
    ],
    "strategy.checkpoint_rollback": [
        "ws get base:$REPO --name=task",
        "# ... work ...",
        "ws keep task --json  # → $CHECKPOINT",
        "# ... more work (breaks) ...",
        "# Roll back:",
        "ws get layer:$CHECKPOINT --name=task-recovered --force",
    ],
    "strategy.parallel_hypotheses": [
        "ws get base:$REPO --name=seed",
        "ws keep seed --json  # → $SEED",
        "# N agents test different hypotheses",
        "for i in $(seq 1 $N); do",
        "  ws get layer:$SEED --name=hypothesis-$i",
        "  # ... try fix ...",
        "  ws keep hypothesis-$i --json",
        "done",
        "# Consolidator picks best",
        "ws get layer:$BEST --name=fix --force",
        "ws keep fix --message='best hypothesis'",
    ],
    "strategy.diamond_merge": [
        "ws get base:$REPO --name=base",
        "ws keep base --json  # → $BASE",
        "ws get layer:$BASE --name=branch-a",
        "ws keep branch-a --json  # → $LA",
        "ws get layer:$BASE --name=branch-b",
        "ws keep branch-b --json  # → $LB",
        "# Merge: fork from A, pull B's files",
        "ws get layer:$LA --name=merged",
        "ws layer copy $LB $FILE $(ws path merged)/$FILE",
        "ws keep merged --message='merged A+B'",
    ],
    "strategy.multi_wave": [
        "ws get base:$REPO --name=seed",
        "ws keep seed --json  # → $SEED",
        "# Wave 1",
        "ws get layer:$SEED --name=wave-1",
        "ws keep wave-1 --json  # → $W1",
        "# Wave 2 (depends on wave 1)",
        "ws get layer:$W1 --name=wave-2",
        "ws keep wave-2 --json  # → $W2",
        "# Wave 3 (depends on wave 2)",
        "ws get layer:$W2 --name=wave-3",
        "ws keep wave-3 --message='final wave'",
    ],
    "strategy.selective_merge_3": [
        "ws get base:$REPO --name=base",
        "ws keep base --json  # → $BASE",
        "ws get layer:$BASE --name=branch-1 && ws keep branch-1 --json  # → $L1",
        "ws get layer:$BASE --name=branch-2 && ws keep branch-2 --json  # → $L2",
        "ws get layer:$BASE --name=branch-3 && ws keep branch-3 --json  # → $L3",
        "# Merge: fork from L1, copy specific files from L2 and L3",
        "ws get layer:$L1 --name=merged",
        "ws layer copy $L2 $FILE2 $(ws path merged)/$FILE2",
        "ws layer copy $L3 $FILE3 $(ws path merged)/$FILE3",
        "ws keep merged --message='selective merge'",
    ],
    "strategy.agent_retry": [
        "# First attempt (failed)",
        "ws get base:$REPO --name=agent --force",
        "ws keep agent --message='bad attempt'",
        "# Retry from base",
        "ws get base:$REPO --name=agent --force",
        "ws keep agent --message='retry success'",
    ],
    "strategy.consolidator_pick": [
        "ws get base:$REPO --name=seed",
        "ws keep seed --json  # → $SEED",
        "# N agents produce work",
        "for i in $(seq 1 $N); do",
        "  ws get layer:$SEED --name=agent-$i",
        "  ws keep agent-$i --json",
        "done",
        "# Consolidator evaluates and picks best",
        "ws get layer:$BEST --name=final --force",
        "ws keep final --message='picked best agent'",
    ],
    "strategy.full_lifecycle": [
        "ws get base:$REPO --name=seed",
        "ws keep seed --json  # → $SEED",
        "# Phase: execute with N agents",
        "for i in $(seq 1 $N); do",
        "  ws get layer:$SEED --name=agent-$i",
        "  ws keep agent-$i --json",
        "done",
        "# Phase: consolidate",
        "ws get layer:$BEST --name=final",
        "ws layer copy $OTHER $FILE $(ws path final)/$FILE",
        "ws keep final --message='consolidated'",
        "# Phase: export",
        "ws export final $DEST",
        "ws drop seed agent-{1..$N} final",
    ],
    "strategy.audit": [
        "ws layer ls --json",
        "ws graph $WS",
        "ws layer show $LAYER",
        "ws layer files $LAYER",
        "ws layer diff $LAYER_A $LAYER_B",
    ],
    "coord.merge_two": [
        "ws get layer:$LA --name=merged",
        "ws layer copy $LB $FILE $(ws path merged)/$FILE",
        "ws keep merged --message='merged two'",
    ],
    "coord.merge_three": [
        "ws get layer:$LA --name=merged",
        "ws layer copy $LB $FILE_B $(ws path merged)/$FILE_B",
        "ws layer copy $LC $FILE_C $(ws path merged)/$FILE_C",
        "ws keep merged --message='merged three'",
    ],
    "coord.sequential_branch": [
        "ws get layer:$BASE --name=agent-1",
        "ws keep agent-1 --json  # → $L1",
        "ws get layer:$L1 --name=agent-2",
        "ws keep agent-2 --message='built on agent-1'",
    ],
}


def main():
    if len(sys.argv) > 1:
        task = " ".join(sys.argv[1:])
    else:
        task = sys.stdin.read().strip()

    if not task:
        print("Usage: strategy-selector.py <task description>", file=sys.stderr)
        sys.exit(1)

    features = extract_features(task)
    strategy = select_strategy(features)
    commands = STRATEGIES.get(strategy, ["# Unknown strategy: " + strategy])

    result = {
        "task": task,
        "strategy": strategy,
        "features": features,
        "commands": commands,
    }
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    main()
