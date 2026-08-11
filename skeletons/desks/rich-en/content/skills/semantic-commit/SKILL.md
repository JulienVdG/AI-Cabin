---
name: semantic-commit
description: Write semantic git commits following conventional format
license: MIT
compatibility: opencode
metadata:
  source: important/instructions.md:53-69
  workflow: git
---

## What I do

I enforce semantic commit message format for all git commits.

## Commit Format

```
<type>: <description>
```

**Rules:**
- No trailing period
- Lowercase type
- Imperative mood in description
- Concise (max 72 chars for subject)

## Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `style` | Formatting, no code change |
| `refactor` | Code change, no behavior change |
| `test` | Adding or updating tests |
| `chore` | Maintenance, dependencies |

## Examples

```
feat: add dark mode toggle
fix: resolve nil pointer dereference in parser
docs: update ARCHITECTURE.md with multilingual strategy
refactor: extract wpautop logic to separate function
test: add DOM snapshot tests for inline elements
chore: bump Hugo version to 0.156.0
```

## Pre-Commit Checklist

Before ANY commit:

- [ ] Propose exact commit message (in English, no trailing period)
- [ ] Wait for explicit user approval ("OK", "validated", "go")
- [ ] Only commit in the repository that was discussed
- [ ] Do NOT chain commits across multiple repos without validation
- [ ] **When code and docs are both modified**: Remind user to approve both before commit
  - Suggest: "Code committed, docs also modified - review and approve?"
  - Prevents orphaned documentation changes

## Multi-Repository Workflow

**One approval = One commit**: Each file or logical change requires explicit user approval before committing.

**Example:**

```
✅ Code changes ready in importer/

Proposed commit:
feat: add priority system for PreRenderer

- PriorityWpAutoP = 495 (runs first)
- PriorityShortCodes = 510 (runs second)
- Preserves spaces around inline shortcodes

Validate?
```

**After approval:**

```bash
cd importer && git add . && git commit -m "feat: add priority system for PreRenderer"
```
