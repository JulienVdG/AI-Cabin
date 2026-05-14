---
name: debug-go
description: Debug Go code with testing and TDD patterns [unit tests, integration tests, DOM snapshots, test failures]
license: MIT
compatibility: opencode
metadata:
  language: go
  related: desk/DEVELOPMENT.md
---

## What I do

I help debug Go code using testing patterns, TDD workflows, and DOM exploration tests.

## When to use me

Use this skill when:
- A test fails unexpectedly
- You need to understand a bug
- Implementing new features with TDD
- Debugging DOM/tree manipulation code

## CRITICAL: Validation Rules

**Before iterating on failures:**

- ✅ Test fails once → Analyze, propose fix, ask validation
- ✅ Test fails twice → Stop, ask: "Should I debug more, change approach, or accept?"
- ✅ Test reveals design issue → Return to Plan mode, explore options
- ✅ Multiple failures in a row → Stop and ask for guidance

**NEVER:**
- ❌ Iterate alone: write→test→fix→test→fix→... (without asking)
- ❌ Modify test expectations without validation
- ❌ Make design decisions when tests reveal plan issues

## Debugging Workflow

### 1. Reproduce the Bug

```bash
make test run=TestSpecificCase
```

### 2. Create Minimal Test Case

```go
func TestConversionIssue(t *testing.T) {
    input := `some input data`
    expected := `expected output`
    
    result, err := Process(input)
    
    if err != nil {
        t.Fatal(err)
    }
    if result != expected {
        t.Errorf("Expected %q, got %q", expected, result)
    }
}
```

### 3. Run Test with Verbose Output

```bash
make test run=TestSpecificCase
```

### 4. Analyze Failure

**Expected vs Got:**
- Compare output carefully
- Look for whitespace differences
- Check DOM/tree structure if applicable

### 5. Propose Fix (Ask Validation)

**Before modifying code:**

```
Test failed. Expected X but got Y.

Root cause: [analysis]

Proposed fix: [description]

Should I:
1. Apply this fix?
2. Debug more?
3. Change approach?
```

**After validation:**

```bash
# Apply fix
# Re-run test
make test run=TestSpecificCase
```

**If test fails again:** Stop and ask for guidance (don't iterate alone).

## TDD Workflow (DOM Snapshots)

For complex DOM/tree manipulations:

### 1. Create Test with Expected DOM

```go
func TestFeature(t *testing.T) {
    testCases := []testCase{
        {
            name:  "simple_case",
            input: "input data",
            expectedAfterDOM: `
Document:
    - Element<root>:
        - Element<child>:
            - Text: "content"
`,
        },
    }
}
```

### 2. Run Test - It Will Fail

```bash
make test run=TestFeature/simple_case
```

### 3. Copy Actual DOM from Debug Output

```yaml
# Debug log shows actual DOM
# Copy it to fix the expectation
```

**Ask validation before modifying test:**

```
Test shows actual DOM structure. 
Proposed fix: Update expectedAfterDOM to match actual.

This reflects correct behavior?
1. Yes, update test
2. No, fix the code instead
```

### 4. Verify Semantics

Check if the DOM structure matches expected behavior.

### 5. Apply Fix (After Validation)

- If test expectation wrong → Update test
- If code wrong → Fix code
- **Re-run test**

**If still failing:** Stop and ask for guidance (don't iterate alone).

## DOM Exploration Tests

For learning DOM manipulation patterns:

```bash
make test run=TestDOM
```

**Purpose:** Understand mechanics without assertions.

**Output:** Verbose YAML snapshots showing DOM before/after.

## Testing Infrastructure

### YAML Tree Representation

Compact YAML DOM/tree representation:

```go
// Create from DOM
node := simplenode.New(htmlNode)

// Parse from YAML
node, err := simplenode.ParseYAML(yamlStr)

// String representation
fmt.Println(node.String())
```

### Test Commands

```bash
# Run all tests
make test

# Run specific test
make test run=TestFeature

# Run with coverage
make cover

# Run with verbose
make test run=.
```

## Common Patterns

### Two-Pass Approach

Safe for DOM/tree manipulation:

```go
// Pass 1: Collect nodes
var toMove []*Node
for child := root.FirstChild; child != nil; child = child.NextSibling {
    if shouldMove(child) {
        toMove = append(toMove, child)
    }
}

// Pass 2: Move nodes
for _, node := range toMove {
    root.RemoveChild(node)
    newParent.AppendChild(node)
}
```

### Preserved Elements

Never process these in transformations:
- `script`, `style`, `textarea`, `noscript`
- `svg`, `math`, `canvas`, `video`, `audio`
- `pre`, `code` (handled as blocks)

### Block Elements

Close paragraph, don't wrap:
- `div`, `ul`, `ol`, `li`, `pre`
- `h1-h6`, `blockquote`, `address`
- `table`, `form`, `fieldset`

## Related Skills

- `validation-checkpoint` - When to ask validation (CRITICAL)
- `todo-workflow` - Full task protocol
