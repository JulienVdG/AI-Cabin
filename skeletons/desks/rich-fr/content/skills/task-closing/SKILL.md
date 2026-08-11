---
name: task-closing
description: Task closing protocol - documentation updates and completion tracking
license: MIT
compatibility: opencode
metadata:
  related:
    - desk/TODO.md, project documentation (README.md, ARCHITECTURE.md, desk/*.md)
    - skill:workflow-protocol
---

## What I do

I ensure proper task closing with documentation updates and completion tracking in `desk/TODO.md`.

## When to Use Me

Use this skill when:
- ✅ Implementation is complete and tested
- ✅ User has approved the code changes
- ✅ Code has been committed
- ✅ Ready to mark task as complete

## Task Closing Process

### Step 1: Assess Completion

Determine if feature is complete:

**Incomplete feature:**
- Plan remaining work
- Return to Execution step (skill:workflow-protocol step 2)
- Create sub-tasks in `desk/TODO.md` if needed

**Complete feature:**
- Proceed to documentation

### Step 2: Identify Documentation Needs

**Check for:**

- 📝 Significant patterns worth documenting
- 📝 Architectural decisions made
- 📝 Implementation details for future reference
- 📝 Lessons learned

**Questions to ask:**

```
Should I update:
1. project documentation (README.md, ARCHITECTURE.md, desk/*.md) with new patterns?
2. desk/DEVELOPMENT.md with commands/examples?
3. Other documentation files?
```

### Step 3: Propose Documentation Changes

**Format:**

```markdown
## Documentation Updates

I propose adding to project documentation (README.md, ARCHITECTURE.md, desk/*.md):

### Section: [New Section Name]

[Proposed content]

Review and validate?
```

**Wait for explicit approval before writing.**

### Step 4: Update `desk/TODO.md`

**Mark task as complete:**

```markdown
- [x] **Task name** (Terminé ✅) : Summary description of what was done.
```

**Include:**
- ✅ Status: `(Terminé ✅)`
- ✅ Summary: 1-2 sentences describing what was implemented

**Example:**

```markdown
- [x] **Refactor validation layer** (Terminé ✅) : Validation API extracted to separate package with public `validator.New()` function. 23 tests passants.
```

### Step 5: Commit Documentation

**IMPORTANT:** Commit docs separately from code!

```bash
# After user approval of docs
cd desk
git add todo.md
git commit -m "docs: [description]"

# For project documentation, commit in the project repo:
cd path/to/project && git add README.md ARCHITECTURE.md
git commit -m "docs: [description]"
```

**Semantic commit format:**
- `docs: [description]` for documentation changes
- No trailing period in commit message

### Step 6: Propose Session Title

**For user's session tracking:**

```markdown
## 🏁 Session Title

Suggested: `Done: [5-7 words summary]`

Example: `Done: Greyproxy security documentation (4 options + firewalld)`
```

**Why:** User tracks sessions with `Done:` prefix for their records.

### Step 7: Confirm Completion

```markdown
✅ Task complete: [Task name]

Committed:
- Code: [commit hash or message]
- Docs: [commit hash or message]

Next steps: [suggest next task from desk/TODO.md or ask for new direction]
```

## Examples

### Simple Bug Fix

```
User: "Fix converter edge case"

[Implementation complete, tests passing, code committed]

Me:

## Documentation

This fix is covered by existing docs in `desk/DEVELOPMENT.md`. No architecture updates needed.

Should I:
1. Mark task complete in desk/TODO.md?
2. Add test coverage note to DEVELOPMENT.md?

## Update desk/TODO.md

- [x] **Converter edge case fix** (Terminé ✅) : Handle empty input and nil pointer cases. 23 tests passants.

Validate and commit?
```

### New Feature

```
User: "Add validation layer"

[Implementation complete]

Me:

## Documentation Updates

I propose adding to project documentation (README.md, ARCHITECTURE.md, desk/*.md):

### Validation Strategy

**Decision:** Separate validator package with public API

**Implementation:**
- `src/validator/validator.go`: Core validation logic
- `src/validator/options.go`: Functional options pattern

Should I proceed?

## Update desk/TODO.md

- [x] **Validation layer** (Terminé ✅) : Validator package with functional options pattern. Public API `validator.New()`. 23 tests passants.

Validate docs changes?
```

### Incomplete Feature

```
[Partial implementation complete]

Me:

## Completion Status

Core implementation done. Remaining work:

- [ ] Unit tests for validator
- [ ] Documentation
- [ ] Integration with main converter

Should I:
1. Mark as complete and create follow-up tasks?
2. Continue with remaining items?
3. Document as-is?
```

## Dynamic Task Management

**When bugs or new ideas are discovered:**

### Case 1: Idea from User

**At user request:** Note it immediately to avoid forgetting.

**If in Plan mode:** Keep as Step 0 - note the idea before planning (will need validation before commit).

**Format:** Reformulate user's text properly in `desk/TODO.md`:

```markdown
- [ ] **User's feature request** (À faire 🚩) : Reformulated description.
```

**Ask validation:**

```
Adding to desk/TODO.md:

- [ ] **[name]** (À faire 🚩) : [reformulated text]

Validate?
```

### Case 2: Idea from Agent

**If discovered during implementation:** Propose adding the item with category and suggested text.

```
During implementation, I found:
- Edge cases not handled for empty input

Should I add to desk/TODO.md:

- [ ] **Empty input handling** (À faire 🚩) : Add validation for empty/nil input in converter.

Validate?
```
