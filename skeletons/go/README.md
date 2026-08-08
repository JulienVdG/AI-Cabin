# ${ProjectName}

Brief description of the project.

## Structure

```
project/
├── cmd/
│   └── ${project}/
│       └── main.go      # Application entry point
├── internal/             # Private code (not importable by other modules)
│   └── .gitkeep          # Empty by default
├── pkg/                  # Public code (importable by other modules)
│   └── .gitkeep          # Empty by default
├── Makefile
├── go.mod
├── .golangci.yml
└── README.md
```

**Directory usage:**

- `cmd/` - Application entry points (thin wrappers around internal logic)
- `internal/` - Private application code (cannot be imported by other modules)
  - Use for business logic, domain models, services
  - Enforces encapsulation at compile time
- `pkg/` - Public library code (can be imported by other modules)
  - Use for shared utilities, public APIs
  - Keep minimal - most code should be in `internal/`

**Note:** Both `internal/` and `pkg/` are empty by default. Add packages as needed.

## Quick Start

```bash
# Run the application
make project=${project} run

# Run tests
make test

# Run specific test
make test run=TestName

# Build binary
make project=${project} build

# Lint code
make lint

# Format imports
make module=${module} fmt
```

## Development

See `desk/DEVELOPMENT.md` in the AI Cabin workspace for development workflow, debugging tips, and testing patterns.

## Project Workflow

This project uses an AI-assisted workflow with:
- Task tracking in `desk/TODO.md` (AI Cabin workspace)
- Validation checkpoints before commits
- Skills in `skills/` for consistent patterns

See `rules/agent.md` in the AI Cabin workspace for AI agent instructions.

## Example Project Layout

For a complete example, see the AI Cabin project structure in `desk/ai-cabin.md` (AI Cabin workspace).

**Typical layout:**
```
myapp/
├── cmd/
│   └── myapp/
│       └── main.go       # Entry point
├── internal/
│   ├── converter/        # Business logic
│   │   ├── converter.go
│   │   └── converter_test.go
│   └── validator/
│       └── validator.go
├── pkg/
│   └── simplenode/       # Public library (optional)
│       └── simplenode.go
├── Makefile
├── go.mod
└── README.md
```

## License

MIT
