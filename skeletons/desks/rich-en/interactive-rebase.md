# Interactive Rebase Guide

## Purpose

Clean up git history before merging a feature branch by reorganizing, combining, and rewriting commits for clarity and coherence.

## Core Principle: Two-Pass Method

**Always split interactive rebase into two separate passes:**

| Pass | Purpose | Commands | Risk Level |
|------|---------|----------|------------|
| **1. Reorder** | Organize commits chronologically/logically | `reorder`, `drop` | Low - no content changes |
| **2. Squash** | Combine related commits, rewrite messages | `squash`, `reword`, `edit` | Medium - content may change |

**Why two passes?**
- **Easier conflict detection**: Reordering first reveals conflicts early
- **Simpler splits**: One commit = one logical change (e.g., separate files)
- **Safer**: If conflicts occur, you're only reordering, not modifying content
- **Clearer mental model**: Structure first, then refine

---

## Pass 1: Reorder

### Goal

Organize commits in logical order without modifying their content.

### When to Reorder

- Commits are out of chronological order
- Related commits are scattered
- You need to split a commit that touches multiple unrelated files
- You want to group commits by theme before squashing

### Process

1. **List all commits**
   ```bash
   git log --oneline main..HEAD
   git log --reverse --oneline main..HEAD
   ```
   **Tip**: Use `gitk main..HEAD` or your favorite Git GUI for a visual overview.

2. **Analyze each commit**
   ```bash
   git log --stat main..HEAD
   git show <commit> --name-status
   ```
   **Tip**: GUI tools like `gitk`, `git gui`, or IDE integrations (VS Code, GitKraken, SourceTree) make it easier to browse commit contents visually.

3. **Identify conflicts early**
   - Look for commits that modify the same files
   - Check if file renames happen before modifications
   - Ensure dependencies are in correct order
   - **Tip**: In `gitk`, search for file names to quickly find all commits touching a file

4. **Start rebase (reorder only)**
   ```bash
   git rebase -i main
   ```

5. **Use only `pick` and `reorder`**
   - Do NOT use `squash`, `fixup`, or `edit` in this pass
   - Just arrange commits in desired order

6. **Complete rebase**
   ```bash
   # If conflicts occur, resolve and continue
   git add <resolved-files>
   git rebase --continue
   
   # Or abort if needed
   git rebase --abort
   ```

### Example: Splitting a Multi-File Commit

**Problem**: One commit modifies both `config.yml` and `README.md`

**Solution**: Split during rebase using `edit`

#### Option A: Command line

1. Start rebase with `edit` on the commit to split:
   ```bash
   git rebase -i main
   # Change 'pick' to 'edit' on the target commit
   ```

2. When rebase stops at the commit:
   ```bash
   # Undo the commit, keep changes staged
   git reset --soft HEAD~1
   
   # Commit files separately
   git add config.yml
   git commit -m "chore: update config"
   
   git add README.md
   git commit -m "docs: update readme"
   ```

3. Continue rebase:
   ```bash
   git rebase --continue
   ```

#### Option B: Using `git gui` (recommended for complex splits)

1. Start rebase with `edit` on the commit to split:
   ```bash
   git rebase -i main
   # Change 'pick' to 'edit' on the target commit
   ```

2. When rebase stops at the commit, launch `git gui`:
   ```bash
   git gui
   ```

3. In `git gui`, enable amend mode:
   - Check the box `[X] Amend last commit` (top right, above commit message area)
   - All changes from the commit now appear as "Staged Changes"
   - The last commit message is populated in the window

4. Unstage/Stage files selectively:
   - **Unstage all**: Select all in "Staged Changes" → `Unstage from Commit`
   - **By file**: Select file in "Unstaged Changes" → `Stage to Commit`
   - **By line**: Right-click on file → `Stage Hunk` or select lines → `Stage Selected Lines`
   - Click `Commit` → Write message → `Commit`
   - **Note**: Commit automatically unchecks `[X] Amend last commit`

5. Repeat for each file/section:
   - **Do not check** `[ ] Amend last commit` (to create new commit, not amend)
   - Stage next file/lines
   - `Commit` → New commit

6. Continue rebase:
   ```bash
   git rebase --continue
   ```

**Why `git gui`?**
- Visual file selection (no `git add -i` nightmare)
- Line-by-line staging within files (right-click → select lines)
- Clear commit message interface
- Perfect for complex multi-file splits
- Works great after `git reset --soft` during rebase

### Conflict Resolution Strategy

| Conflict Type | Solution |
|---------------|----------|
| Same file modified in adjacent commits | Keep chronological order, resolve in pass 2 |
| File rename + modification | Ensure rename comes first |
| Unrelated files in one commit | Split commit before rebase |

---

## Pass 2: Squash

### Goal

Combine related commits into coherent, well-named commits.

### Squash Strategies

| Strategy | When to Use | Example |
|----------|-------------|---------|
| **By file** | Multiple commits touch same file | `rewrite ai-cabin.md` (4 commits → 1) |
| **By feature** | Commits implement one feature | `add user authentication` (3 commits → 1) |
| **By type** | Separate code, docs, config | `feat: add API` + `docs: API docs` |
| **Keep separate** | Independent changes | `fix: typo` + `feat: new feature` |

### Process

1. **Review reordered commits**
   ```bash
   git log --oneline main..HEAD
   ```

2. **Identify squash groups**
   - Mark commits to combine
   - Decide on final commit message for each group

3. **Start rebase (squash)**
   ```bash
   git rebase -i main
   ```

4. **Mark commits**
   ```
   pick  abc123  Initial implementation
   squash  def456  Add tests
   squash  ghi789  Fix bugs
   pick  jkl012  Unrelated change
   ```

5. **Write clear commit messages**
   - Use semantic commit format
   - Summarize all changes in the group
   - Keep first line under 72 characters

6. **Complete rebase**
   ```bash
   git rebase --continue
   ```

### Commit Message Format

**Semantic commits:**
```
<type>: <subject>

[optional body]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `chore`: Configuration, tooling
- `refactor`: Code restructuring
- `test`: Adding tests

**Examples:**
```
docs: rewrite ai-cabin.md
chore: sync updated desk files to template
refactor: merge rules and skills into desk directory
```

---

## Verification

### After Rebase Complete

1. **Check commit count**
   ```bash
   git log --oneline main..HEAD
   # Should show fewer commits than before
   ```

2. **Review final history**
   ```bash
   git log --stat main..HEAD
   ```

3. **Verify no content changes**
   ```bash
   git diff main HEAD
   # Should show same end state as before rebase
   ```

4. **Run tests (if applicable)**
   ```bash
   make test
   # Ensure nothing broke
   ```

---

## Common Patterns

### Pattern 1: Documentation Cleanup

**Before** (15 commits):
```
docs: update intro
docs: fix typo
docs: add section
docs: restructure
docs: fix links
...
```

**After** (3 commits):
```
docs: rewrite getting-started.md
docs: update API reference
docs: fix typos and links
```

### Pattern 2: Feature Development

**Before** (8 commits):
```
feat: add initial handler
fix: handle edge case
test: add unit tests
fix: another bug
docs: add comments
refactor: clean up
fix: one more thing
chore: update config
```

**After** (3 commits):
```
feat: add request handler
test: add handler unit tests
chore: update handler config
```

### Pattern 3: Multi-File Refactoring

**Before** (1 commit touching 10 files):
```
refactor: restructure project
```

**After** (4 commits by logical group):
```
refactor: move utils to lib/
refactor: update imports in handlers
refactor: restructure tests
chore: update tsconfig paths
```

---

## AI Agent Assistance

An AI agent can help with:

1. **Analysis**: List commits with `git log --stat`, identify patterns
2. **Strategy**: Propose grouping (by file, feature, type)
3. **Guide**: Generate temporary `rebase-guide.md` with exact commands
4. **Conflict help**: Suggest commit splits when conflicts occur
5. **Updates**: Keep guide synchronized after user splits/reorders

**See**: `skill:interactive-rebase` for AI workflow details.

---

## Checklist

### Before Starting

- [ ] Branch is up to date with main
- [ ] No uncommitted changes (`git status` clean)
- [ ] Tests pass on current branch
- [ ] Backup created (optional: `git branch backup-branch`)

### During Pass 1 (Reorder)

- [ ] Commits arranged in logical order
- [ ] No `squash`/`fixup`/`edit` used yet
- [ ] Conflicts resolved (if any)
- [ ] `git log --oneline` shows correct order

### During Pass 2 (Squash)

- [ ] Related commits grouped
- [ ] Commit messages are clear and semantic
- [ ] No duplicate or redundant commits
- [ ] Final commit count is reasonable

### After Rebase

- [ ] `git log --oneline` shows clean history
- [ ] `git diff main HEAD` shows expected changes
- [ ] `git diff backup-branch HEAD` shows no diff (or expected ones: dropped commits, typo fixes)
- [ ] Tests still pass
- [ ] Ready for review/merge

---

## Troubleshooting

### "I made a mistake during rebase"

```bash
# Abort current rebase
git rebase --abort

# Or if already finished, reset to backup
git reset --hard backup-branch
```

### "Too many conflicts"

- Split the problematic commit before rebase
- Use `edit` to manually fix during rebase
- Consider smaller, more focused commits next time

### "I lost a commit"

```bash
# Check reflog
git reflog

# Recover commit
git reset --hard <commit-hash>
```

### "Commit message is wrong after squash"

```bash
# Reword specific commit
git rebase -i HEAD~3  # Adjust number as needed
# Change 'pick' to 'reword' for that commit
```
