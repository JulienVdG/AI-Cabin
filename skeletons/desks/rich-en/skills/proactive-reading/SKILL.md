---
name: proactive-reading
description: Read files BEFORE asking questions. Inspect to assist, not just to plan [reading, inspection, context, workflow, collaborative editing]
license: MIT
compatibility: opencode
metadata:
  source: desk/AGENTS.md
  related:
    - skill:workflow-protocol
    - skill:makefile-awareness
---

## What I do

I read files proactively to understand context BEFORE asking questions. This applies ALL THE TIME, not just in Plan mode.

## Quick Reference

**Full details**: Read `desk/AGENTS.md` section "Proactive Inspection" for complete guide.

## Critical Rule

**BEFORE asking ANY question:**

```
1. Check git status / git diff (what changed)
2. Read relevant files (configs, logs, code)
3. THEN respond with understanding
```

## Examples

### ✅ Good Pattern

```
User: "I modified the Dockerfile"
Me: [reads Dockerfile, git diff]
Me: "I see you added socat. Is it to forward the greyproxy API?"
```

### ❌ Bad Pattern

```
User: "I modified the Dockerfile"
Me: "What did you change?"
Me: "Can you show me?"
[User already did this - AI should have read]
```

### ❌ Bad Pattern (Unilateral Overwrite)

```
Me: [sees --auto-profile in greypi, assumes error]
Me: [rewrites file without --auto-profile, erasing user's change]
```

### ✅ Good Pattern (Ask Before Overwriting)

```
Me: [sees --auto-profile in greypi]
Me: "I see you added --auto-profile. Is it intentional or a test?"
Me: "Should I keep it, or do you want me to remove it?"
```

## When to Read Proactively

**ALWAYS read before asking:**
- User mentions a file changed
- User mentions a test/error
- Starting work after a pause
- User seems frustrated by repeated questions
- Debugging any issue
- **File content doesn't match your mental model** (user probably edited)

**Read FIRST, then ask:**
- "I see X, do you want me to do Y?"
- "The logs show Z. Should we investigate?"
- "Git diff shows A and B. Is it intentional?"
- **"I see you modified [file]. Do you want me to integrate it?"**

## What to Read

**Git commands:**
```bash
git status
git diff --name-only
git diff --stat
git log --oneline -5
```

**Logs:**
```bash
# From host: docker compose logs agent | tail -100
# From container: journalctl, systemctl logs, etc.
```

**Configs:**
```bash
cat docker-compose.yml
cat greywall.json
cat .env
```

## Shared Workspace Reality

**Remember:** User can see ALL your file modifications in real-time.

**Implications:**
- User doesn't need you to "show" changes
- You don't need to describe what you did in detail
- Focus on: "Is the approach correct?" not "Did you see my change?"

**Good pattern:**
```
✅ File modified. Review in your editor. Should I commit?
```

## Collaborative Editing (CRITICAL)

**Context:** You and the user are BOTH editing files. File states can diverge from your mental model.

**When files don't match your expectations:**

| Scenario | Bad Response | Good Response |
|----------|--------------|---------------|
| User added flag you didn't | Remove it silently | "I see [flag]. Do you want to keep it?" |
| File changed since your read | Rewrite without checking | "Did you modify X while I was doing Y?" |
| Git shows user commits | Ignore and continue | "I see your commits. Do you want me to integrate?" |

**Core principle:**
> **Assume intentionality.** User changes are deliberate unless stated otherwise.  
> **When in doubt: ASK, don't overwrite.**
