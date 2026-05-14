---
name: todo-workflow
description: 7-step workflow protocol for task management (activation → closing) [desk/TODO.md, task tracking, validation, review, commit]
license: MIT
compatibility: opencode
metadata:
  related: desk/TODO.md, validation-checkpoint, task-closing
---

## What I do

I enforce the 7-step workflow protocol for every task to ensure traceability and user alignment.

## When to use me

Use this skill at the start of every new task or feature request.

## Quick Reference

**Full workflow:** See `desk/TODO.md` section "## Workflow" for complete 7-step protocol

**Core steps:**
1. **Activation** ← **Start**: Mark `(En cours 🚧)` + propose plan (Plan mode)
2. **Execution**: Implement changes (Build mode)
3. **Validation**: Run tests (`go test ./...`)
4. **Review (BLOCKER)**: Present summary + request user approval
5. **Commit**: Commit with semantic message (after explicit "OK")
6. **Documentation**: Update docs if needed
7. **Closing** ← **End**: Mark `(Terminé ✅)` + summary (after validation & commit)

## Key Rules

**Validation:**
- ✅ 1 action = 1 validation (after each file edit, test, commit)
- ✅ Test failed → STOP + ask for guidance (don't iterate alone)
- ✅ Review before commit: NEVER commit without explicit user approval
- ✅ User reviews with `git diff` or `meld` before approval

**Mode management:**
- ✅ Plan mode required for design/options discussion
- ✅ Build mode required for code changes
- ✅ Ask "Peux-tu activer le mode Build ?" (user controls UI dropdown)

**Task management:**
- ✅ One task = One commit per repository (separate commits for code + docs)
- ✅ Add bugs/new ideas to `desk/TODO.md` immediately as `(À faire 🚩)`
- ✅ Read `desk/TODO.md` + project documentation (README.md, ARCHITECTURE.md, desk/*.md) before starting implementation

## Benefits

1. **Traceability**: Every change tracked in `desk/TODO.md`
2. **Alignment**: User validates before each commit
3. **Quality**: Tests run at each step, no solo iteration on failures
4. **Documentation**: Architecture decisions captured as you go

## Related

- `validation-checkpoint` - When to ask for validation
- `task-closing` - Documentation and closing process
