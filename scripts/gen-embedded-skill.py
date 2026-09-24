#!/usr/bin/env python3
"""Generate cmd/ws/skill-embedded.go from assets/skill/ws-workspace-graph/SKILL.md.

Run after editing the SKILL.md:
    python3 scripts/gen-embedded-skill.py
"""

import os
import sys

SKILL_PATH = "assets/skill/ws-workspace-graph/SKILL.md"
OUTPUT_PATH = "cmd/ws/skill-embedded.go"

def main():
    if not os.path.exists(SKILL_PATH):
        print(f"error: {SKILL_PATH} not found", file=sys.stderr)
        sys.exit(1)

    with open(SKILL_PATH) as f:
        content = f.read()

    # Escape backticks for Go raw string: ` becomes ` + "`" + `
    escaped = content.replace("`", "` + \"`\" + `")

    output = f"""package main

// autonomous-ai-agents: Skill for using ws (workspace graph CLI) in agent workflows.
//
// Generated from {SKILL_PATH} — edit the original,
// then regenerate with: python3 scripts/gen-embedded-skill.py

const defaultSkillName = "ws-workspace-graph"

const defaultSkillContent = `{escaped}`
"""

    with open(OUTPUT_PATH, "w") as f:
        f.write(output)

    print(f"written {len(output)} bytes to {OUTPUT_PATH}")

if __name__ == "__main__":
    main()
