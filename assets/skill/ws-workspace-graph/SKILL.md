---
name: ws-workspace-graph
description: Use ws (workspace graph CLI) for managing mutable workspaces on immutable content-addressed layers. Covers 10 reproducible multi-agent scenarios, full CLI command reference, and platform behavior.
---

# ws Workspace Graph — Agent Skill

## When to use me

You have been asked to work on a codebase, and you need a workspace. The codebase is managed by `ws`. Follow the patterns below to create, modify, share, and clean up your work safely.

## What ws is

`ws` creates **mutable workspaces** from **immutable layers**. Think Git worktrees, but:
- You can branch from another agent's uncommitted work
- You can isolate to a guaranteed-clean base
- No Git index races
- Content-addressed: hash `a3f4d...` is that content, forever

## Platform behavior

| OS      | Default backend | Fork cost | Overlayfs? |
|---------|----------------|-----------|------------|
| Linux   | overlayfs       | O(1)      | Yes (fuse) |
| macOS   | copy            | O(n)      | No         |
| macOS+* | container VM    | ~VM boot  | Yes        |

*macOS Apple Silicon with `container` (github.com/apple/container) installed

## The 10 Scenarios (use these)

### Scenario 1: Fresh Start (isolation)

Use when you want a clean slate, unaffected by other work. This is the safest pattern.

```bash
# Clone a repo into a clean workspace
ws get base:<repo> --name=<task>

# Work inside the workspace
ws run <task> -- make test

# Check what changed before keeping
ws diff <task>

# Promote workspace to an immutable layer
ws keep <task> --message="fixed nil pointer in handler.go" --json

# Parse the JSON output to extract the layer hash for coordination
# Example JSON: {"workspace":"task","layer":"a3f4d2e1c8b9...","message":"..."}

# Clean up the workspace (the layer persists)
ws drop <task>
```

With `--json` on `keep`, the output is machine-readable so coordinating agents can programmatically capture the layer hash:

```bash
LAYER_HASH=$(ws keep <task> --message="done" --json | jq -r '.layer')
```

### Scenario 2: Build on Another Agent's Work (collaboration)

Use when you need to continue from another agent's current state, even if they haven't kept yet. The source workspace is snapshotted into a new layer first, then you fork from that layer.

```bash
# Fork from another agent's workspace (includes uncommitted state)
ws get ws:<source-workspace> --name=<task>

# Continue the work
ws run <task> -- go build ./...

# Keep your incremental work as a new layer
ws keep <task> --message="added input validation on top of agent-A work"
```

### Scenario 3: Checkpoint Recovery (safety)

Use for long-running tasks to create safe rollback points. A layer is cheap; a dropped workspace with hours of work is expensive.

```bash
ws get base:<repo> --name=<task>
ws run <task> -- <command-1>
ws keep <task> --message="after step 1"
# Layer hash: e.g. a3f4d2e1c8b9

ws run <task> -- <command-2>
ws keep <task> --message="after step 2"
# Layer hash: e.g. b7c8d9e0f1a2

# If step 3 breaks everything, roll back:
ws drop <task>
ws get layer:b7c8d9e0f1a2 --name=<task-recovered>
ws run <task-recovered> -- go test ./...
```

### Scenario 4: Parallel Tasks (independent workstreams)

Use when two tasks branch from the same base but do not interact.

```bash
ws get base:<repo> --name=task-a
ws get base:<repo> --name=task-b

# Work on both independently — each workspace is fully isolated
ws run task-a -- go test ./pkg/auth/...
ws run task-b -- go test ./pkg/store/...

ws keep task-a --message="result A: auth tests pass"
ws keep task-b --message="result B: store tests pass"
```

### Scenario 5: Merge (conflict resolution)

`ws` does NOT implement merge. Overlayfs itself does not merge — it overlays. True three-way merge is a policy decision implemented in a workspace with standard tools, followed by `ws keep`.

```bash
# Fork from the common base layer
ws get layer:<base-hash> --name=merge

# Bring in changes from both branches using standard tools
# For example, copy files from two different layer checkpoints:
ws layer copy <layer-a-hash> pkg/auth/handler.go ./pkg/auth/handler.go
ws layer copy <layer-b-hash> pkg/store/db.go ./pkg/store/db.go

# Or use git merge-file for three-way merge of a specific file
ws run merge -- git merge-file pkg/auth/handler.go.base pkg/auth/handler.go.common pkg/auth/handler.go.theirs

# Resolve any conflicts manually, then keep the merged result
ws diff merge
ws keep merge --message="merged auth handler changes from A and B"
```

### Scenario 6: Cleanup (hygiene)

Run after work is completed to free disk space. Drop all agent workspaces, then garbage-collect unreferenced layers.

```bash
# Drop multiple workspaces at once
ws drop agent-1 agent-2 agent-3

# Garbage-collect layers no longer referenced by any workspace
ws layer gc
```

### Scenario 7: Plan-Execute-Consolidate (the multi-agent demo pattern)

A planner creates N workspaces from a base. Workers execute in parallel. Dependent agents branch from workers. A consolidator merges all layers.

This is the canonical pattern for multi-agent software development with `ws`.

**Concrete example: 6 agents building a Go HTTP API.**

#### Role: Planner

The planner clones the repo and creates one workspace per worker:

```bash
# Clone the base repo once
ws get base:github.com/example/api-project --name=base-ws
ws keep base-ws --message="base for API build" --json
# Capture the base layer hash:
BASE_HASH=$(ws keep base-ws --message="base for API build" --json | jq -r '.layer')

# Create a workspace for each worker, all branching from the same base layer
ws get layer:$BASE_HASH --name=worker-routes
ws get layer:$BASE_HASH --name=worker-middleware
ws get layer:$BASE_HASH --name=worker-models
ws get layer:$BASE_HASH --name=worker-db
ws get layer:$BASE_HASH --name=worker-tests
ws get layer:$BASE_HASH --name=worker-docs

# Drop the planner's workspace — workers each have their own
ws drop base-ws

# Broadcast: each worker now runs in its own workspace
```

#### Role: Worker (example: worker-routes)

Each worker executes its assigned task independently:

```bash
# Get the filesystem path to the workspace
ws path worker-routes
# /home/gweslien/.ws/workspaces/worker-routes

# Implement the assigned package
ws run worker-routes -- mkdir -p internal/routes
ws run worker-routes -- go mod tidy
ws run worker-routes -- sh -c 'cat > internal/routes/handler.go << EOF
package routes

import "net/http"

func HealthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("ok"))
}
EOF'

# Run any tests relevant to this package
ws run worker-routes -- go build ./...

# Keep the work as an immutable layer and capture the hash
ws keep worker-routes --message="routes: implemented health handler" --json
# {"workspace":"worker-routes","layer":"a1b2c3d4e5f6","message":"routes: ..."}
```

Each other worker follows the same pattern for their package. After all workers `keep`, the planner (or a coordinator) has 6 layer hashes.

#### Role: Dependent Agent (branch from a worker)

A dependent agent branches from a worker's completed layer to build on it:

```bash
# Branch from worker-routes' kept layer
ws get layer:a1b2c3d4e5f6 --name=dep-auth-routes

# Add authentication middleware on top of the routes
ws run dep-auth-routes -- sh -c 'cat > internal/middleware/auth.go << EOF
package middleware

import "net/http"

func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("Authorization") == "" {
            w.WriteHeader(http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
EOF'

ws keep dep-auth-routes --message="auth middleware on top of routes" --json
# Capture: {"layer":"f7e8d9c0b1a2",...}
```

#### Role: Consolidator

The consolidator merges all layers into a single workspace:

```bash
# Fork from the original base
ws get layer:$BASE_HASH --name=consolidated

# Merge each worker's contribution by copying their files from their layers
# Use ws layer copy to pull specific files from each worker's layer
ws layer copy a1b2c3d4e5f6 internal/routes/handler.go ./internal/routes/handler.go
ws layer copy b2c3d4e5f6a7 internal/middleware/cors.go ./internal/middleware/cors.go
ws layer copy c3d4e5f6a7b8 internal/models/user.go ./internal/models/user.go
ws layer copy d4e5f6a7b8c9 internal/db/conn.go ./internal/db/conn.go
ws layer copy e5f6a7b8c9d0 internal/api/handlers_test.go ./internal/api/handlers_test.go
ws layer copy f6a7b8c9d0e1 README.md ./README.md

# Inspect what was merged
ws diff consolidated

# Build and test the consolidated result
ws run consolidated -- go build ./...
ws run consolidated -- go test ./...

# Keep the merged result as a new layer
ws keep consolidated --message="consolidated all 6 agent layers into working API" --json
```

#### Cross-agent visibility with `ws status`

Any agent can check the state of all workspaces at any time:

```bash
ws status --json
# Returns machine-readable JSON array of all workspaces with dirty/clean state:
# [
#   {"name":"worker-routes","base":"a1b2...","dirty":false,"path":"/home/..."},
#   {"name":"worker-middleware","base":"a1b2...","dirty":true,"path":"/home/..."},
#   ...
# ]

# Human-readable table (default):
ws status
# NAME               BASE       DIRTY  PATH
# worker-routes      a1b2c3...  no     /home/.../worker-routes
# worker-middleware  a1b2c3...  yes    /home/.../worker-middleware
# ...
```

#### Cross-agent visibility with `ws graph`

The dependency graph shows how all workspaces and layers relate:

```bash
ws graph --json
# Machine-readable graph of workspace → layer relationships

ws graph
# Human-readable tree:
# base-ws [kept: a1b2c3]
# ├── worker-routes [kept: a1b2c3 → d4e5f6]
# │   └── dep-auth-routes [kept: f7e8d9]
# ├── worker-middleware [kept: b2c3d4]
# ├── worker-models [kept: c3d4e5]
# ├── worker-db [kept: d4e5f6]
# ├── worker-tests [kept: e5f6a7]
# └── worker-docs [kept: f6a7b8]
#     └── consolidated [dirty]
```

#### Cleanup after consolidation

```bash
# Drop all agent workspaces at once
ws drop worker-routes worker-middleware worker-models worker-db worker-tests worker-docs
ws drop dep-auth-routes
ws drop consolidated

# Garbage-collect unreferenced layers
ws layer gc
```

### Scenario 8: Parallel Code Review

Multiple agents review different parts of a codebase in isolated workspaces. Each agent checks out the repo, reviews their assigned files, and keeps a layer with review comments. A consolidator merges all review layers into one review report.

```bash
# Each reviewer gets a clean fork of the repo
ws get base:github.com/example/service --name=review-auth
ws get base:github.com/example/service --name=review-api
ws get base:github.com/example/service --name=review-db

# Reviewer 1: review authentication code
ws run review-auth -- sh -c 'cat > REVIEW.md << EOF
# Auth Code Review

## Findings
- [HIGH] token validation missing in middleware/auth.go:42
- [LOW] unnecessary re-export in auth/token.go:15
EOF'

# Reviewer 2: review API handlers
ws run review-api -- sh -c 'cat > REVIEW.md << EOF
# API Code Review

## Findings
- [MEDIUM] missing input length check in api/handlers.go:88
- [INFO] consistent error handling in api/middleware.go
EOF'

# Reviewer 3: review database layer
ws run review-db -- sh -c 'cat > REVIEW.md << EOF
# DB Code Review

## Findings
- [HIGH] SQL injection risk in db/queries.go:23
- [MEDIUM] missing context timeout in db/conn.go:12
EOF'

# Each reviewer keeps their layer
ws keep review-auth --message="auth review complete" --json
ws keep review-api --message="api review complete" --json
ws keep review-db --message="db review complete" --json

# Consolidator merges all review files
ws get base:github.com/example/service --name=review-consolidated

# Copy each reviewer's REVIEW.md with a unique name
ws layer copy <auth-hash> REVIEW.md ./REVIEW-auth.md
ws layer copy <api-hash>  REVIEW.md ./REVIEW-api.md
ws layer copy <db-hash>   REVIEW.md ./REVIEW-db.md

# Merge into a single report
ws run review-consolidated -- sh -c 'cat REVIEW-auth.md REVIEW-api.md REVIEW-db.md > FINAL-REVIEW.md'

ws keep review-consolidated --message="consolidated code review report"

# Cleanup
ws drop review-auth review-api review-db review-consolidated
ws layer gc
```

### Scenario 9: Iterative Refactoring with Rollback

An agent refactors a module across multiple checkpoints. If tests fail at a checkpoint, the agent rolls back to the previous layer using `ws get layer:<hash>`.

```bash
# Start from the repo
ws get base:github.com/example/service --name=refactor

# Checkpoint 1: extract interface
ws run refactor -- sh -c 'cat > internal/store/interface.go << EOF
package store

type Store interface {
    Get(id string) ([]byte, error)
    Put(id string, data []byte) error
}
EOF'

ws run refactor -- go test ./...
ws keep refactor --message="checkpoint 1: extracted Store interface" --json
# CP1: hash = e1a2b3c4d5e6

# Checkpoint 2: implement new store backend
ws run refactor -- sh -c 'cat > internal/store/pg.go << EOF
package store

import "database/sql"

type PGStore struct { db *sql.DB }
func (s *PGStore) Get(id string) ([]byte, error) { /* ... */ return nil, nil }
func (s *PGStore) Put(id string, data []byte) error { /* ... */ return nil }
EOF'

ws run refactor -- go test ./...
# Tests FAIL — the new PGStore has a nil pointer bug

# Roll back to checkpoint 1
ws drop refactor
ws get layer:e1a2b3c4d5e6 --name=refactor

# Verify the rollback restored the working state
ws run refactor -- go test ./...
# Tests PASS — back to checkpoint 1

# Checkpoint 2 (retry): fix the nil pointer and reimplement
ws run refactor -- sh -c 'cat > internal/store/pg.go << EOF
package store

import "database/sql"

type PGStore struct { db *sql.DB }
func NewPGStore(db *sql.DB) *PGStore { return &PGStore{db: db} }
func (s *PGStore) Get(id string) ([]byte, error) { if s.db == nil { return nil, fmt.Errorf("nil db") }; /* ... */ return nil, nil }
func (s *PGStore) Put(id string, data []byte) error { if s.db == nil { return fmt.Errorf("nil db") }; /* ... */ return nil }
EOF'

ws run refactor -- go test ./...
ws keep refactor --message="checkpoint 2: PGStore with nil guard" --json
# CP2: hash = f2b3c4d5e6f7

# Checkpoint 3: migrate callers to use NewPGStore
ws run refactor -- go test ./...
ws keep refactor --message="checkpoint 3: all callers migrated" --json

# Final cleanup
ws drop refactor
```

### Scenario 10: Bug Hunt (parallel investigation)

Multiple agents investigate different potential causes of a bug. Each agent has their own workspace with the same base. They test different hypotheses. The consolidator picks the fix that works and merges it.

```bash
# Shared base for all investigators
ws get base:github.com/example/service --name=bug-base
ws keep bug-base --message="base for bug hunt: nil pointer in /users endpoint" --json
BASE_HASH=$(ws keep bug-base --message="base for bug hunt" --json | jq -r '.layer')
ws drop bug-base

# Each investigator forks from the same base
ws get layer:$BASE_HASH --name=hunt-conn-pool
ws get layer:$BASE_HASH --name=hunt-nil-check
ws get layer:$BASE_HASH --name=hunt-race

# Investigator 1: hypothesis — connection pool exhaustion
ws run hunt-conn-pool -- sh -c 'echo "setMaxOpenConns(1)" >> internal/db/conn.go && go test ./... 2>&1'
# Result: tests still fail — hypothesis rejected
ws keep hunt-conn-pool --message="HYPOTHESIS REJECTED: conn pool not the cause" --json

# Investigator 2: hypothesis — missing nil check on user lookup
ws run hunt-nil-check -- sh -c 'sed -i "s/return user/return user, err/" internal/api/handlers.go && go test ./... 2>&1'
# Result: tests PASS — hypothesis confirmed
ws keep hunt-nil-check --message="HYPOTHESIS CONFIRMED: nil check fixes the bug" --json
# WINNER: hash = a9b8c7d6e5f4

# Investigator 3: hypothesis — race condition
ws run hunt-race -- sh -c 'go test -race ./... 2>&1'
# Result: no race detected — hypothesis rejected
ws keep hunt-race --message="HYPOTHESIS REJECTED: no race condition found" --json

# Consolidator picks the winning fix
ws get layer:a9b8c7d6e5f4 --name=bug-fix-final

# Verify the fix works in a clean workspace
ws run bug-fix-final -- go test ./...

# Export the fixed workspace to an external directory for delivery
ws export bug-fix-final /home/gweslien/bug-fix-delivery/

# Keep the final fix as a layer
ws keep bug-fix-final --message="merged winning fix: nil check on user lookup" --json

# Cleanup all investigation workspaces
ws drop hunt-conn-pool hunt-nil-check hunt-race bug-fix-final
ws layer gc
```

## Rules for Agents

1. **ALWAYS keep before communicating results.** Your workspace is ephemeral. The layer is permanent and addressable.
2. **NEVER drop another agent's workspace.** Only your own. Coordination is via layer hashes, not workspace names.
3. **PREFER `base:` over `ws:`** when you don't need the other agent's work. Isolation is safer than shared state.
4. **SMALL units of work.** Keep frequently. A layer is cheap.
5. **CHECK before keeping.** `ws diff <workspace>` shows exactly what will be committed.
6. **NAME meaningfully.** 'bug-1234-fix' is better than 'workspace-7'.
7. **USE `--json`** when consuming ws output programmatically. Text output is for humans. Layer hashes, workspace lists, and status are all available as structured JSON.
8. **USE `ws path <workspace>`** to get the filesystem path. Don't hardcode paths — workspace locations vary by platform and install.
9. **USE `ws get dir:<path>`** for local directories. Don't `git init` throwaway repos just to get a workspace.
10. **CLEAN UP after consolidation:** `ws drop` all agent workspaces, then `ws layer gc`.

## Command Reference

| Command | Purpose |
|---------|---------|
| `ws get layer:<hash> --name=<ws>` | Fork from an immutable layer |
| `ws get ws:<name> --name=<ws>` | Branch from another workspace |
| `ws get base:<repo>#<ref> --name=<ws>` | Clone repo and create workspace |
| `ws get dir:<path> --name=<ws>` | Create workspace from a local directory (no git repo needed) |
| `ws run <ws> -- <cmd>` | Execute command inside workspace |
| `ws diff <ws>` | Diff workspace vs its base layer |
| `ws diff <ws-a> <ws-b>` | Diff two workspaces |
| `ws keep <ws> --message=...` | Promote workspace to immutable layer |
| `ws drop <ws> [<ws> ...]` | Destroy one or more workspaces (destructive) |
| `ws graph [ws]` | Print dependency graph |
| `ws status` | All workspaces table with dirty/clean state |
| `ws path <workspace>` | Print filesystem path to workspace |
| `ws export <workspace> <dest>` | Copy workspace contents to external directory |
| `ws layer ls` | List all layers |
| `ws layer show <hash>` | Show layer metadata |
| `ws layer diff <hash-a> <hash-b>` | Diff two layers |
| `ws layer cat <hash> <file>` | Print a file from a layer |
| `ws layer copy <hash> <file> <dest>` | Copy a file from a layer to a destination path |
| `ws layer path <hash>` | Print filesystem path to a layer |
| `ws layer files <hash>` | List files in a layer |
| `ws layer gc` | Garbage-collect unreferenced layers |
| `ws skill` | Install this skill into Hermes |
| `ws update` | Self-update ws binary |
| `ws help <topic>` | Detailed help (topic = command, agent, concepts) |

### Flags

| Flag | Applies to | Purpose |
|------|-----------|---------|
| `--name=<ws>` | `get` | Name the new workspace |
| `--message=...` | `keep` | Attach a message to the layer |
| `--json` | `get`, `keep`, `layer ls`, `status`, `graph` | Machine-readable JSON output for agent consumption |

## Help topics built in

- `ws help get` / `run` / `diff` / `keep` / `drop` / `graph` / `path` / `export` / `layer` / `skill` / `update`
- `ws help agent` — the 10 scenarios and rules above
- `ws help concepts` — layers, workspaces, graph, merge semantics, GC
