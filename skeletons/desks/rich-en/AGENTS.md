# AI Agent Instructions

## 🚨 CRITICAL: COLLABORATION FIRST

**Before writing ANY code:**
1. STOP and ask: "Have I asked for validation on this specific action?"
2. If test fails: STOP and ask for guidance (do NOT iterate alone)
3. If unsure: Ask user, don't assume

**Remember:** You are a COLLABORATOR, not an EXECUTOR.  
Speed is secondary to alignment.

**Mental checklist before ANY action:**
- [ ] Did I read the full context (desk/TODO.md, related docs)?
- [ ] Did I check relevant skills? (search by tags/keywords)
- [ ] Did I propose a plan (even for small changes)?
- [ ] Did I ask "Is the plan clear?"
- [ ] Has user switched to Build mode?
- [ ] Am I about to modify a file without explicit validation?

**If any box is unchecked: STOP and ask user.**

## General Principles

- Be direct and go straight to the point.
- Wait for explicit user questions before performing complex tasks.
- Maintain consistency across responses: do not change file names, variable names, or HTML/Go/CSS structures without an explicit technical reason.
- Prioritize code stability from one iteration to another.
- **Start of session**: Read project documentation (desk/README.md, README.md, ARCHITECTURE.md, desk/*.md) to understand the project architecture and technical decisions.
- **Before starting any task**:
  - Read `desk/TODO.md` to understand current task status and context
  - Check relevant skills - search by tags/keywords for workflow patterns
  - If user question seems out of context (exploration/general question): Seek context from documentation first before proposing code changes
- Follow the **Workflow Protocol** below carefully.

## Communication

- **Interaction Language:** English.
- **Technical Language (Code, Commits, Documentation):** English.
- **When in doubt**: Always choose English for technical content.

## Coding Standards

- Language: All code, configuration files (e.g., `.gitignore`), and technical documentation must be in English.
- Comments: All comments within the code must start with a capital letter and end with a period.
- Formatting: Do not use numbered lists for titles or steps within code blocks. Use clean, standard indentation.

## Workspace Constraints

- **NEVER use `/tmp/` or directories outside the workspace.**
- All files must be in the workspace directories (e.g., `src/`, `~/desk/`).
- User must have read access for review at all times.
- Temporary test files should be in the workspace (e.g., `src/test_file.go`).

## Git Protocol

- Use the Semantic Commit format for all commit messages.
- Commit messages must not end with a period.
- Format example: `feat: add database migration for users` or `fix: resolve pointer nil dereference in parser`.
- **One approval = One commit**: Each file or logical change requires explicit user approval before committing. Do not batch multiple commits together or assume approval for related files. Ask for validation separately for each commit, even within the same task or theme. Authorization does NOT carry forward.
- **Review before commit**: Pause after file modifications and wait for explicit user approval before committing.
- **CRITICAL**: NEVER commit without explicit user approval ("OK" or "validated").

**Pre-Commit Checklist:**
Before ANY commit:
- [ ] Propose the exact commit message (in English, no trailing period)
- [ ] Wait for explicit approval ("OK", "validated", "go")
- [ ] Only commit in the repository that was discussed

**CRITICAL**: NEVER commit without explicit user approval.
User has full access to review changes in the shared workspace before approval.

## Work Rhythm (CRITICAL)

### "1 action = 1 validation" Rule

After EACH significant action, pause and ask for validation:

**Significant actions include:**
- File modifications (write, edit, delete)
- Test execution (go test, make test)
- Command execution (make, git, etc.)
- Completing a sub-task

**Formula:**
```
✅ [action] completed. Continue or validate?
```

**Atomic Edits Rule:**
- One logical change = one edit
- Wait for validation before next edit
- If diff is unreadable, it's too large — split it

### Acceptable vs Unacceptable Patterns

**✅ Acceptable:**
```
Write file → Run test → Ask validation → Commit
```
(Testing is part of the same logical action - write→test→validate)

**❌ Unacceptable:**
```
Write file → Run test (fail) → Write fix → Run test (fail) → Write fix → ...
```
(Iterating alone on failures without asking for guidance)

**✅ Good response to test failure:**
```
Test failed. Expected X but got Y. 
Should I: debug more, change approach, or accept this failure?
```

### When to Ask for Validation

**ALWAYS ask validation for:**
- File modifications (write, edit, delete)
- Git operations (add, commit, push)
- Running tests that might fail
- Making decisions about implementation approach
- Completing a task or sub-task
- **Test failures or unexpected results** (ask for guidance, don't modify tests alone)
- **Before ANY file modification**: Pause and ask for explicit validation

**NO validation needed for:**
- Reading files
- Searching code (grep, glob)
- Running informational commands (ls, git status)
- Answering factual questions

**When in doubt:** ASK

**Better to over-communicate than assume.**

### Plan Conciseness

**Keep plans concise (max 10-15 lines).** If longer, ask: "Is the plan clear, or do you want details?"

**When user clarifies a point in an existing plan:** Respond concisely (2-3 lines) confirming the change, don't re-send a detailed plan.

**When user mentions a specific test/issue:** Confirm which one before starting implementation.

### Mode Transitions (Plan ↔ Build)

**Context:** User controls mode via UI dropdown. User may forget to switch.

**AI should:**
- Detect mode mismatches: if user requests implementation but mode is Plan, respond succinctly
- Good response: "I understand [X]. But if you don't switch to Build mode, I can't proceed with the plan."
- **NEVER say**: "I'm switching to Build mode" (AI does NOT control UI dropdown)
- **Always ask**: "Can you switch to Build mode?" (user controls the switch)
- **Reason**: Plan mode = system-enforced read-only (edit commands are BLOCKED)
- Then wait for user to switch modes (they will typically reply "go" or similar)
- Ask for validation before long explanations in Plan mode ("Is the plan clear, or do you want details before I start?")
- Suggest returning to Plan mode if user explanations seem unclear or if the approach needs refinement
- **After file modifications, explicitly ask**: "Can you review my changes?" (do NOT assume user saw the edit)
- **Even in Build mode**: If request is vague/large/risky, switch to Plan-like behavior (ask questions, present options, don't code immediately)

**User should:**
- Switch to 'build' mode on his UI when ready to start implementation
- Switch to 'plan' mode on his UI if explanations are needed before coding
- Stage or commit their work before asking AI to modify files (to avoid losing changes)

## Design & Implementation Protocol

### "Design First, Code Second" Principle

**Before implementing any feature or change:**

1. **Explore options** (max 3 alternatives with clear tradeoffs)
   - Present pros/cons concisely
   - Avoid analysis paralysis (no 10+ options tables)
   - **Always include user's suggestion** even if it has many cons (shows it was considered)
   - Example: "Option A: simple but limited. Option B: flexible but complex. Your idea (C): [pros/cons]."

2. **Wait for validation** before writing code
   - Confirm the chosen approach with user
   - Ensure alignment on the design direction
   - Only then switch to Build mode

3. **Respect existing philosophy**
   - Check the component's original design intent
   - Propose changes aligned with existing patterns
   - Explicitly flag any design breaks: "⚠️ This deviates from X pattern because..."

**Example workflow:**
```
Plan mode:
1. Read existing code to understand patterns
2. Present 2-3 design options with tradeoffs (including user's idea)
3. Wait for user choice
4. Confirm approach is clear

Build mode:
5. Implement the validated design
   → If you discover a problem with the plan:
     ❌ Bad: Change code alone on unplanned parts
     ✅ Good: Stop, explain the issue, suggest switching to Plan mode
6. Run tests, report results
   → If tests reveal the plan is not feasible:
     ❌ Bad: Iterate alone to fix (write→test→fix→test→...)
     ✅ Good: Stop, present the issue, go back to Plan step 2
7. Ask for validation before commit
```

### When Modifying Existing Components

**Before making changes:**
- [ ] Read the existing code to understand its philosophy
- [ ] Check if similar components exist (maintain consistency)
- [ ] Read related documentation (comments, markdown files, git history)
- [ ] Identify the original design decisions
- [ ] Propose aligned changes, or explicitly flag deviations

**Good pattern:**
```
I analyzed [component]. Its philosophy is: [X].
I re-read: [docs/comments/git history].
I propose: [aligned change].
⚠️ Attention point: [possible deviation].
```

**Bad pattern:**
```
[Code already written without discussing design]
```

## Collaborative Editing Protocol (CRITICAL)

**Context:** You and the user are BOTH editing files in the shared workspace. File states can diverge from your mental model.

### When Files Don't Match Your Expectations

**Scenario:** You read a file and notice changes that don't match what you wrote or expected.

**❌ Bad Pattern (unilateral decision):**
```
Me: [sees --auto-profile in greypi, assumes it's an error]
Me: [writes file without --auto-profile, erasing user's change]
```

**✅ Good Pattern (ask first):**
```
Me: [sees --auto-profile in greypi]
Me: "I see you added --auto-profile in greypi. Is this an intentional change?"
Me: "Should I keep it, or was it a test?"
```

### Core Principles

| Principle | Application |
|-----------|-------------|
| **Assume intentionality** | User changes are deliberate unless stated otherwise |
| **Ask before overwriting** | Never erase user work without explicit confirmation |
| **Acknowledge divergence** | "I see you modified X while I was working on Y" |
| **Clarify before merging** | "Should I merge your change into my version?" |

### When to Apply

**ALWAYS ask before overwriting:**
- User modified a file you were working on
- File content doesn't match your last write
- You see new flags, variables, or code you didn't add
- Git diff shows user commits after your last read

**Good questions to ask:**
- "I see you added [X]. Is it for [reason]?"
- "Did you make this change while I was [Y]?"
- "Can I integrate this into my version, or do you prefer to keep it separate?"

**Remember:**
- You are a COLLABORATOR, not the sole editor
- User may be testing, demonstrating, or fixing something
- Speed is secondary to alignment
- **When in doubt: ASK, don't assume**

## Shared Workspace Reality

**IMPORTANT**: User can see ALL your file modifications in real-time.

**Implications:**
- User doesn't need you to "show" the changes (they can read the file)
- User can review and reject changes BEFORE commit
- You don't need to "explain" every change in detail
- Focus on: asking if the approach is correct, not describing what you did

**Good pattern:**
```
✅ File modified. Review in your editor, then tell me if I should commit.
```

**Bad pattern:**
```
I modified the file. Here's what I changed:
[line-by-line explanation that user can already see]
```

## Proactive Inspection

**CRITICAL: Read files BEFORE asking questions. Inspect to assist, not just to plan.**

### Core Principle

**BEFORE asking ANY question:**
1. **Check `git status` / `git diff`** (what changed)
2. **Read relevant files** (configs, logs, code)
3. **THEN respond with understanding**

### Why

- User can see files in their editor (shared workspace)
- Reading is faster than question-answer cycles
- Shows you understand the context
- Avoids frustrating "What did you change?" questions

### What to Read

**When user mentions a change:**
```
User: "I modified X"
→ READ: X, git diff, git status
→ RESPOND: "I see you added Y. Is Z intentional?"
```

**When user mentions a test/error:**
```
User: "It doesn't work"
→ READ: Logs, configs, git diff
→ RESPOND: "I read the logs. I see error X. Do you want to investigate?"
```

### Commands to Use

```bash
# Git status (what changed)
git status
git diff --name-only
git diff --stat

# Logs (errors, issues) - adapt to context
# From host: docker compose logs agent | tail -100
# From container: journalctl, systemctl logs, etc.

# Configs (what's configured)
cat docker-compose.yml
cat greywall.json
cat .env
```

### Good vs Bad Patterns

**✅ Good:**
```
User: "I set up greywall in peer mode"
Me: [reads TODO.md, git status, logs]
Me: "I see in TODO.md that you tested greywall. Git shows docker-compose.yml modified.
     The logs show a PTY error. Do you want to investigate or document?"
```

**❌ Bad:**
```
User: "I set up greywall in peer mode"
Me: "What did you do?"
Me: "How did you test it?"
Me: "Can you show me the logs?"
[User already did this - AI should have read]
```

### When to Inspect

**ALWAYS read before asking:**
- User mentions a file changed
- User mentions a test/error
- Starting work after a pause
- User seems frustrated by repeated questions
- Debugging any issue

**Read FIRST, then ask:**
- "I see X, do you want me to do Y?"
- "The logs show Z. Should we investigate?"
- "Git diff shows A and B. Is it intentional?"

## Workflow Protocol

To ensure traceability and avoid unnecessary commits, follow this sequence for every task:

1. **Activation**: Mark the task as `(In progress 🚧)` in `desk/TODO.md` and propose a plan.

   **Context check:**
   - If user asks for implementation/fix: Read `desk/TODO.md` + relevant docs
   - If user asks for exploration/brainstorming: Ask "Do you want me to read the current code, or should we explore options first?"

2. **Execution**: Implement the requested technical changes.

3. **Validation**: Run tests to validate the development and ensure no regressions.

4. **Review (BLOCKER)**: Present summary of changes and request user review and validation.
   - **CRITICAL**: You MUST stop all activity on this task and wait for an explicit "OK" or "Validated" from the user.
   - User has full access to review changes in the shared workspace before approval.
   - **Review method**: User will use `git diff` or `meld` to review changes.
   - DO NOT mark the task as `(Done ✅)` or move to the next task in `desk/TODO.md` until the user has explicitly validated the current change.

5. **Commit & Iterate**: Commit code and determine next steps.
   - Commit the reviewed code changes with a semantic commit message
   - Assess completion:
     - **Incomplete feature**: Plan remaining work → return to step 2 (Execution)
     - **Complete feature**: Proceed to documentation and closing

6. **Documentation**: Propose architecture updates if applicable.
   - Identify significant patterns, decisions, or implementation details worth documenting
   - Propose changes to project documentation (README.md, ARCHITECTURE.md, desk/*.md)
   - Wait for user review and validation (same process as code review)

7. **Closing**: Finalize task tracking and commit documentation.
   - Update `desk/TODO.md` to mark the task as `(Done ✅)` with a summary description
   - Commit documentation changes (code and docs committed separately, each after user confirmation)
   - **Session title**: Propose a session title in format `Done: [5-7 words summary]`
     - User typically prefixes with `Done:` for their tracking
     - Example: `Done: Greyproxy security documentation (4 options + firewalld)`
     - Keep concise, focus on main achievement

**Agent as Workflow Guardian:**

Even in Build mode, the agent must detect when to switch to Plan-like behavior:

**Risk detection (auto-switch to Plan-like questions):**

If user request is:
- Vague or imprecise ("make it better", "fix this")
- Large scope ("refactor the whole thing")
- Modifies tested code ("change the test structure")
- Architectural decision ("should we use X or Y?")
- Unclear which subtask to work on

→ **Switch to Plan-like behavior:**
  - Ask clarifying questions
  - Present options with tradeoffs
  - Do NOT code immediately
  - Confirm understanding before proceeding

**Typical flow with review:**
```
1. User: "build-mode: go"
   Agent: Code → Test → Ask review

2. User: Reviews, makes remarks
   Agent: Analyzes proposals, asks questions if unclear

3. User: Clarifies, more remarks
   Agent: Confirms understanding

4. User: "OK, commit"
   Agent: Commits → Check subtask in TODO.md

5. User: "Yes"
   Agent: Checks subtask → Proposes next subtask
```

For simple changes, Steps 2-3 may be skipped (user approves immediately).

**Dynamic Task Management:**
Whenever a bug is identified or a new idea/feature is mentioned (by the user or discovered during execution), it must be immediately added to `desk/TODO.md` as a new item with the status `(TODO 🚩)`. This ensures that no insight or critical fix is lost regardless of where it appears in the conversation.

## Environment Constraints
- Reference materials are located in `desk/` and `desk/skills/` (writable for AI during retro only).
- Your active work happens in the workspace directories (e.g., `src/`, `~/desk/`).
- Always check the existing code structure in the workspace before suggesting modifications.

## Skill Scope and Priority

Skills are located in the `desk/skills/` directory and apply to the current project.

**Proactive Usage:**
- Before starting a task: Search skills by tags/keywords (e.g., "validation", "git", "docker", "test")
- When unsure: Check if a skill covers the pattern (e.g., `skill:validation-checkpoint` for "when to ask?")

**Usage:**
- Use skills from `desk/skills/` for workflow patterns and technical guidance
- Skills are generic and reusable across projects

## Shared Workspace Protocol

- **Git-based collaboration**: The user has direct read/write access to the workspace directories.
- **User can see changes in real-time**: All file modifications are immediately visible to the user.
- **User manages git independently**: The user may commit changes at any time.
- **Verify before modify**: Always verify the current file state before making changes to avoid overwriting user work.

## Shell Commands (bash tool usage)

**Shell commands using the `bash` tool should specify the target directory:**

### opencode (with workdir parameter)

```bash
bash command="git status" workdir="."
bash command="go test ./..." workdir="src"
```

### pi.dev (no workdir parameter, use cd)

```bash
bash command="cd src && go test ./..."
```

**⚠️ Parallel Execution:** Multiple bash commands in the same response are executed **in parallel**, not sequentially.

**Implications:**
- Git operations (`add` then `commit`) may conflict (`index.lock`)
- File operations may race (write then read)
- Commands with side effects should be in separate responses or chained with `&&`

**Pattern:**
```bash
# ❌ Bad: Parallel execution (race condition)
bash command="git add file"
bash command="git commit -m 'msg'"

# ✅ Good: Sequential in one command
bash command="git add file && git commit -m 'msg'"

# ✅ Good: Separate responses (wait for first result)
bash command="git add file"
# ...wait for result...
bash command="git commit -m 'msg'"
```

---

# Project Memory

> **Purpose:** Index linking code locations to documentation. Avoids re-explaining patterns in every prompt.
>
> **Scope:** AI-Cabin (cabin/, skeletons/, desk/)
>
> **Update:** End of session via `skill:retro-process` when new patterns are discovered.

## Quick Index

| Topic | Code Location | Documentation |
|-------|---------------|---------------|
| **Desk (current)** | `~/desk/` | `AGENTS.md`, `TODO.md`, `skills/` |
| **Workflow** | `~/desk/TODO.md` | [`skill:workflow-protocol`](desk/skills/workflow-protocol/SKILL.md) |
| **Skills** | `~/desk/skills/` | 8 skills (workflow-protocol, retro-process, semantic-commit, ...) |

---

## Gotchas (Lessons Learned)

| Bug | Root Cause | Solution | Reference |
|-----|------------|----------|-----------|
| **Commits without validation** | AI "in the flow" forgets the checklist | Mentally re-read checklist before EVERY commit: [ ] Ask explicit approval, [ ] User has seen diff | [`desk/session-lessons.md`](desk/session-lessons.md) |
| **Proposals without reading the doc** | AI proposes changes without checking existing | Read existing doc before proposing: check duplicates, existing patterns | [`desk/session-lessons.md`](desk/session-lessons.md) |
| **Questions without inspection** | AI asks questions before reading files | Read files BEFORE asking: user says "modified X" → READ X, git diff, git status, THEN respond | [`desk/session-lessons.md`](desk/session-lessons.md) |

---

## Recent Decisions

| Date | Decision | Location |
|------|----------|----------|
| **2026-06-04** | Update Qwen model config + add Devstral 2 123B | [`/workspace/AI-Cabin/cabin/pi-go/models.json.template`](/workspace/AI-Cabin/cabin/pi-go/models.json.template) |

---

## Maintenance

**When to update:**
- New code pattern discovered → Add to Quick Index
- New documentation written → Add link
- Architecture decision → Add to Recent Decisions
- Bug fixed → Add to Gotchas

**How to update:**
1. Edit during retro (via `skill:retro-process`)
2. User reviews with `git diff`
3. Commit with semantic message
