# Session Retro Process

## Purpose

Continuously improve the AI agent's behavior based on actual session experience. Run a short retrospective when the user asks for feedback.

## Process

### Analyze the session

- What went well? (successful patterns, good decisions)
- What went wrong? (violations, misunderstandings, frustrations)
- What was unclear? (ambiguous instructions, missing guidance)
- When did the agent iterate alone instead of asking for validation?

### Ask the user

- "What do you think went wrong in this session?"
- "What should I do differently next time?"
- "What worked well that we should keep?"

### Propose updates (in priority order)

1. `desk/AGENTS.md` — workflow rules, workspace constraints, gotchas.
2. `desk/skills/` — new or updated skills for recurring patterns.
3. Project documentation — `README.md`, `ARCHITECTURE.md`, `desk/*.md`.

For each proposal, state the file, the change, and the rationale (one or two lines). Wait for explicit user approval before writing.

### After approval

- Write the proposals in place (the user reviews with `git diff`).
- Do not commit: the user commits after approval.
- Remind the user to commit and reload the agent session (skills and AGENTS.md are loaded at startup, so a reload is needed for changes to take effect).
