---
name: workflow-protocol
description: 7-step workflow protocol with validation checkpoints [validation, workflow, commit, review, TODO.md]
license: MIT
compatibility: opencode
metadata:
  related: desk/AGENTS.md, skill:task-closing, skill:semantic-commit
---

## What I do

I enforce the 7-step workflow protocol for every task to ensure traceability and user alignment.

## When to use me

Use this skill at the start of every new task or feature request.

## Quick Reference

**Full documentation:** `desk/AGENTS.md` ## Workflow Protocol (7 steps)

**Core steps:**
1. **Activation** ← **Start**: Mark `(In progress 🚧)` in TODO.md + propose plan (Plan mode)
2. **Execution**: Implement changes (Build mode)
3. **Validation**: Run tests (`go test ./...`)
4. **Review (BLOCKER)**: Mark `(In review 📋)` in TODO.md + present summary + request user approval
5. **Commit**: Commit with semantic message (after explicit "OK")
6. **Documentation**: Update docs if needed
7. **Closing** ← **End**: Mark `(Done ✅)` + 1 line summary with doc link (after validation & commit)

## 🚨 Non-Negotiable Rules

1. **1 file modified = ask review** → "Review my changes?" (show git diff)
2. **1 commit = 1 explicit approval** → "OK to commit?" (wait for "yes")
3. **Approval is TEMPORAL** → Valid ONLY for immediate next commit, NOT for subsequent ones
4. **Test failed = STOP + ask guidance** → "Expected X, got Y. Fix or record for later?"
5. **Doubt = ask question** (never assume)

## ✅ When to Ask Validation

| Action | Ask | Example |
|--------|-----|---------|
| File write/edit/delete | ✅ Before commit | "Review my changes in X?" |
| Git commit | ✅ Always, ONE at a time | "OK for commit: 'feat: ...'?" |
| After ANY commit | ✅ Before next action | "Continue with next subtask?" |
| Test failure | ✅ Immediately | "Test failed. Fix or record for later?" (never accept) |
| Plan deviation | ✅ Before coding | "This differs from plan. Continue?" |
| TODO.md status change | ✅ Before editing | "I propose to mark this task as done" |

## ❌ Never Do

- Chain 2+ commits without intermediate validation
- Assume "OK commit" applies to future commits (approval is TEMPORAL)
- Iterate on test failures without guidance
- Modify user files without confirmation
- Accept a failing test (fix now or record in TODO.md)
- Mark TODO.md `(Done ✅)` before user reviewed AND approved

## Good/Bad Patterns

### Approval temporal (CRITICAL)

**❌ Bad Pattern - Assuming approval carries forward:**
```
User:  "OK, commit" (approval for code commit)
Agent: Commits code ✅
Agent: Updates TODO.md "(Done ✅)" → Commits TODO.md ❌
                                                    ↑
                                    (assumed "OK commit" applies to TODO.md too!)
```

**Problem:** User approved code commit, NOT TODO.md change. TODO.md commit happened without explicit approval.

**✅ Good Pattern - One approval = One commit:**
```
User:  "OK, commit" (approval for code commit)
Agent: Commits code ✅
Agent: "I propose to mark this task as done in TODO.md"
User:  "OK" (explicit approval for TODO.md change)
Agent: Updates TODO.md → "Can you review?" → User review → Commit TODO.md ✅
```

**Why:** Each commit requires separate, explicit approval. Authorization does NOT carry forward.

**✅ Good Pattern:**
```
Agent: Code changes → "Can you review my changes?" (code review first)
User:  Reviews code → "OK, validated" (explicit approval)
Agent: Commits code
Agent: "I propose to mark this task as done" (announce)
User:  "OK" or "Validated" (explicit approval)
Agent: Update TODO.md "(Done ✅)" → "Can you review?" → User review → Commit TODO.md
```

**Why:** TODO.md is like any other file: modification → review → commit. Status `(Done ✅)` means "validated AND committed", not "I finished coding".

**Rule:** Before updating TODO.md (even just checking a box):
1. **Agent announces**: "I propose to mark this task as done"
2. **User approves**: "OK" or "Validated" (explicit)
3. **Agent edits + commits**: TODO.md change → review → commit

### TODO.md Status Honesty (CRITICAL)

**❌ Dishonest Pattern - Marking task done before completion:**
```
Agent: Implements feature → Updates TODO.md subtask "[x] Done" → "Review my code?"
                                                                  ↑
                                                  (Task marked done BEFORE review!)
```

**Problem:** This is dishonest. The task is NOT done - it's still pending review, approval, and commit. Marking it as done creates false progress.

**✅ Honest Pattern - Status reflects reality:**
```
Agent: Implements feature → "Review my code?" → User approves → Commits
Agent: "I propose to mark the subtask as done" → User approves → Update TODO.md "[x]"
```

**Why:** TODO.md status must reflect the **actual state**, not the **intended state**.

**Status meanings:**
| Status | Meaning |
|--------|---------|
| `[ ]` (TODO 🚩) | Task not started |
| `[ ]` (In progress 🚧) | Work in progress (steps 1-3) |
| `[ ]` (In review 📋) | Code done, tests passing, awaiting user review (step 4) |
| `[x]` (Done ✅) | **Validated by user AND committed** (steps 5-7) |

**Rule:** A subtask is `[x]` ONLY after:
1. ✅ Code written
2. ✅ Tests passing
3. ✅ User reviewed (git diff)
4. ✅ User approved (explicit "OK")
5. ✅ Code committed

**If any step is pending → Keep `[ ]`**
