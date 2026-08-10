# Skills Writing Guide

## Principle: Reference, Don't Duplicate

**A skill should REFERENCE documentation, not duplicate it.**

- ✅ **DO**: Reference `desk/retro.md` for complete process
- ❌ **DON'T**: Duplicate process steps in skill files

**Rationale:**
- Skills (`skills/*.md`) = AI-focused, quick reference
- Rules (`desk/*.md`) = Human-readable, comprehensive

**Benefit:** Single source of truth, easier maintenance, less duplication.

---

## Principle: Reference Sections, Not Line Numbers

**When referencing documentation, use section titles instead of line numbers.**

- ✅ **DO**: "See `desk/DEVELOPMENT.md` section 'TDD Workflow'"
- ❌ **DON'T**: "See `desk/DEVELOPMENT.md` line 305"

**Rationale:**
- Line numbers change every time the file is edited
- Section titles are stable and semantic
- Easier for humans to find (search by title)

**Benefit:** References remain valid even as documentation evolves.

---

## Skill Structure

### 1. Header (YAML metadata)

```yaml
---
name: skill-name
description: Short description [keyword1, keyword2, keyword3]
license: MIT
compatibility: opencode
metadata:
  source: desk/some-doc.md
  related:
    - other-skill-1
    - other-skill-2
---
```

**Description with tags:**
- Add keywords in brackets at the end for better discovery

**Keywords should include:**
- Component names (e.g., `converter`, `validator`)
- Tools (e.g., `sqlc`, `YAML`)
- Concepts (e.g., `migration`, `validation`)

**Example:**
```yaml
description: Session retrospective following desk/retro.md [feedback, session review, improvement, workflow, validation]
```

### 2. What I do (1-2 sentences)

```markdown
## What I do

I [action] following the process in **desk/some-doc.md**.
```

**Keep it brief:** Reference the main documentation for details.

### 3. Quick Reference (optional)

```markdown
## Quick Reference

**Full process**: Read desk/some-doc.md for complete workflow, tables, and checklists.

**Output format**:
```markdown
[template or example]
```

**Critical Rules**:
- ✅ **DO**: [rule 1]
- ❌ **DO NOT**: [rule 2]
```

**Purpose:** Give AI quick access to essential info without reading full doc.

### 4. Tables (if applicable)

```markdown
## Skills Locations

| Type | Write to | Review |
|------|----------|--------|
| Global (universal) | `skills/` | `git diff` in project |
| Technical (project) | `.agents/skills/` in that project | `git diff` in project |
```

**Purpose:** Quick lookup for common patterns.

### 5. Related Skills

```markdown
## Related Skills

- `skill-1` - Description
- `skill-2` - Description
```

**Purpose:** Cross-reference related workflows.

---

## Skill vs Documentation

| Create a **Skill** when... | Create a **Doc** when... |
|---------------------------|--------------------------|
| Reusable AI pattern | Human-facing guide |
| Workflow enforcement | Comprehensive reference |
| Step-by-step process | Examples + tutorials |
| Quick reference needed | Checklists + tables |
| AI needs to "follow" or "enforce" | Human needs to "understand" or "learn" |

**Rule of thumb:**
- **Skill** = AI workflow enforcement (follow this process)
- **Doc** = Human comprehensive guide (understand this concept)

---

## Examples

### ✅ Good Skill (focused, references doc)

**File:** `skills/retro-process/SKILL.md` (95 lines)

```yaml
---
name: retro-process
description: Session retrospective following desk/retro.md [feedback, session review, improvement, workflow, validation]
---

## What I do

I run session retrospectives following the process in **desk/retro.md**.

## Quick Reference

**Full process**: Read desk/retro.md for complete workflow, tables, and checklists.

**Output format (2 steps)**:
...
```

**Why it's good:**
- ✅ References `desk/retro.md` (single source of truth)
- ✅ Concise (95 lines vs 304 lines in doc)
- ✅ Includes output format template
- ✅ Has tags for discovery

### ✅ Good Doc (comprehensive)

**File:** `desk/retro.md` (310 lines)

- Complete process description
- Detailed tables (skills locations, commands, files)
- Full example session
- Documentation Index
- End of Session Checklist

**Why it's good:**
- ✅ Human-readable
- ✅ Comprehensive reference
- ✅ Examples with full context
- ✅ Checklists for humans

### ❌ Bad Pattern (duplication)

**Before:** `skills/retro-process/SKILL.md` (295 lines)

- Duplicated entire process from `desk/retro.md`
- Duplicated tables
- Duplicated examples
- Duplicated checklists

**Why it's bad:**
- ❌ 295 lines vs 304 lines in doc (almost identical!)
- ❌ Two sources of truth (diverge over time)
- ❌ Harder to maintain (update in two places)
- ❌ No benefit over just reading the doc

**After:** Reduced to 95 lines by referencing `desk/retro.md`

---

## Checklist for New Skills

Before creating a new skill:

- [ ] **Check if doc exists**: Is there a `desk/*.md` that describes this?
- [ ] **Reference, don't duplicate**: Link to doc instead of copying content
- [ ] **Keep it concise**: Target < 100 lines for most skills
- [ ] **Add tags**: Include keywords in description brackets
- [ ] **Add metadata**: `source` and `related` in YAML header
- [ ] **Include output format**: Template or example if applicable
- [ ] **List related skills**: Cross-reference for discoverability

---

## Maintenance

**When updating documentation:**
1. Update `desk/*.md` first (source of truth)
2. Review related skills to ensure references are still accurate
3. Update skills only if quick reference needs changes

**When updating skills:**
1. Check if change belongs in doc instead
2. If yes, update doc first, then skill reference
3. If no (skill-specific), update skill directly

**Golden rule:** Documentation is the source of truth. Skills reference docs.
