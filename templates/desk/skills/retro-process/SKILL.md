---
name: retro-process
description: Session retrospective following desk/retro.md [feedback, session review, improvement, workflow, validation]
license: MIT
compatibility: opencode
metadata:
  source: desk/retro.md
  related:
    - skill:workflow-protocol
    - skill:task-closing
---

## What I do

I run session retrospectives following the process in **desk/retro.md**.

## Quick Reference

**Full process**: Read `desk/retro.md` for complete workflow, tables, and checklists.

**Key steps:**
1. Analyze the session (what went well/wrong)
2. Ask user for perspective
3. **Priority 1:** Update AGENTS.md (Workflow + Project Memory sections)
4. **Priority 2:** Create/update skills (multi-step workflows only)
5. **Priority 3:** Update project docs (ARCHITECTURE.md, DEVELOPMENT.md, etc.)
6. Remind user of actions (make setup, restart)

## Critical Rules

**DO:**
- Follow `desk/retro.md` step-by-step
- Present proposals clearly
- Wait for explicit user approval
- Let user manage commits and cleanup
- Remind user to restart session after AGENTS.md/skills changes (see "After Writing Proposals")

**DO NOT:**
- Commit any changes

## Skills Locations

| Type | Write to | Review |
|------|----------|--------|
| Workflow (universal) | `desk/skills/` | `git diff` in workspace |
| Technical (project) | `.agents/skills/` in project repo | `git diff` in project |

**Note:** System skills are in `desk/skills/`. Edit these directly during retro (in-place).

**Full details**: Read `desk/retro.md` section "Identify Skills Opportunities".

## After Writing Proposals

**Remind user (priority order):**
1. `git diff` AGENTS.md → commit → propagate
   - **opencode:** `make setup` then `make docker-restart-agent`
     - ⚠️ **Restart required** — opencode loads AGENTS.md only at startup
   - **pi.dev:** `make setup` is sufficient
     - ✅ **No restart needed** — reads at each session start
2. `git diff` skills → commit → restart (only if skills created/updated)
   - **opencode:** `make docker-restart-agent` or exit + `make opencode`
     - ⚠️ **Critical for Web UI** - skills NOT discovered without restart
   - **pi.dev:** End session (`/quit`) and start new `pi` session
     - 💡 **Workaround:** Tell agent to `ls desk/skills/` to discover without restart
3. `git diff` project docs → commit if relevant

**Full details**: Read `desk/retro.md` section "Propose Updates".
