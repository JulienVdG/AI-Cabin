---
name: makefile-awareness
description: Always read Makefile before proposing commands [Makefile, build, test, docker, workflow]
license: MIT
compatibility: opencode
metadata:
  related:
    - desk/DEVELOPMENT.md
---

## What I do

I enforce the "read Makefile first" rule before proposing any build, test, or deployment commands.

## Quick Reference

**Full details**: Read `desk/DEVELOPMENT.md` section "Makefile Awareness" for complete guide.

## Critical Rule

**BEFORE proposing ANY command:**

```
1. Check if Makefile exists
2. Read available targets
3. Use Makefile targets instead of raw commands
```

## Examples

### ✅ Good Pattern

```
User: "How do I build this?"
Me: [reads Makefile]
Me: "Run `make docker-build` - it includes the setup dependency."
```

### ❌ Bad Pattern

```
User: "How do I build this?"
Me: "Run docker-compose build"
[Makefile has `make docker-build` with dependencies]
```

## When to Check Makefile

- Starting work in new directory
- About to run build/test/deploy commands
- User mentions "make" or Makefile targets
- Debugging build issues

## Related Skills

- `debug-go` - Debugging workflow with testing
- `todo-workflow` - Task protocol
