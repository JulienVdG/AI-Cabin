---
name: validation-checkpoint
description: Rules for when to ask user validation (1 action = 1 validation)
license: MIT
compatibility: opencode
metadata:
  related: desk/TODO.md
---

## What I do

I enforce the "1 action = 1 validation" rule to ensure traceability and avoid assumptions.

## Core Principle

**After EACH significant action, pause and ask for validation:**

```
✅ [action] completed. Continue or validate?
```

## When to Ask Validation (ALWAYS)

**Require explicit approval for:**

- ✅ File modifications (write, edit, delete)
- ✅ Git operations (add, commit, push)
- ✅ Running tests that might fail
- ✅ Making decisions about implementation approach
- ✅ Completing a task or sub-task
- ✅ **Test failures or unexpected results** (ask for guidance, don't modify tests alone)

## Git Commits (CRITICAL)

**Each commit requires separate, explicit approval. Authorization does NOT carry forward.**

**Pattern:**
```
Complete task → Present summary → Ask "Je commit ?" → Wait for "OK" → Commit
Complete next task → Ask "Je commit ?" → Wait for "OK" → Commit
```

**❌ WRONG:**
- "User said OK to commit docs, so I'll commit cleanup too"
- "This is just a small fix, no need to ask"
- "User trusts me for documentation commits"

**✅ RIGHT:**
- Ask before EVERY commit, even for:
  - Documentation
  - Small fixes
  - Cleanup
  - Follow-up to previous commit

**Example:**
```
AI: "J'ai créé 7 docs d'architecture. Je commit ?"
User: "OK"
AI: [commits the 7 docs]

AI: "J'ai nettoyé les références Greywall. Je commit ?"
User: "OK"
AI: [commits the cleanup]

AI: "J'ai ajouté les limitations pi-web-ui. Je commit ?"
User: "OK"
AI: [commits the limitations]
```

**Authorization is NOT transferable:**
- "OK to commit" → applies ONLY to what was just reviewed
- Does NOT authorize future commits
- Does NOT authorize related files unless explicitly mentioned

## When NO Validation Needed

**Can proceed without asking:**

- ✅ Reading files
- ✅ Searching code (grep, glob)
- ✅ Running informational commands (ls, git status)
- ✅ Answering factual questions

## Mode Awareness

**Plan vs Build (User controls via UI dropdown):**

- ✅ **Plan mode** (read-only):
  - I can ONLY read/inspect
  - I must ASK: "Peux-tu activer le mode Build ?"
  - Edit commands are SYSTEM-BLOCKED (not just politeness)

- ✅ **Build mode** (more permissive):
  - I can read AND write
  - I can still choose to only read (Plan-like behavior)

**Auto-detect Plan-like situations (even in Build mode):**

Even when user is in Build mode, switch to Plan-like behavior if:
- Request is vague or imprecise ("make it better")
- Large scope ("refactor everything")
- Modifies tested code ("change test structure")
- Architectural decision ("should we use X or Y?")

**Response:**
- ❌ Do NOT code immediately
- ✅ Ask clarifying questions
- ✅ Present options with tradeoffs
- ✅ Confirm understanding before proceeding

**Correct phrasing:**
- ❌ "Je passe en Build mode" (AI does NOT control UI dropdown)
- ✅ "Peux-tu activer le mode Build ?" (user controls the switch)
- ✅ "J'ai bien compris [X]. Mais si tu n'actives pas le mode Build, je ne peux pas écrire."
- ✅ "Ta demande semble large/imprécise. Veux-tu que je te pose des questions avant de coder ?"

## Acceptable vs Unacceptable Patterns

### ✅ Acceptable Pattern

```
Write file → Run test → Ask validation → Commit
```

(Testing is part of the same logical action - write→test→validate)

### ❌ Unacceptable Pattern

```
Write file → Run test (fail) → Write fix → Run test (fail) → Write fix → ...
```

(Iterating alone on failures without asking for guidance)

### ✅ Good Response to Test Failure

```
Test failed. Expected X but got Y.
Should I:
1. Debug more?
2. Change approach?
3. Accept this failure?
```

## Validation Formula

After each significant action:

```markdown
✅ [Action name] completed.

[Optional: brief context if needed]

Continue or validate?
```

## Examples

### After file modification

```
✅ File modified: src/converter.go

Review in your editor. Should I commit?
```

**Important:** Always explicitly ask "Peux-tu review mes changements ?" after file modifications. Do NOT assume user saw the edit in their editor.

**User review method:** User will use `git diff` or `meld` to review changes. This is why AI must NOT commit before explicit approval.

### After test failure

```
❌ Test failed: TestConverter/edge_case

Expected: "output value"
Got: "output"

Should I:
1. Debug the Process function?
2. Change the test expectation?
3. Accept this as known issue?
```

### After completing sub-task

```
✅ Sub-task complete: Added validation layer

Files modified:
- src/validator.go
- src/validator_test.go

Continue with integration tests or validate first?
```

### After running tests

```
✅ Tests passed: 23/23 passing

No regressions detected. Should I proceed to commit?
```

## Special Cases

### Mode Mismatch Detection

If user requests implementation but mode is Plan:

```
J'ai bien compris [X]. Mais si tu n'actives pas le mode Build, je ne peux pas dérouler le plan.
```

Wait for user to switch modes (they typically reply "go" or similar).

### Long Explanations in Plan Mode

Before long explanations:

```
Le plan est-il clair ou veux-tu des précisions avant que je commence ?
```

### Unclear User Intent

When user explanations seem unclear:

```
Should I:
1. Return to Plan mode for clarification?
2. Proceed with my understanding [summary]?
```

## Related Skills

- `todo-workflow` - Full 7-step task protocol
- `design-protocol` - Design validation before coding
