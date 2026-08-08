# AI-Cabin

## You're the captain, AI is just another passenger on the boat 🚢

**Constrained Docker Environment for AI Coding Agents**

AI-Cabin provides a secure, reproducible, and agent-agnostic environment for AI coding tools. Run AI agents (OpenCode, Pi.dev, or future tools) in a sandboxed Docker container with controlled access to your filesystem, network, and credentials.

![AI-Cabin Concept](AI-Cabin-Concept.png)

---

## Key Features

### 🔒 Security & Isolation
- **Greywall Sandboxing**: Filesystem restrictions, seccomp profiles, capability limits
- **Greyproxy Integration**: Credential management with domain allowlisting
- **Localhost-only Dashboard**: No LAN exposure for web interfaces
- **Docker Isolation**: Complete environment separation from host

### 🤖 Multi-Agent Support
- Works with **OpenCode**, **Pi.dev**, and future AI coding tools
- Same workflow and conventions across all agents
- Agent-agnostic skills system for consistent behavior

### 🏗️ Collaborative Workspace
- **Shared Files**: User and AI work together in real-time
- **Persistent Sessions**: Continue work across multiple runs
- **Clean Separation**: 
  - `desk/` → Cross-project organization (tasks, skills, docs)
  - `workdir/` → Project code and git repositories

### 🔄 Reproducibility
- Same environment on any machine
- No dependency conflicts with host
- Full toolchain in container (Go, Node.js, databases, etc.)

---

## Quick Start

### Prerequisites

- **Docker** + **Docker Compose**
- **Greywall** (installed on host) - [GreyhavenHQ/greywall](https://github.com/GreyhavenHQ/greywall)
- **Greyproxy** (running on host for credential management) - [GreyhavenHQ/greyproxy](https://github.com/GreyhavenHQ/greyproxy)
- **Git**
- **direnv** (optional, recommended for environment management)
- **Scaleway access key** (for AI model access)

### 1. Bootstrap Your Environment

```bash
# Clone AI-Cabin
git clone https://github.com/JulienVdG/AI-Cabin.git
cd AI-Cabin

# Bootstrap a new environment (creates desk, workdir, home directories)
./skeletons/bin/bootstrap-cabin.sh /home/user/ai-cabin-prod
```

This creates:
```
/home/user/ai-cabin-prod/
├── .envrc              # Environment variables
├── home/               # AI_CABIN_HOME (agent data)
├── desk/               # AI_CABIN_DESK (skills, tasks, docs)
└── workdir/            # AI_CABIN_WORKDIR (git repos)
```

### 2. Configure and Start the Cabin

Required variables:
```bash
export AI_CABIN_HOME=/home/user/ai-cabin-prod/home/
export AI_CABIN_DESK=/home/user/ai-cabin-prod/desk/
export AI_CABIN_WORKDIR=/home/user/ai-cabin-prod/workdir/
export SCW_PROJECT_ID=your-scaleway-project-id
export OPENCODE_SERVER_PASSWORD=your-password  # For OpenCode WebUI
export GIT_AGENT_NAME="AI Agent + $(git config --global user.name)"  # Git name for agent
export GIT_AGENT_EMAIL=$(git config --global user.email)  # Git email for agent
```

**Option 1: Source .envrc manually (without direnv)**

```bash
# Source the environment before each session
source ~/ai-cabin-prod/.envrc
cd cabin/opencode-go

# Verify variables are set
env | grep AI_CABIN

# Setup and start
make setup
make docker-up
```

**Option 2: Use direnv (automatic)**

```bash
# Copy .envrc to your cabin
cp /home/user/ai-cabin-prod/.envrc cabin/opencode-go/.envrc

# Allow direnv (loads .envrc automatically when entering directory)
cd cabin/opencode-go
direnv allow

# Setup and start (variables loaded automatically)
make setup
make docker-up
```

### 3. Access the Agent

**Web Interface:**
- Navigate to http://localhost:9090
- Enter your `OPENCODE_SERVER_PASSWORD`

**TUI (Terminal UI):**
```bash
make opencode
```

**Sandboxed Shell:**
```bash
make docker-greyshell
```

---

## Architecture

### High-Level Overview

```
┌─────────────────────────────────────────────────┐
│                    HOST                         │
│  ┌─────────────────────────────────────────┐    │
│  │           AI-Cabin Container            │    │
│  │  ┌───────────────────────────────────┐  │    │
│  │  │ Greywall (Sandbox)                │  │    │
│  │  │  ┌──────────────┐                 │  │    │
│  │  │  │ AI Agent     │                 │  │    │
│  │  │  │ (OpenCode/   │ ◄──────────┐    │  │    │
│  │  │  │  Pi.dev)     │            │    │  │    │
│  │  │  └──────────────┘            │    │  │    │
│  │  └──────────────────────────────│────┘  │    │
│  └─────────────────────────────────│───────┘    │
│         ▲                          │            │
│         │ Bind mounts              │ Network    │
│  ┌──────┴──────────────┐  ┌────────▼─────────┐  │
│  │ Host Directories    │  │ Greyproxy        │  │
│  │                     │  │ (HTTP/SOCKS5)    │  │
│  └─────────────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────┘
```

### Workspace Structure

```
┌─────────────────────────────────────────────────────┐
│                 SHARED WORKSPACE                    │
│  ┌──────────────────┐    ┌──────────────────┐      │
│  │      desk/       │    │    workdir/      │      │
│  ├──────────────────┤    ├──────────────────┤      │
│  │ - AGENTS.md      │    │ - product-a/     │      │
│  │ - TODO.md        │    │ - product-b/     │      │
│  │ - skills/        │    │ - personal/      │      │
│  │ - retro.md       │    │                  │      │
│  │                  │    │ (Git repos)      │      │
│  └──────────────────┘    └──────────────────┘      │
│        ▲                         ▲                 │
│        │ Cross-project           │ Project code    │
│        │ organization            │ & docs          │
└─────────────────────────────────────────────────────┘
```

### Available Cabins

| Cabin | Agent | Language | Use Case |
|-------|-------|----------|----------|
| `opencode-go` | OpenCode | Go | Production Go projects |
| `pi-go` | Pi.dev | Go | Alternative agent for Go |

---

## Commands Reference

### Cabin Management

| Command | Description |
|---------|-------------|
| `make setup` | Setup environment (directories + config) |
| `make docker-up` | Start cabin in background |
| `make docker-down` | Stop cabin |
| `make docker-build` | Rebuild container |
| `make docker-restart-agent` | Restart agent container |

### Development

| Command | Description |
|---------|-------------|
| `make opencode` | Continue OpenCode session (TUI) |
| `make docker-shell` | Get bash shell inside container |
| `make docker-greyshell` | Get greywall-sandboxed shell |
| `make docker-logs` | Follow agent logs |

### Environment Variables

| Variable | Description | Example | Default |
|----------|-------------|---------|----------|
| `AI_CABIN_HOME` | Host home for agent data | `/home/user/ai-cabin/` | _Required_ |
| `AI_CABIN_DESK` | Shared desk directory | `/home/user/ai-cabin/desk/` | _Required_ |
| `AI_CABIN_WORKDIR` | Git repositories | `/home/user/workdir/` | _Required_ |
| `SCW_PROJECT_ID` | Scaleway project ID | `12345678-...` | _Required_ |
| `GIT_AGENT_NAME` | Git name for agent commits | `AI Agent + John Doe` | `AI Agent` |
| `GIT_AGENT_EMAIL` | Git email for agent commits | `user@example.com` | `ai-agent@vdg.name` |
| `CONTAINER_WORKDIR` | Container workdir path (advanced) | `/workspace/` | `${AI_CABIN_WORKDIR}` |

---

## Greyproxy Integration

Greyproxy handles credentials and network access:

- **Automatic Injection**: `SCW_SECRET_KEY`, API tokens
- **Domain Allowlist**: Only approved domains accessible
- **Audit Log**: All requests logged for review
- **Dashboard**: http://localhost:43080 (host only)

**Requirements:**
- Greyproxy running on host
- CA certificate at `~/.local/share/greyproxy/ca-cert.pem`

**Update with Greyproxy v0.4.4:**

Starting v0.4.4 greyproxy no longer bind 0.0.0.0 by default but 127.0.0.1 which makes it unreachable to the docker networks.

To make it bind another interface add the `-host ip-to-bind` parameter ([greyproxy doc](https://github.com/GreyhavenHQ/greyproxy/blob/main/docs/cli-reference.md#greyproxy-install)).

I strongly recommend to use the docker network host address (172.17.0.1 by default, 100.64.0.1 for me) see next section for details.

On linux edit `~/.config/systemd/user/greyproxy.service` and add the parameter
```diff
- ExecStart=/home/jvdg/.local/bin/greyproxy "serve"
+ ExecStart=/home/jvdg/.local/bin/greyproxy "serve" -host 100.64.0.1
```

**Firewall Configuration (Recommended for Production):**

By default, greyproxy listens on `0.0.0.0` (all interfaces). To secure it from LAN access while preserving Docker access:

*1. Identify Docker subnet:*

- Check Docker configuration
    ```bash
    cat /etc/docker/daemon.json
    ```
- Extract subnet (if customized in daemon.json)
    ```bash
    jq -r '."default-address-pools"[].base' < /etc/docker/daemon.json
    ```
- Default subnet (if no daemon.json config): 172.17.0.0/16


*2. Configure UFW (Ubuntu/Debian):*

- Allow Docker (adapt subnet to your config)
  - Custom subnet (e.g., 100.64.0.0/15):
    ```bash
    sudo ufw allow from 100.64.0.0/15 to any port 43051,43052,43053,43080 proto tcp comment "greyproxy docker"
    sudo ufw allow from 100.64.0.0/15 to any port 43053 proto udp comment "greyproxy dns docker"
    ```
  - *OR* default subnet (172.17.0.0/16):
    ```bash
    sudo ufw allow from 172.17.0.0/16 to any port 43051,43052,43053,43080 proto tcp comment "greyproxy docker"
    sudo ufw allow from 172.17.0.0/16 to any port 43053 proto udp comment "greyproxy dns docker"
    ```
- Allow localhost and block the rest (optional - UFW should do this by default)
    ```bash
    sudo ufw allow from 127.0.0.1 to any port 43051,43052,43053,43080 proto tcp comment "greyproxy localhost"
    sudo ufw deny from any to any port 43051,43052,43053,43080 proto tcp comment "greyproxy deny external"
    ```
- Reload UFW
    ```bash
    sudo ufw reload
    ```
**Verification:**

```bash
# Test local access (should work)
curl http://127.0.0.1:43080/api/health

# Test Docker access (should work)
docker exec <container> curl http://host.docker.internal:43080/api/health

# Test external access (should be blocked)
curl --connect-timeout 2 http://<your-IP>:43080/api/health
```

---

## Documentation

- **[Bootstrap Script](skeletons/bin/README.md)** - Environment setup guide
- **[Cabin Docs](cabin/opencode-go/README.md)** - Detailed cabin configuration

---

## Troubleshooting

### Ubuntu AppArmor Blocks Greywall

**Issue:** Bubblewrap cannot mount tun due to AppArmor restrictions.

**Solution:**
```bash
sudo sysctl kernel.apparmor_restrict_unprivileged_userns=0
```

**Note:** A PPA-based solution may also be available.

---

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

---

## Contributing

Contributions welcome! Please read our development guide and open an issue before submitting PRs.

---

**You're the captain. AI is just another passenger. Stay in control. 🚢**
