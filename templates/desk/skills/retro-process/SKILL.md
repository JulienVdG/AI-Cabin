---
name: retro-process
description: Session retrospective following desk/retro.md [feedback, session review, improvement, workflow, validation]
license: MIT
compatibility: opencode
metadata:
  source: desk/retro.md
  related:
    - todo-workflow
    - validation-checkpoint
    - task-closing
---

## What I do

I run session retrospectives following the process in **desk/retro.md**.

## Quick Reference

**Full process**: Read `desk/retro.md` for complete workflow, tables, and checklists.

**Key steps:**
1. Analyze the session (what went well/wrong)
2. Ask user for perspective
3. Identify skills opportunities
4. Propose updates (AGENT.md, skills, docs)
5. Remind user of actions (make setup, restart cabin)

## Critical Rules

**DO:**
- Follow `desk/retro.md` step-by-step
- Present proposals clearly
- Wait for explicit user approval
- Let user manage commits and cleanup
- Remind user to run `make setup` after AGENT.md changes
- Remind user to restart cabin after skill changes

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

**Remind user:**
1. `git diff` AGENTS.md → commit → run `make setup` in cabin
2. `git diff` skills → commit → restart cabin
3. `git diff` project docs → commit if relevant

**Full details**: Read `desk/retro.md` section "Propose Updates".

## Related Skills

- `todo-workflow` - Full 7-step task protocol
- `validation-checkpoint` - When to ask validation
- `task-closing` - Documentation and closing process
- `design-protocol` - Design options and tradeoffs
