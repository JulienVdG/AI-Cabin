# AI Agent Instructions

## General Principles

- Be direct and go straight to the point.
- Wait for explicit user questions before performing complex tasks.
- Maintain consistency across responses: do not change file names, variable names, or structures without an explicit technical reason.
- Prioritize code stability from one iteration to another.
- Start of session: read project documentation (README.md, ARCHITECTURE.md, desk/*.md) to understand the project architecture and technical decisions.
- Before starting any task: read `desk/TODO.md` to understand current task status and context, and check relevant skills.

## Communication

- Interaction language: follow the user's language.
- Technical language (code, commits, documentation): English.
- When in doubt: always choose English for technical content.

## Coding Standards

- All code, configuration files, and technical documentation must be in English.
- Comments within the code must start with a capital letter and end with a period.
- Code and comments must not reference internal specs. Describe the decision inline instead.
- Run the formatter after each coding step, then before tests.

## Workspace Constraints

- All files must be in the workspace directories (e.g., `src/`, `desk/`).
- The user must have read access for review at all times.
- Never use `/tmp/` or directories outside the workspace.

## Git Protocol

- Use the Semantic Commit format for all commit messages (e.g., `feat: add database migration for users`).
- Commit messages must not end with a period.
- One approval = one commit: each change requires explicit user approval before committing.
- Never commit without explicit user approval ("OK" or "validated").

## Workflow

- Plan before modifying: read the relevant files and propose a direction, then wait for validation before implementing.
- Validate with tests after implementing, and pause for review before committing.
- One change = one validation: after each significant action, pause and ask for direction.
- If a test fails, stop and ask for guidance. Do not iterate alone on failures.
- Track progress in `desk/TODO.md` (mark a task in progress when starting, done when complete).

## Learning and Gotchas

Append lessons learned and gotchas here as they are discovered during sessions.
One short entry per lesson: the root cause and the solution.

| Lesson | Root cause | Solution |
|--------|------------|----------|
|        |            |          |
