---
name: interactive-rebase
description: Assist with interactive git rebase [git, rebase, history, cleanup, squash, reorder]
license: MIT
compatibility: opencode
metadata:
  source: desk/interactive-rebase.md
  related:
    - todo-workflow
    - validation-checkpoint
    - semantic-commit
---

## What I do

I assist with interactive git rebase following the process in **desk/interactive-rebase.md**.

## Quick Reference

**Full process**: Read `desk/interactive-rebase.md` for complete workflow, patterns, and checklists.

**Core principle**: Two-pass method
1. **Reorder first** - Organize commits without modifying content
2. **Squash second** - Combine related commits, rewrite messages

## How I Help

### Before Rebase

- [ ] List commits: `git log --oneline main..HEAD`
- [ ] Analyze changes: `git log --stat main..HEAD`
- [ ] Propose grouping strategy (by file, feature, or type)
- [ ] Generate `rebase-guide.md` with exact commands

### During Rebase

- [ ] Track which commits are reordered/squashed
- [ ] Update `rebase-guide.md` if user splits commits
- [ ] Suggest commit splits when conflicts occur
- [ ] Propose alternative placements for problematic commits

### After Rebase

- [ ] Verify commit count reduced
- [ ] Check `git log --oneline` for clarity
- [ ] Remove temporary `rebase-guide.md`
- [ ] Confirm tests still pass (if applicable)

## Critical Rules

**DO:**
- ✅ Propose two-pass method (reorder → squash)
- ✅ Generate and maintain `rebase-guide.md` during process
- ✅ Update guide after user splits/reorders commits
- ✅ Suggest commit splits for conflicts
- ✅ Listen to user preferences on grouping
- ✅ Remove guide after rebase complete

**DO NOT:**
- ❌ Run `git rebase` commands yourself (user controls git)
- ❌ Commit changes without explicit approval
- ❌ Keep `rebase-guide.md` after rebase done
- ❌ Assume grouping strategy without validation

## Rebase Guide Template

**File**: `rebase-guide.md` (temporary, delete after rebase)

```markdown
# Rebase Interactive Guide
# Branch: <branch-name>
# From: <N> commits → <M> commits

## Commande de départ
git rebase -i main

## Ordre des commits (du plus ancien au plus récent)

### Commit 1 : KEEP/SQUASH
pick <hash> <message>
squash <hash> <message>

Message de commit suggéré :
<final message>

[... repeat for each group ...]

## Résumé visuel
<final commit list>

## Si conflits
git add <resolved-files>
git rebase --continue
```

## Common Grouping Strategies

| Strategy | Use When | Example |
|----------|----------|---------|
| **By file** | Multiple commits same file | `rewrite ai-cabin.md` (4→1) |
| **By feature** | Commits implement one feature | `add auth` (3→1) |
| **By type** | Separate code, docs, config | `feat:` + `docs:` + `chore:` |
| **Keep separate** | Independent changes | `fix: typo` + `feat: new` |

## Conflict Assistance

**When user reports conflict:**

1. Ask which commit/file caused conflict
2. Propose splitting the commit:
   ```bash
   git reset --soft HEAD~1
   git add <file1>
   git commit -m "part 1"
   git add <file2>
   git commit -m "part 2"
   ```
3. Suggest alternative placements (like `c82eac2` example)
4. Update `rebase-guide.md` with new commit hashes

## Related Skills

- `todo-workflow` - Task tracking and validation
- `validation-checkpoint` - When to ask for approval
- `semantic-commit` - Writing clear commit messages
