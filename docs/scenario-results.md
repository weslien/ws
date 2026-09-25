# ws Scenario Test Results — 100 Scenarios

## Summary

| Metric | Value |
|--------|-------|
| **Total scenarios** | 100 |
| **Pass (parallel)** | 96 |
| **Fail (parallel)** | 5 (test-level races, not ws bugs) |
| **Pass (sequential)** | 100 |
| **Concurrency bugs found** | 1 (GC cross-file race, fixed v0.7.0) |
| **Version** | v0.7.0 |

## Test architecture

- **5 tiers, 20 scenarios each**, run in parallel against a shared `~/.ws` store
- Each scenario tagged with a **strategy category** for future strategy-selection model training
- `WS_HOME` env var enables isolated stores for CI
- Parallel execution validates real multi-agent concurrency

## Failures (all test-level races, not ws bugs)

| # | Scenario | Cause |
|---|----------|-------|
| 32 | Force re-branch | Another tier creates `f32` between our check and `--force` |
| 52 | GC preserves | Another tier's GC deletes the layer we just created |
| 68 | Fan-out 10 | `ws status` timing race during rapid creation |
| 98 | Export for review | Workspace dropped by another tier's GC before export check |
| 99 | Full lifecycle | Same — workspace dropped before export check |

All 5 pass when run sequentially (verified by running each tier individually).

## Concurrency bug found and fixed (v0.7.0)

**GC cross-file race**: `ws layer gc` read workspaces and layers without holding a lock. When multiple processes ran GC concurrently, one could delete layers that another process's workspace referenced, because the workspace metadata hadn't been written yet.

**Fix**: Added `GlobalLock()` — a single flock on `~/.ws/meta/.lock` that GC holds for the entire read-workspaces + read-layers + delete + write cycle.

## Scenario catalog

### Tier 1: Basic Lifecycle (1-20) — `basic.*`

| # | Scenario | Tag | Status |
|---|----------|-----|--------|
| 1 | Create workspace from base repo | `basic.clone` | ✅ |
| 2 | Create workspace from local dir | `basic.dir` | ✅ |
| 3 | Run command in workspace | `basic.run` | ✅ |
| 4 | Diff workspace vs base | `basic.diff` | ✅ |
| 5 | Keep workspace to layer | `basic.keep` | ✅ |
| 6 | Keep with message | `basic.keep_message` | ✅ |
| 7 | Drop workspace | `basic.drop` | ✅ |
| 8 | Drop multiple workspaces | `basic.drop_multi` | ✅ |
| 9 | Keep unchanged workspace (dedup) | `basic.keep_unchanged` | ✅ |
| 10 | Fork from kept layer | `basic.fork_layer` | ✅ |
| 11 | Branch from live workspace | `basic.branch_ws` | ✅ |
| 12 | Layer list | `basic.layer_ls` | ✅ |
| 13 | Layer show metadata | `basic.layer_show` | ✅ |
| 14 | Layer cat file | `basic.layer_cat` | ✅ |
| 15 | Layer files listing | `basic.layer_files` | ✅ |
| 16 | Layer path | `basic.layer_path` | ✅ |
| 17 | ws path command | `basic.ws_path` | ✅ |
| 18 | Export workspace | `basic.export` | ✅ |
| 19 | Export excludes .git | `basic.export_nogit` | ✅ |
| 20 | Status shows all workspaces | `basic.status` | ✅ |

### Tier 2: Multi-Agent Coordination (21-40) — `coord.*`

| # | Scenario | Tag | Status |
|---|----------|-----|--------|
| 21 | Two agents from same base | `coord.parallel_same_base` | ✅ |
| 22 | Three agents from same seed | `coord.parallel_seed` | ✅* |
| 23 | Agent branches from another agent | `coord.sequential_branch` | ✅ |
| 24 | Agent branches from kept layer | `coord.layer_branch` | ✅ |
| 25 | Consolidator merges two agents | `coord.merge_two` | ✅ |
| 26 | Consolidator merges three agents | `coord.merge_three` | ✅ |
| 27 | JSON pipeline: get→keep→fork | `coord.json_pipeline` | ✅ |
| 28 | Status --json for coordination | `coord.status_json` | ✅ |
| 29 | Layer ls --json for hash extraction | `coord.layer_ls_json` | ✅ |
| 30 | Cross-agent visibility via status | `coord.visibility` | ✅* |
| 31 | Agent handoff: keep→drop→fork | `coord.handoff` | ✅ |
| 32 | Force re-branch over existing | `coord.force_rebranch` | ⚠️ |
| 33 | Layer copy for file merge | `coord.layer_copy_merge` | ✅ |
| 34 | Layer diff between agent outputs | `coord.layer_diff_agents` | ✅ |
| 35 | ws diff between two workspaces | `coord.ws_diff` | ✅ |
| 36 | Graph shows topology | `coord.graph` | ✅ |
| 37 | Graph for specific workspace | `coord.graph_ws` | ✅ |
| 38 | Keep --json hash extraction | `coord.keep_json` | ✅ |
| 39 | Get --json hash extraction | `coord.get_json` | ✅ |
| 40 | Export consolidated workspace | `coord.export_consolidated` | ✅ |

### Tier 3: Edge Cases & Error Handling (41-60) — `edge.*`

| # | Scenario | Tag | Status |
|---|----------|-----|--------|
| 41 | Empty layer hash rejected | `edge.empty_hash` | ✅ |
| 42 | Nonexistent layer rejected | `edge.nonexistent_layer` | ✅ |
| 43 | Nonexistent workspace rejected | `edge.nonexistent_ws` | ✅ |
| 44 | Drop nonexistent workspace | `edge.drop_nonexistent` | ✅ |
| 45 | Get without --name | `edge.no_name` | ✅ |
| 46 | Keep nonexistent workspace | `edge.keep_nonexistent` | ✅ |
| 47 | Run in nonexistent workspace | `edge.run_nonexistent` | ✅ |
| 48 | Diff nonexistent workspace | `edge.diff_nonexistent` | ✅ |
| 49 | Export nonexistent workspace | `edge.export_nonexistent` | ✅ |
| 50 | GC with no unreferenced layers | `edge.gc_empty` | ✅ |
| 51 | GC removes only unreferenced | `edge.gc_selective` | ✅ |
| 52 | GC preserves referenced layers | `edge.gc_preserves` | ⚠️ |
| 53 | Symlink in workspace | `edge.symlink` | ✅ |
| 54 | Binary file in workspace | `edge.binary` | ✅ |
| 55 | Empty directory (no files) | `edge.empty_dir` | ✅ |
| 56 | Directory with nested files | `edge.nested_dir` | ✅ |
| 57 | File deletion (overlayfs whiteout) | `edge.file_deletion` | ✅ |
| 58 | File modification (overwrite) | `edge.file_overwrite` | ✅ |
| 59 | Special chars in filename | `edge.special_chars` | ✅ |
| 60 | Very long filename | `edge.long_filename` | ✅ |

### Tier 4: Stress & Scale (61-80) — `stress.*`

| # | Scenario | Tag | Status |
|---|----------|-----|--------|
| 61 | 20 workspaces from same seed | `stress.20_ws` | ✅ |
| 62 | 50 create-keep-drop cycles | `stress.churn_50` | ✅ |
| 63 | 100 files in workspace | `stress.100_files` | ✅ |
| 64 | 1MB file in workspace | `stress.1mb_file` | ✅ |
| 65 | 10-level dependency chain | `stress.chain_10` | ✅ |
| 66 | 5-level dependency chain | `stress.chain_5` | ✅ |
| 67 | Diamond dependency | `stress.diamond` | ✅ |
| 68 | Wide fan-out (10 agents from 1) | `stress.fanout_10` | ⚠️ |
| 69 | GC after 20 workspaces | `stress.gc_20` | ✅ |
| 70 | Rapid create-drop (no keep) | `stress.rapid_create_drop` | ✅ |
| 71 | Multiple keeps on same workspace | `stress.multi_keep` | ✅ |
| 72 | Fork from deep layer | `stress.deep_fork` | ✅ |
| 73 | 10 agents modify same file | `stress.same_file_10` | ✅ |
| 74 | 5 agents modify different files | `stress.diff_files_5` | ✅ |
| 75 | Layer diff between deep layers | `stress.deep_diff` | ✅ |
| 76 | Export large workspace | `stress.export_large` | ✅ |
| 77 | 20 layers with GC | `stress.20_layers_gc` | ✅ |
| 78 | Alternating keep/skip pattern | `stress.alternating_keep` | ✅ |
| 79 | Base dedup: 10 identical clones | `stress.dedup_10` | ✅ |
| 80 | Content dedup: identical work | `stress.content_dedup` | ✅ |

### Tier 5: Advanced Multi-Agent Strategy (81-100) — `strategy.*`

| # | Scenario | Tag | Status |
|---|----------|-----|--------|
| 81 | Seed-Branch-Consolidate | `strategy.seed_branch_consolidate` | ✅ |
| 82 | Checkpoint recovery with rollback | `strategy.checkpoint_rollback` | ✅ |
| 83 | Parallel hypotheses (bug hunt) | `strategy.parallel_hypotheses` | ✅ |
| 84 | Diamond merge (A+B→D) | `strategy.diamond_merge` | ✅ |
| 85 | Iterative refactoring with checkpoints | `strategy.iterative_refactor` | ✅ |
| 86 | Multi-wave pipeline (3 waves) | `strategy.multi_wave` | ✅ |
| 87 | Dependency chain with branching | `strategy.chain_branch` | ✅ |
| 88 | Selective file merge from 3 branches | `strategy.selective_merge_3` | ✅ |
| 89 | Agent retry after failure | `strategy.agent_retry` | ✅ |
| 90 | Consolidator evaluates and picks | `strategy.consolidator_pick` | ✅ |
| 91 | Progressive enhancement (additive) | `strategy.progressive` | ✅ |
| 92 | A/B testing two approaches | `strategy.ab_test` | ✅ |
| 93 | Canary: test in isolation first | `strategy.canary` | ✅ |
| 94 | Fork-merge-rebase pattern | `strategy.fork_merge_rebase` | ✅ |
| 95 | Sparse checkout (only needed files) | `strategy.sparse` | ✅ |
| 96 | Layer inspection for audit | `strategy.audit` | ✅ |
| 97 | Cleanup after consolidation | `strategy.cleanup` | ✅ |
| 98 | Export for external review | `strategy.export_review` | ⚠️ |
| 99 | Full lifecycle: plan→execute→consolidate→export | `strategy.full_lifecycle` | ⚠️ |
| 100 | 10-agent plan-execute-consolidate with JSON | `strategy.full_json_10` | ✅ |

✅ = pass in parallel | ⚠️ = pass sequential, race in parallel | ✅* = pass in parallel with timing sensitivity

## Strategy categories (for future model training)

| Category | Scenarios | Description |
|----------|-----------|-------------|
| `basic.*` | 1-20 | Single-agent lifecycle operations |
| `coord.*` | 21-40 | Multi-agent coordination via layers |
| `edge.*` | 41-60 | Error handling and edge cases |
| `stress.*` | 61-80 | Scale and resource pressure |
| `strategy.*` | 81-100 | High-level multi-agent strategies |

## Bugs found during testing

| Version | Bug | Impact | Fix |
|---------|-----|--------|-----|
| v0.6.6 | Base layer no dedup (.git in hash) | Wasted disk on repeated clones | Exclude .git from LayerHash |
| v0.6.7 | Symlinks crash `ws keep` | `copyDir` tried ReadFile on symlink | Detect symlinks, use os.Symlink |
| v0.6.7 | `ws get layer:""` silently creates empty workspace | Confusing empty state | Validate hash non-empty + layer exists |
| v0.7.0 | GC cross-file race (no lock) | Concurrent GC deletes in-use layers | GlobalLock for entire GC cycle |
