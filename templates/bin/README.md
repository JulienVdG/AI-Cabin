# Bootstrap AI Cabin Environment

Generic script to bootstrap a new AI Cabin environment from scratch.

## Quick Start

```bash
# Test environment in /tmp
./bootstrap-cabin.sh /tmp/ai-cabin-test-1

# Production environment in home
./bootstrap-cabin.sh /home/user/ai-cabin-prod

# Custom environment with name
./bootstrap-cabin.sh /home/user/my-cabins project-x
```

## What the Script Does

1. **Creates directories:**
   - `<base-path>/home/` → `AI_CABIN_HOME` (empty, `make setup` creates subdirs)
   - `<base-path>/desk/` → `AI_CABIN_DESK` (populated with AGENTS.md, working docs, skills/)
   - `<base-path>/workdir/` → `AI_CABIN_WORKDIR` (empty, for git repos)

2. **Copies templates to WORKFLOW:**
   - `desk/` → TODO.md (from template), docs including  system rules (AGENTS.md, DEVELOPMENT.md, etc.)
   - `desk/skills/` → All skill modules

3. **Creates `.envrc`** at base path with all variables configured

4. **You copy `.envrc` to your cabin** and test from there

## Usage

```bash
# 1. Bootstrap environment
./bootstrap-cabin.sh /tmp/ai-cabin-test-1

# 2. Copy .envrc to your cabin
cp /tmp/ai-cabin-test-1/.envrc /path/to/your/cabin/.envrc

# 3. Go to your cabin
cd /path/to/your/cabin  # e.g., cabin/opencode-go/

# 4. Allow direnv
direnv allow

# 5. Run setup (creates AI_CABIN_HOME subdirs, copies config)
make setup

# 6. Start cabin
make docker-up
```

## Testing Checklist

### Prerequisites (on host)

```bash
# Check greywall installed
~/.local/bin/greywall --version

# Check greyproxy running
greyproxy --version
curl -s http://localhost:43080/health

# Check CA cert exists
ls -la ~/.local/share/greyproxy/ca-cert.pem
```

### Test Steps

```bash
# 1. Bootstrap environment
./bootstrap-cabin.sh /tmp/ai-cabin-test-1

# 2. Copy .envrc to cabin
cp /tmp/ai-cabin-test-1/.envrc cabin/opencode-go/.envrc

# 3. Allow direnv
cd cabin/opencode-go
direnv allow

# 4. Run setup
make setup

# Expected:
# - AI_CABIN_HOME/.local/share/opencode/ created
# - AI_CABIN_HOME/.local/state/opencode/ created
# - AI_CABIN_HOME/.config/opencode/ created
# - AI_CABIN_HOME/.config/greywall/ created
# - Config files copied (greywall.json, opencode.json, AGENTS.md)

# 5. Build and start
make docker-up

# Expected: Container starts without errors

# 6. Check logs
make docker-logs

# Expected: No permission errors, greywall loaded

# 7. Test greyshell
make docker-greyshell

# Expected: Sandboxed shell works

# 8. Test desk write
docker compose exec agent bash -c "echo test > ~/desk/test.txt"

# Expected: No permission denied
```

## Directory Structure After Bootstrap

```
/tmp/ai-cabin-test-1/
├── .envrc                    # To source before launching the cabin
├── home/                     # AI_CABIN_HOME (empty)
├── desk/                     # AI_CABIN_DESK
│   ├── AGENTS.md
│   ├── TODO.md
│   ├── DEVELOPMENT.md
│   ├── (other docs)
│   ├── ...
│   └── skills/
│       ├── todo-desk/
│       ├── semantic-commit/
│       └── ...
└── workdir/                  # AI_CABIN_WORKDIR (empty)
```

## Use Cases

### 1. Test from scratch (isolated)

```bash
./bootstrap-cabin.sh /tmp/ai-cabin-test-1
cp /tmp/ai-cabin-test-1/.envrc cabin/opencode-go/.envrc
cd cabin/opencode-go && direnv allow && make setup && make docker-up
```

### 2. New user setup (production)

```bash
./bootstrap-cabin.sh /home/user/ai-cabin-prod
cp /home/user/ai-cabin-prod/.envrc cabin/opencode-go/.envrc
# Edit .envrc to set real SCW_PROJECT_ID and OPENCODE_SERVER_PASSWORD
cd cabin/opencode-go && direnv allow && make setup && make docker-up
```

### 3. Multi-environment (dev/prod/meta)

```bash
# Dev environment
./bootstrap-cabin.sh /home/user/ai-cabin-dev dev

# Meta environment (for AI Cabin development)
./bootstrap-cabin.sh /home/user/ai-cabin-meta meta

# Copy respective .envrc to different cabins
cp /home/user/ai-cabin-dev/.envrc cabin/opencode-go-dev/.envrc
cp /home/user/ai-cabin-meta/.envrc cabin/opencode-go-meta/.envrc
```

## Common Issues

### Greywall not found

```bash
# Install greywall on host
# (depends on your setup)
```

### Greyproxy not running

```bash
# Start greyproxy on host
greyproxy serve
```

### Permission denied on desk

Check `greywall.json` allows write to `~/desk/`:
```bash
cat cabin/opencode-go/greywall.json | grep -A5 allowWrite
```

### SCW_SECRET_KEY missing

Add to `.envrc`:
```bash
export SCW_SECRET_KEY=<your-key>
```

Or ensure greyproxy is running and injecting it.

## Cleanup

```bash
# Remove test environment
rm -rf /tmp/ai-cabin-test-1

# For production, keep the environment
```

## Related

- [cabin/opencode-go/README.md](../../cabin/opencode-go/README.md) - Cabin documentation
- [desk/target-directory-structure.md](../../desk/target-directory-structure.md) - Architecture reference
