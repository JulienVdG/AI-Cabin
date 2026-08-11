---
name: retro-process
description: Session retrospective following desk/retro.md [feedback, session review, improvement, workflow, validation]
license: MIT
---

## What I do

I run a short session retrospective when the user asks for feedback. The goal is to continuously improve behavior based on actual session experience.

## Quick Reference

**Key steps:**
1. Analyze the session: what went well, what went wrong, what was unclear.
2. Ask the user for their perspective.
3. Propose updates in priority order:
   - `desk/AGENTS.md` (workflow rules, gotchas).
   - `desk/skills/` (new or updated skills for recurring patterns).
   - Project documentation (`README.md`, `ARCHITECTURE.md`, `desk/*.md`).
4. Remind the user to commit and reload the agent after changes (skills and AGENTS.md are loaded at startup).

## Critical Rules

**DO:**
- Present proposals clearly and wait for explicit user approval.
- Let the user manage commits and cleanup.
- Reference docs, do not duplicate content.

**DO NOT:**
- Commit any changes. The user reviews with `git diff` and commits after approval.

## Proposal Format

For each proposed update, state the file, the change, and the rationale (one or two lines). Group proposals by priority. After the user approves, write the proposals in place and remind them to commit and reload the agent session.
