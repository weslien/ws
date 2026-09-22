package main

//go:generate go run ./scripts/gen-embed skill --in=assets/skill/ws-workspace-graph/SKILL.md --out=cmd/ws/skill-embedded.go

import (
	"fmt"
	"os"
)

// ---------- Help System ----------

var cmdHelps = map[string]string{
	"get": `ws get <source> --name=<workspace>

Create a new workspace from a source.

SOURCE TYPES
  layer:<hash>      Fork from an immutable layer — you get a clean, isolated
                    workspace with the exact content of that layer.

  ws:<name>         Branch from another workspace — captures the workspace's
                    current state (including uncommitted changes) into a new
                    workspace. The source workspace keeps working independently.

  base:<repo>#<ref> Clone a git repository and create a workspace from it.
                    <repo> can be a local path or a remote URL.
                    <ref> defaults to HEAD if omitted.

OPTIONS
  --name=<name>     Required. The name of the new workspace.

EXAMPLES
  ws get base:https://github.com/you/project#main --name=feature-a
  ws get layer:a3f4d9c2e1b5 --name=fix-bug
  ws get ws:feature-a --name=feature-a-test

NOTES
  - Workspace names are arbitrary identifiers (letters, numbers, hyphens, underscores).
  - Two workspaces cannot share the same name. If you try, ws aborts.
  - On Linux, workspaces are overlayfs mounts (instant, zero-copy).
  - On macOS, workspaces are directory copies (O(n) for size n).`,

	"run": `ws run <workspace> -- <command> [args...]

Execute a command inside an existing workspace.

ARGUMENTS
  <workspace>       Name of the workspace to run in.
  --                Separator: everything after this is the command.
  <command>         The command to execute. Must be available in the workspace.

EXAMPLES
  ws run feature-a -- cat package.json
  ws run feature-a -- npm install
  ws run feature-a -- go test ./...
  ws run feature-a -- sh -c 'echo "hello" > test.txt'

NOTES
  - The command runs with the workspace directory as its working directory.
  - Stdin, stdout, and stderr are forwarded directly.
  - On Linux, writes go to the overlayfs upper directory — the lower layer is
    never modified, so isolation from the base is guaranteed.
  - Running two commands simultaneously in the same workspace may cause file
    races. For concurrent agents, use separate workspaces.`,

	"diff": `ws diff <workspace-A> [<workspace-B>|<layer-hash>]

Show differences between two filesystem states.

FORMS
  ws diff <workspace>                    Compare workspace to its base layer.
  ws diff <workspace-A> <workspace-B>    Compare two workspaces head-to-head.
  ws diff <workspace> layer:<hash>       Compare workspace to a specific layer.

OUTPUT
  Unified diff format (same as diff -ruN), suitable for piping to patch(1)
  or reading directly.

EXAMPLES
  ws diff feature-a
  ws diff feature-a feature-b
  ws diff feature-a layer:a3f4d9c2e1b5`,

	"keep": `ws keep <workspace> [--message=<text>]

Materialize all changes in a workspace into a new immutable layer.

ARGUMENTS
  <workspace>          Name of the workspace to commit.
  --message=<text>     Optional annotation for the layer.

WHAT IT DOES
  1. Computes a content-addressed hash of the entire workspace state.
  2. Copies the workspace content to ~/.ws/layers/<hash>.
  3. Records the layer in the graph (with parent = the workspace's formed_from).
  4. Updates the workspace's formed_from to the new layer.

RESULT
  The workspace now sits on top of the new layer. All its changes are
  permanently stored, shareable, and derivable. The old base layer is still
  retained if other workspaces depend on it.

EXAMPLES
  ws keep feature-a --message="added lodash and fixed auth middleware"

NOTES
  - If no changes were made since the last keep, no new layer is created.
  - keep is safe: it never modifies the workspace, only creates a new read-only
    snapshot.
  - After keep, the workspace tracks from the new layer.`,

	"drop": `ws drop <workspace>

Destroy a workspace and all its uncommitted changes.

WARNING
  This is DESTRUCTIVE. Any changes not kept are permanently lost.

WHAT IT DOES
  1. Unmounts the workspace (overlayfs on Linux, no-op on macOS).
  2. Removes the workspace directory, upper directory, and work directory.
  3. Removes the workspace from the metadata graph.
  4. Does NOT delete layers referenced by the workspace (those are shared).

EXAMPLES
  ws drop feature-a

NOTES
  - Keep before dropping if you want to save the work.
  - Other workspaces are unaffected — they either still reference the old
    layer (isolated) or their own copy (branched).
  - On Linux, leftover mounts from crashes are cleaned up by lazy unmount.
  - Safe to call even if the workspace is already gone.`,

	"graph": `ws graph [workspace]

Print the layer and workspace dependency graph.

FORMS
  ws graph               Show all layers and all active workspaces.
  ws graph <workspace>   Show the provenance tree for a specific workspace:
                         ws → layers → parent layers → ... → base.

OUTPUT FORMAT
  ws:<name> [active]
    └─ layer:<hash> [message]
      └─ layer:<hash> [message]
        └─ ...

EXAMPLES
  ws graph
  ws graph feature-a

NOTES
  - Inactive (dropped) workspaces do not appear.
  - Orphan layers (not reachable from any workspace) still appear in 'ws layer ls'
    but are hidden from graph. Use 'ws layer gc' to remove them.`,

	"layer": `ws layer <subcommand>

Manage the immutable layer store.

SUBCOMMANDS
  ls                      List all layers with creation time and message.
  show <hash>             Print detailed metadata for a layer.
  gc                      Garbage-collect unreferenced layers.

EXAMPLES
  ws layer ls
  ws layer show a3f4d9c2e1b5
  ws layer gc

GC BEHAVIOR
  - A layer is "referenced" if it is the formed_from of any workspace,
    or is an ancestor (parent, grandparent, ...) of a referenced layer.
  - Unreferenced layers are deleted permanently.
  - Always run gc AFTER dropping workspaces, not before.`,

	"help": `ws help [<topic>]

Print help information.

TOPICS
  Any command name: get, run, diff, keep, drop, graph, layer
  agent                Agent workflows and best practices
  concepts             Key concepts: layers, workspaces, merge semantics

EXAMPLES
  ws help get
  ws help agent`,
		"update": `ws update

Update the ws CLI binary to the latest release.

WHAT IT DOES
  1. Fetches the latest GitHub release for your platform (linux/darwin, amd64/arm64).
  2. Tries prebuilt release asset first (fastest).
  3. Falls back to go install, then installer script, then source build.
  4. Replaces the running binary in-place atomically (backup as .old).

STRATEGIES (tried in order)
  1. Prebuilt:  download ws-<ver>-<os>-<arch>.tar.gz, extract, replace.
  2. go install: go install github.com/weslien/ws/cmd/ws@latest
  3. installer:  curl -fsSL .../install.sh | bash
  4. source:      git clone + go build

EXAMPLES
  ws update

NOTES
  - Requires network access to GitHub releases and/or go.mod proxy.
  - On Windows the binary is named ws.exe.
  - If the binary is a symlink it is resolved before replacement.
`,
}

var agentGuide = `AGENT WORKFLOWS
===============

This section is for AI agents and automated workflows using ws.
If you are a human, you may find it useful too. If you are an agent,
these are the canonical patterns for safe, composable work.

  PATTERN 1: FRESH START (isolation)
  ───────────────────────────────────
  Purpose: Start from a guaranteed-clean base, unaffected by other work.
  Flow:
    ws get base:<repo> --name=<task>
    ws run <task> -- <command>
    ws keep <task> --message="<what was done>"
    ws drop <task>          # if done, or keep it alive for continuation

  PATTERN 2: BUILD ON ANOTHER AGENT'S WORK (collaboration)
  ──────────────────────────────────────────────────────────
  Purpose: Pick up where another agent left off, even with uncommitted work.
  Flow:
    ws get ws:<source-workspace> --name=<task>
    ws run <task> -- <command>
    ws keep <task> --message="<incremental work>"
    # Communicate the new layer hash to downstream agents

  PATTERN 3: CHECKPOINT (safety)
  ──────────────────────────────
  Purpose: Create a safe rollback point during long tasks.
  Flow:
    ws get base:<repo> --name=<task>
    ws run <task> -- <command-1>
    ws keep <task> --message="after step 1"
    ws run <task> -- <command-2>
    ws keep <task> --message="after step 2"
    # If step 3 fails, you can create a fresh workspace from checkpoint
    ws get layer:<step-2-hash> --name=<task-recovered>

  PATTERN 4: PARALLEL TASKS (independent workstreams)
  ──────────────────────────────────────────────────
  Purpose: Run multiple tasks from the same base in parallel.
  Flow:
    ws get base:<repo> --name=task-a
    ws get base:<repo> --name=task-b
    ws run task-a -- <work-a> &
    ws run task-b -- <work-b> &
    ws keep task-a --message="result A"
    ws keep task-b --message="result B"
    # Both outputs exist as independent layers, merge policy applies

  PATTERN 5: MERGE (conflict resolution)
  ──────────────────────────────────────
  Purpose: Combine work from two workspaces that touched the same files.
  Flow:
    ws get layer:<base> --name=merge
    ws run merge -- <install merge tool: npm, go, etc.>
    ws run merge -- <merge-tool> $(ws layer path <hash-a>) $(ws layer path <hash-b>)
    # Or, more practically, copy both into merge workspace and resolve manually
    # Then: ws keep merge --message="merged A and B"

  PATTERN 6: CLEANUP (hygiene)
  ────────────────────────────
  Purpose: Free disk space after tasks complete.
  Flow:
    ws drop <task-a>
    ws drop <task-b>
    ws layer gc             # removes all unreferenced layers

  RULES FOR AGENTS
  ────────────────
  1. ALWAYS keep before communicating results. Your workspace is ephemeral.
     The layer is permanent and addressable.
  2. NEVER drop another agent's workspace. Only your own. Coordination is
     via layer hashes, not workspace names.
  3. PREFER base: over ws: when you don't need the other agent's work.
     Isolation is safer than shared state.
  4. SMALL units of work. Keep frequently. A layer is cheap. A dropped
     workspace with hours of work is expensive.
  5. CHECK before running. 'ws diff <workspace>' before keep shows exactly
     what will be committed.
  6. NAME meaningfully. 'bug-1234-fix' is better than 'workspace-7'.

  ENVIRONMENT REQUIREMENTS
  ────────────────────────
  - Linux: fuse-overlayfs must be installed (apt install fuse-overlayfs).
  - macOS: No additional deps for copy backend.
           If container (github.com/apple/container) is installed, ws uses
           lightweight Linux VMs automatically. Ensure 'container system start'.
`

var conceptsGuide = `CONCEPTS
========

Layer
  An immutable, content-addressed directory stored in ~/.ws/layers/<hash>.
  Once created, a layer never changes. It is the unit of sharing and
  dependency in the ws graph.

Workspace
  A mutable working directory named by the user. Created from a layer
  (its "formed_from"). Can be modified, committed (keep), or destroyed
  (drop).

Graph
  The directed acyclic graph of layers connected by parent relationships.
  Workspaces are leaves in this graph — mutable views on top of immutable
  layers.

Fork vs Branch
  - Fork (get layer:...) creates a NEW mutable view from an immutable base.
    The workspace shares the lower layer with no other workspace.
  - Branch (get ws:...) creates a NEW mutable view from another workspace's
    current state. It snapshots the source workspace into a new layer first,
    then forks from that snapshot.

Content Addressing
  Each layer's hash is a SHA-256 of its file contents (file paths + file data).
  Identical content always produces the same hash, regardless of when or
  where it was created. This makes layers deterministic and cachable.

Merge Semantics
  ws does NOT implement merge. Overlayfs itself does not merge — it overlays.
  True three-way merge (with conflict resolution) is a policy decision
  implemented in a workspace using standard tools (git merge-file, diff3, etc.)
  followed by 'ws keep'.

Garbage Collection
  Layers are reference-counted transitively: a layer is referenced if it is
  the formed_from of any workspace, or if it is an ancestor of a referenced
  layer. Unreferenced layers can be removed with 'ws layer gc'.

Platform Differences
  - Linux: overlayfs mounts provide copy-on-write isolation.
  - macOS: directory copies provide isolation.
  - Both expose the exact same CLI and graph model.
`


func printHelp(topic string) {
	if topic == "" || topic == "help" {
		fmt.Println(usageText)
		return
	}
	if text, ok := cmdHelps[topic]; ok {
		fmt.Println(text)
		return
	}
	switch topic {
	case "agent":
		fmt.Println(agentGuide)
	case "concepts":
		fmt.Println(conceptsGuide)
	default:
		fmt.Fprintf(os.Stderr, "unknown help topic: %q\n\n", topic)
		fmt.Println(usageText)
		os.Exit(1)
	}
}

func printVersion() {
	fmt.Printf("ws %s\n", version)
}

const usageText = `ws — workspace graph CLI

Managing mutable workspaces on immutable, content-addressed layers.
Linux: overlayfs (COW). macOS: directory copies.

USAGE
  ws <command> [options]

COMMANDS
  get    <source> --name=<ws>     Create a workspace from a layer, ws, or repo
  run    <ws> -- <cmd> [args...]  Execute a command inside a workspace
  diff   <ws> [target]           Diff workspace vs base, vs ws, or vs layer
  keep   <ws> [--message=...]    Promote workspace changes to a new layer
  drop   <ws>                    Destroy workspace (changes lost unless kept)
  graph  [ws]                    Print layer/workspace dependency graph
  layer   <ls|show|gc>           Manage the immutable layer store
  skill                           Install the ws-workspace-graph agent skill
  update                          Update ws to the latest release
  help    [topic]                 Print help for a command or topic

QUICK START
  ws get base:/path/to/repo --name=mytask
  ws run mytask -- cat package.json
  ws keep mytask --message="initial state"
  ws drop mytask

HELP TOPICS
  ws help get        Detailed help for 'get'
  ws help run        Detailed help for 'run'
  ws help layer      Detailed help for 'layer'
  ws help update     Detailed help for 'update'
  ws help skill      Detailed help for 'skill'
  ws help agent      Agent workflows and best practices
  ws help concepts   Key concepts and terminology

For full documentation: https://github.com/weslien/ws
`
