# Development Guide

Practical guide for developing, debugging, and building Go projects.

## Quick Commands

### Code Quality

```bash
# Run linters
make lint

# Auto-fix lint issues
make lint-auto-fix

# Format imports only
make fmt
```

**Configuration:** `.golangci.yml` enables:
- `goimports` - automatic import management (local prefix: customize per project)
- All default linters (errcheck, gosimple, etc.)

---

### Build & Test

```bash
# Run Go code without building binaries
go run .

# Run tests (quick, no verbose)
make test

# Run specific test with verbose output
make test run=TestFeature

# Run all tests with verbose
make test run=.

# Run tests with coverage
make cover

# Never build binary
Ban `go build` from your usage, if needed (profiling for instance) ask for permission.

# Prefer the use of Makefile for normal tasks
make
```

**Examples:**
```bash
make test run=TestDOM              # Run DOM exploration tests
make test run=TestConverter        # Run converter integration tests
make test run=TestMarshalYAML      # Run specific test case
```

---

### Full Build Procedure

#### Complete rebuild (from scratch):

```bash
make clean build
```

#### Step by step (for debugging):

```bash
make clean
# check stuff
make build
```

---

## Debugging Workflow

### Creating Reproduction Tests

When investigating conversion issues:

1. Create a minimal test case in `src/converter_test.go`
2. Use the exact input that causes the issue
3. Assert the expected output
4. Run with `go test -v -run TestSpecificCase`

Example test structure:

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

---

## Makefile Patterns

### Standard Targets

```makefile
# Default: run all tests
.PHONY: test
test:
	go test ./...

# Run specific test
.PHONY: test
test:
	go test -v -run $(run) ./...

# Lint code
.PHONY: lint
lint:
	golangci-lint run

# Auto-fix lint issues
.PHONY: lint-auto-fix
lint-auto-fix:
	golangci-lint run --fix

# Format imports
.PHONY: fmt
fmt:
	goimports -local $(module) -w .

# Test coverage
.PHONY: cover
cover:
	go test -cover ./...

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf bin/ dist/
```

### Module-Specific Commands

Customize `$(module)` to your Go module name (e.g., `github.com/user/project`).

---

## Makefile Awareness

**CRITICAL: Always read the Makefile BEFORE proposing commands.**

### Why

- Makefiles encapsulate project best practices
- They may have dependencies (e.g., `setup` before `docker-build`)
- Centralized maintenance (changes in one place)
- Avoids proposing raw `docker-compose` or `go` commands when Makefile targets exist

### How

```bash
# Read the Makefile
cat Makefile

# Or list targets
grep "^[a-z].*:" Makefile
```

### Example (cabin/greymeta)

```bash
# ❌ Don't propose
docker-compose build
docker-compose up -d

# ✅ Use Makefile targets
make docker-build      # Includes setup dependency
make docker-up         # Includes setup dependency
make docker-shell      # Wrapper for docker compose exec
```

### When to Use Makefile

**Always check for Makefile when:**
- Starting work in a new directory
- About to run build/test/deploy commands
- User mentions "make" or Makefile targets
- Debugging build issues

**Propose raw commands only if:**
- No Makefile exists
- Makefile doesn't have the needed target
- User explicitly asks for raw command

---

## Testing Infrastructure

### Table-Driven Tests

```go
func TestFeature(t *testing.T) {
    testCases := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "simple_case",
            input:    "input",
            expected: "output",
        },
        {
            name:     "edge_case",
            input:    "",
            expected: "",
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            result := Process(tc.input)
            if result != tc.expected {
                t.Errorf("Expected %q, got %q", tc.expected, result)
            }
        })
    }
}
```

---

## Related

- `skill:debug-go` - Debugging patterns and validation rules
- `skill:sqlc-queries` - Type-safe SQL with sqlc
- project documentation (README.md, ARCHITECTURE.md, desk/*.md) - Project-specific architecture notes
