---
name: design-protocol
description: Design First, Code Second - explore options before implementation
license: MIT
compatibility: opencode
metadata:
  related:
    - project documentation (README.md, ARCHITECTURE.md, desk/*.md)
    - skill:workflow-protocol
    - skill:semantic-commit
---

## What I do

I enforce the "Design First, Code Second" principle: explore design options with tradeoffs BEFORE writing any code.

## Core Principle

**Before implementing ANY feature or change:**

1. **Explore options** (max 3 alternatives with clear tradeoffs)
2. **Wait for validation** before writing code
3. **Respect existing philosophy** of the component

## Step 1: Explore Options

Present **max 3 alternatives** with concise tradeoffs:

### Format

```markdown
## Design Options

### Option A: [Simple but limited]
**Pros:** [list]
**Cons:** [list]

### Option B: [Flexible but complex]
**Pros:** [list]
**Cons:** [list]

### Option C: [User's suggestion]
**Pros:** [list]
**Cons:** [list]

## Recommendation

I recommend [Option X] because [reason].

Which approach do you prefer?
```

### Rules

- ✅ **Always include user's suggestion** even if it has many cons (shows it was considered)
- ✅ Avoid analysis paralysis (no 10+ options tables)
- ✅ Present pros/cons concisely
- ✅ Include recommendation with reasoning

## Step 2: Wait for Validation

**Do NOT write code until:**

- ✅ User explicitly chooses an option
- ✅ User says "go", "OK", or "validated"
- ✅ User confirms approach is clear

**Example confirmation:**

```
✅ Approach validated: Option B (Context + localStorage)

Proceeding to implementation.
```

## Step 3: Respect Existing Philosophy

### Before Making Changes

Checklist:

- [ ] Read the existing code to understand its philosophy
- [ ] Check if similar components exist (maintain consistency)
- [ ] Read related documentation (comments, markdown files, git history)
- [ ] Identify the original design decisions
- [ ] Propose aligned changes, or explicitly flag deviations

### Good Pattern

```markdown
J'ai analysé [composant]. Sa philosophie est : [X].
J'ai relu : [docs, commentaires, git history].
Je propose : [changement aligné].
⚠️ Point d'attention : [déviation éventuelle].
```

### Bad Pattern

```go
// ❌ Code already written without discussing design
func NewFeature() {
    // ...
}
```

## Mode Transitions

### Plan Mode Workflow

```
1. Read existing code to understand patterns
2. Present 2-3 design options with tradeoffs (including user's idea)
3. Wait for user choice
4. Confirm approach is clear
```

### Build Mode Workflow

```
5. Implement the validated design
   → If you discover a problem with the plan:
     ❌ Bad: Change code alone on unplanned parts
     ✅ Good: Stop, explain the issue, suggest switching to Plan mode
6. Run tests, report results
   → If tests reveal the plan is not feasible:
     ❌ Bad: Iterate alone to fix (write→test→fix→test→...)
     ✅ Good: Stop, present the issue, go back to Plan step 2
7. Ask for validation before commit
```

## When to Return to Plan Mode

**Suggest switching to Plan mode if:**

- ⚠️ Implementation reveals a problem with the plan
- ⚠️ Tests show the approach is not feasible
- ⚠️ User explanations seem unclear
- ⚠️ You need to make a design decision not covered in the original plan

**Example:**

```
⚠️ Problem discovered: The current approach won't work because [reason].

Should I:
1. Switch to Plan mode to explore alternatives?
2. Proceed with [modified approach]?
```

## Examples

### Starting a New Feature

```
User: "Add dark mode toggle"

Me (Plan mode):

## Design Options

### Option A: CSS variables only
**Pros:** Simple, no JavaScript, fast
**Cons:** No persistence across sessions, limited customization

### Option B: Context + localStorage
**Pros:** Persistent, flexible, supports system preference
**Cons:** More complex, requires JavaScript

### Option C: Your idea - Theme provider
**Pros:** Centralized, reusable across components
**Cons:** Over-engineered for single toggle, adds dependency

## Recommendation

Option B balances simplicity and features. It persists user choice and detects system preference automatically.

Which approach do you prefer?
```

### Modifying Existing Component

```
User: "Refactor converter to handle edge cases"

Me (Plan mode):

J'ai analysé `converter.go`. Sa philosophie est : single-pass conversion with minimal allocations.

J'ai relu : 
- `desk/DEVELOPMENT.md` - Conversion patterns
- `src/converter_test.go` - Test cases

Je propose : Two-pass approach (collect → process) to avoid iteration issues during node movement.

⚠️ Point d'attention : This deviates from current single-pass pattern because it requires additional memory.

Alternative options:
- Option A: Keep single-pass with node copying (more memory, safer)
- Option B: Two-pass approach (cleaner, requires refactoring)

Which approach?
```
