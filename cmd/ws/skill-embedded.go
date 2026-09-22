package main

// autonomous-ai-agents: Skill for using ws (workspace graph CLI) in agent workflows.
//
// Generated from assets/skill/ws-workspace-graph/SKILL.md — edit the original,
// then copy here or regenerate.

const defaultSkillName = "ws-workspace-graph"

const defaultSkillContent = `---
name: ws-workspace-graph
description: Use ws (workspace graph CLI) for managing mutable workspaces on immutable content-addressed layers. Covers 6 agent patterns, CLI commands, and backend behavior.
---

# ws Workspace Graph — Agent Skill

## When to use me

You have been asked to work on a codebase, and you need a workspace. The codebase is managed by ` + "`ws`" + `. You should follow the patterns below to create, modify, share, and clean up your work safely.

## What ws is

` + "`ws`" + ` creates **mutable workspaces** from **immutable layers**. Think Git worktrees, but:
- You can branch from another agent's uncommitted work
- You can isolate to a guaranteed-clean base
- No Git index races
- Content-addressed: hash ` + "`a3f4d...`" + ` is that content, forever

## Platform behavior

| OS      | Default backend | Fork cost | Overlayfs? |
|---------|----------------|-----------|------------|
| Linux   | overlayfs       | O(1)      | Yes (fuse) |
| macOS   | copy            | O(n)      | No         |
| macOS+* | container VM    | ~VM boot  | Yes        |

*macOS Apple Silicon with ` + "`container`" + ` (github.com/apple/container) installed

## The 6 Patterns (use these)

### Pattern 1: Fresh Start (isolation)

` + "```bash\nws get base:<repo> --name=<task>\nws run <task> -- <command>\nws keep <task> --message=\"<what was done>\"\nws drop <task>\n```" + `

Use when you want a clean slate, unaffected by other work. This is the safest pattern.

### Pattern 2: Build on Another Agent's Work (collaboration)

` + "```bash\nws get ws:<source-workspace> --name=<task>\nws run <task> -- <command>\nws keep <task> --message=\"<incremental work>\"\n```" + `

Use when you need to continue from another agent's current state, even if they haven't kept yet. The source workspace is snapshotted into a new layer first, then you fork from that layer.

### Pattern 3: Checkpoint (safety)

` + "```bash\nws get base:<repo> --name=<task>\nws run <task> -- <command-1>\nws keep <task> --message=\"after step 1\"\nws run <task> -- <command-2>\nws keep <task> --message=\"after step 2\"\n# Recovery: ws get layer:<step-2-hash> --name=<task-recovered>\n```" + `

Use for long-running tasks to create safe rollback points. A layer is cheap; a dropped workspace with hours of work is expensive.

### Pattern 4: Parallel Tasks (independent workstreams)

` + "```bash\nws get base:<repo> --name=task-a\nws get base:<repo> --name=task-b\n# work on both independently\nws keep task-a --message=\"result A\"\nws keep task-b --message=\"result B\"\n```" + `

Use when two tasks branch from the same base but do not interact.

### Pattern 5: Merge (conflict resolution)

` + "```bash\nws get layer:<base> --name=merge\nws run merge -- <standard merge tool: git merge-file, diff3, etc.>\nws keep merge --message=\"merged A and B\"\n```" + `

` + "`ws`" + ` does NOT implement merge. Overlayfs itself does not merge — it overlays. True three-way merge is a policy decision implemented in a workspace with standard tools, followed by ` + "`ws keep`" + `.

### Pattern 6: Cleanup (hygiene)

` + "```bash\nws drop <task-a>\nws drop <task-b>\nws layer gc\n```" + `

Run after work is completed to free disk space.

## Rules for Agents

1. **ALWAYS keep before communicating results.** Your workspace is ephemeral. The layer is permanent and addressable.
2. **NEVER drop another agent's workspace.** Only your own. Coordination is via layer hashes, not workspace names.
3. **PREFER base: over ws:** when you don't need the other agent's work. Isolation is safer than shared state.
4. **SMALL units of work.** Keep frequently. A layer is cheap.
5. **CHECK before keeping.** ` + "`ws diff <workspace>`" + ` shows exactly what will be committed.
6. **NAME meaningfully.** ` + "`bug-1234-fix`" + ` is better than ` + "`workspace-7`" + `.

## Command Reference

| Command | Purpose |
|---------|---------|
| ` + "`ws get layer:<hash> --name=<ws>`" + ` | Fork from an immutable layer |
| ` + "`ws get ws:<name> --name=<ws>`" + ` | Branch from another workspace |
| ` + "`ws get base:<repo>#<ref> --name=<ws>`" + ` | Clone repo and create workspace |
| ` + "`ws run <ws> -- <cmd>`" + ` | Execute command inside workspace |
| ` + "`ws diff <ws>`" + ` | Diff workspace vs its base layer |
| ` + "`ws diff <ws-a> <ws-b>`" + ` | Diff two workspaces |
| ` + "`ws keep <ws> --message=...`" + ` | Promote workspace to immutable layer |
| ` + "`ws drop <ws>`" + ` | Destroy workspace (destructive) |
| ` + "`ws graph [ws]`" + ` | Print dependency graph |
| ` + "`ws layer ls`" + ` | List all layers |
| ` + "`ws layer show <hash>`" + ` | Show layer metadata |
| ` + "`ws layer gc`" + ` | Garbage-collect unreferenced layers |
| ` + "`ws help <topic>`" + ` | Detailed help (topic = command, agent, concepts) |
| ` + "`ws skill`" + ` | Install this skill into your Hermes skill directory |

## Help topics built in

- ` + "`ws help get`" + ` / ` + "`run`" + ` / ` + "`diff`" + ` / ` + "`keep`" + ` / ` + "`drop`" + ` / ` + "`graph`" + ` / ` + "`layer`" + `
- ` + "`ws help agent`" + ` — the 6 patterns and rules above
- ` + "`ws help concepts`" + ` — layers, workspaces, graph, merge semantics, GC
`
