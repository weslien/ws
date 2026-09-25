# ws Strategy Selection — System1 Model Design

## Problem

When an AI agent is given a task, it must decide *how* to use `ws` to accomplish it. Today this decision is made by the LLM reasoning through the SKILL.md patterns — slow, inconsistent, and context-dependent. A **System1 model** could make instant strategy selection based on task characteristics, leaving the LLM to focus on the actual work.

## The Taxonomy

Every scenario in the 100-scenario catalog is tagged with a strategy category. These tags form the training labels:

| Category | Count | When to select | Example task |
|----------|-------|----------------|--------------|
| `basic.clone` | 1 | Single agent, fresh start | "Fix a bug in the repo" |
| `basic.dir` | 1 | Work on a local directory | "Refactor this folder" |
| `basic.run` | 1 | Execute in isolated workspace | "Run tests in isolation" |
| `basic.keep` | 2 | Checkpoint work | "Save my progress" |
| `basic.fork_layer` | 1 | Continue from a checkpoint | "Start from where I left off" |
| `basic.branch_ws` | 1 | Build on another agent's work | "Add tests to agent-1's code" |
| `basic.export` | 2 | Hand off to external system | "Give me the files to commit" |
| `coord.parallel_same_base` | 2 | Multiple agents, independent tasks | "Split into 3 parallel agents" |
| `coord.parallel_seed` | 1 | Multiple agents from shared base | "Seed-branch-consolidate" |
| `coord.sequential_branch` | 1 | Agent depends on previous agent | "Add logging after routes are done" |
| `coord.merge_two` / `merge_three` | 2 | Merge parallel work | "Combine agent-1 and agent-2 output" |
| `coord.layer_copy_merge` | 1 | Selective file merge | "Take routes.go from agent-A, store.go from agent-B" |
| `coord.handoff` | 1 | Pass work between agents | "Agent-1 finishes, agent-2 continues" |
| `coord.force_rebranch` | 1 | Retry/restart an agent's work | "Agent-3's approach was wrong, redo" |
| `coord.visibility` | 2 | Check what everyone's doing | "What are all agents working on?" |
| `edge.*` | 20 | Error handling | "What if the layer doesn't exist?" |
| `stress.*` | 20 | Scale validation | "Can it handle 100 files?" |
| `strategy.seed_branch_consolidate` | 1 | Core multi-agent workflow | "Build feature X with 5 agents" |
| `strategy.checkpoint_rollback` | 1 | Safe experimentation | "Try this, roll back if it breaks" |
| `strategy.parallel_hypotheses` | 1 | Bug hunt with unknown root cause | "Find the bug — try 3 approaches" |
| `strategy.diamond_merge` | 1 | Two independent changes converge | "Frontend + backend, merge both" |
| `strategy.iterative_refactor` | 1 | Step-by-step refactoring | "Refactor in 3 checkpoints" |
| `strategy.multi_wave` | 1 | Phased delivery with dependencies | "Wave 1: models, wave 2: API, wave 3: tests" |
| `strategy.chain_branch` | 1 | Linear work with side experiments | "Main line + experimental branch" |
| `strategy.selective_merge_3` | 1 | Merge specific files from 3 sources | "Take A's routes, B's store, C's tests" |
| `strategy.agent_retry` | 1 | Agent failed, start over | "Agent-2 produced garbage, redo from base" |
| `strategy.consolidator_pick` | 1 | Evaluate and pick best approach | "Which agent's solution is best?" |
| `strategy.progressive` | 1 | Additive enhancement | "Add feature 1, then 2, then 3" |
| `strategy.ab_test` | 1 | Compare two approaches | "Try approach A and B, compare" |
| `strategy.canary` | 1 | Test before deploying | "Verify in isolation before merge" |
| `strategy.fork_merge_rebase` | 1 | Branch, develop, rebase back | "Feature branch → rebase onto main" |
| `strategy.sparse` | 1 | Only touch needed files | "Just fix this one file" |
| `strategy.audit` | 1 | Review provenance | "Show me what changed in each layer" |
| `strategy.cleanup` | 1 | Post-consolidation cleanup | "Drop all agent workspaces, GC layers" |
| `strategy.export_review` | 1 | External review handoff | "Export for human review" |
| `strategy.full_lifecycle` | 2 | End-to-end workflow | "Plan → execute → consolidate → export" |

## Feature Extraction

Given a task description, extract these features for strategy selection:

| Feature | Type | Values | Example |
|---------|------|--------|---------|
| `agent_count` | int | 1, 2, 3, 5, 10, N | "5 agents" → 5 |
| `dependency_type` | enum | none, sequential, parallel, diamond, chain | "agent-2 depends on agent-1" → sequential |
| `has_merge` | bool | true, false | "combine the results" → true |
| `merge_granularity` | enum | full, selective | "take only routes.go" → selective |
| `has_checkpoint` | bool | true, false | "save progress" → true |
| `has_rollback` | bool | true, false | "roll back if it breaks" → true |
| `has_export` | bool | true, false | "export for review" → true |
| `task_type` | enum | build, test, refactor, bugfix, feature, audit | "fix the bug" → bugfix |
| `isolation_level` | enum | none, workspace, layer, full | "isolate from other work" → workspace |
| `retry_count` | int | 0, 1, 2+ | "redo from scratch" → 1 |
| `has_hypotheses` | bool | true, false | "try 3 approaches" → true |
| `wave_count` | int | 1, 2, 3+ | "3 waves of agents" → 3 |
| `provenance_needed` | bool | true, false | "audit trail" → true |
| `source_type` | enum | base, dir, layer, ws | "clone the repo" → base |

## Model Architecture

### Phase 1: Rule-based selector (immediate)

A simple decision tree mapping task features to strategy tags. Runs in <1ms. No ML needed — the taxonomy is small enough to enumerate.

```python
def select_strategy(features):
    if features.agent_count == 1:
        if features.has_export and features.has_merge:
            return "strategy.full_lifecycle"
        if features.has_checkpoint and features.has_rollback:
            return "strategy.checkpoint_rollback"
        if features.source_type == "dir":
            return "basic.dir"
        return "basic.clone"
    
    if features.has_hypotheses:
        return "strategy.parallel_hypotheses"
    
    if features.dependency_type == "diamond":
        return "strategy.diamond_merge"
    
    if features.dependency_type == "sequential" and features.wave_count > 1:
        return "strategy.multi_wave"
    
    if features.has_merge and features.merge_granularity == "selective":
        return "strategy.selective_merge_3"
    
    if features.has_merge:
        return "coord.merge_two" if features.agent_count == 2 else "coord.merge_three"
    
    if features.dependency_type == "sequential":
        return "coord.sequential_branch"
    
    return "strategy.seed_branch_consolidate"
```

### Phase 2: Fine-tuned model (future)

Train a small model (distilled from the 100-scenario catalog) that takes a natural language task description and outputs:
1. Strategy tag (e.g., `strategy.parallel_hypotheses`)
2. Recommended command sequence (e.g., `ws get base: → ws keep → ws get layer: ×N → ws layer copy → ws keep → ws export`)
3. Expected layer graph shape

Training data: the 100 scenarios + their strategy tags + their command sequences.

```
Input:  "Fix the authentication bug. Try 3 different approaches in parallel,
         pick the best one, and export the fix for review."

Output: {
  "strategy": "strategy.parallel_hypotheses",
  "commands": [
    "ws get base:REPO --name=seed",
    "ws keep seed --json",
    "# Fork 3 hypotheses",
    "ws get layer:$SEED --name=h1",
    "ws get layer:$SEED --name=h2", 
    "ws get layer:$SEED --name=h3",
    "# Each agent tests a different fix",
    "ws keep h1 --json",
    "ws keep h2 --json",
    "ws keep h3 --json",
    "# Consolidator picks best",
    "ws get layer:$BEST --name=fix --force",
    "ws keep fix --message='best fix'",
    "ws export fix /output"
  ],
  "graph_shape": "fan-out-then-converge"
}
```

### Phase 3: System1 + System2 integration

```
User task
    │
    ▼
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  System 1   │────▶│  Strategy    │────▶│  System 2  │
│  (fast)     │     │  Plan        │     │  (LLM)     │
│  NL → tag   │     │  tag → cmds  │     │  Execute    │
└─────────────┘     └──────────────┘     └─────────────┘
                    Fixed lookup           LLM fills in
                    table (Phase 1)        variables, runs
                                           commands, adapts
```

- **System1** (model): instant strategy selection from task description → strategy tag
- **Strategy plan**: deterministic mapping from strategy tag → command template
- **System2** (LLM): fills in variables (repo URL, file names, agent count), executes, adapts on failure

## Training Data Format

Each scenario in the catalog becomes a training example:

```jsonl
{"task":"Fix a bug by trying 3 approaches in parallel and picking the best","strategy":"strategy.parallel_hypotheses","features":{"agent_count":3,"has_hypotheses":true,"has_merge":true,"merge_granularity":"full"}}
{"task":"Build a feature with 5 agents, wave 1 does models, wave 2 does API, wave 3 does tests","strategy":"strategy.multi_wave","features":{"agent_count":5,"dependency_type":"sequential","wave_count":3}}
{"task":"Take routes.go from agent-A and store.go from agent-B","strategy":"strategy.selective_merge_3","features":{"agent_count":2,"has_merge":true,"merge_granularity":"selective"}}
```

The 100-scenario catalog provides the labeled dataset. Additional synthetic examples can be generated by varying agent counts, dependency types, and task descriptions while keeping the same strategy tag.

## Evaluation Metrics

| Metric | Target | How to measure |
|--------|--------|----------------|
| Strategy accuracy | >90% | Run selector on held-out task descriptions, compare to human-labeled strategy |
| Command sequence validity | 100% | Generated commands must execute without error |
| Time to select | <10ms | System1 model inference latency |
| Coverage | 35 strategies | Every strategy tag has at least 1 training example |

## Repository Integration

The strategy selector would ship as part of the `ws` skill:

```
~/.hermes/skills/ws-workspace-graph/
  SKILL.md                    # Agent-facing patterns (current)
  strategy-selector.py        # Phase 1: rule-based selector
  strategy-tags.json          # Tag → command template mapping
  training-data.jsonl         # 100-scenario labeled dataset
```

When an agent loads the skill, it can either:
1. Read SKILL.md and reason through patterns (current, System2 only)
2. Call `strategy-selector.py` with the task description → get a strategy tag + command template → fill in variables → execute (System1 + System2)
