# AI-Cabin

## You're the captain, AI is just another passenger on the boat 🚢

**Constrained Docker Environment for AI Coding Agents**

AI-Cabin provides a secure, reproducible, and agent-agnostic environment for AI coding tools. Run AI agents (OpenCode, Pi.dev, or future tools) in a sandboxed Docker container with controlled access to your filesystem, network, and credentials.

**Who is it for?** Individual developers and small teams who want reproducible coding agents that run sandboxed and never expose host credentials.

**Try it without a provider:** a provider is only needed for the agent to call models — you can open a cabin's sandboxed shell (`cabin greyshell`) without one.

![AI-Cabin Concept](AI-Cabin-Concept.png)

---

## Key Features

### 🔒 Security & Isolation
- **Greywall Sandboxing**: Filesystem restrictions, seccomp profiles, capability limits
- **Greyproxy Integration**: Credential management with domain allowlisting
- **Dashboard on a host-only address**: reachable only on the Docker bridge, never routed to your LAN (no LAN exposure)
- **Docker + Greywall isolation**: Docker provides the filesystem abstraction; Greywall (bubblewrap) enforces isolation and permissions

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
- Same environment on any machine meeting the host prerequisites
- No dependency conflicts with host
- Full toolchain in container (Go, Node.js, databases, etc.) — assemble the toolchain each cabin needs via [authoring](#authoring)

---

## Quick Start

### Prerequisites

This guide targets **Linux** and assumes systemd user units, UFW, and the Docker **Compose v2 plugin** (`docker compose`).

- **Docker** + **Docker Compose v2**
- **Go** (>= 1.26) — to install the `cabin` CLI
- **Greywall** (installed on host) - [GreyhavenHQ/greywall](https://github.com/GreyhavenHQ/greywall)
  - Installed as `~/.local/bin/greywall` (the cabin build expects this exact path)
- **Greyproxy** (running on host for credential management) - [GreyhavenHQ/greyproxy](https://github.com/GreyhavenHQ/greyproxy)
  - Runs on the host with its CA certificate installed at `~/.local/share/greyproxy/ca-cert.pem` and its **bind address configured** so Docker containers can reach it (see [Greyproxy Integration](#greyproxy-integration)); the default `127.0.0.1` bind is not reachable from containers
- **Git**

### 1. Install cabin

```bash
git clone https://github.com/JulienVdG/AI-Cabin.git
cd AI-Cabin
go install ./cmd/cabin
```

Installs the `cabin` CLI to `$(go env GOPATH)/bin`. Add that directory to your `PATH` if it isn't already:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

The repo ships with the cabins `pi-go` and `opencode-go` under `cabin/`. A **cabin** is a ready-made agent environment (Dockerfile + compose + Taskfile) bundled with the CLI to build and run it.

Optionally enable shell completion — for bash, add this to your `.bashrc`:
```bash
if command -v cabin &>/dev/null; then
  source <(cabin completion bash)
fi
```

For other shells: `cabin completion <shell> --help`.

### 2. Bootstrap your environment

```bash
cabin setup
```

Zero-config bootstrap with default paths. It creates:

- a **default profile** with `AI_CABIN_HOME`, `AI_CABIN_DESK`, `AI_CABIN_WORKDIR` and your git identity
- a minimal **desk** at `AI_CABIN_DESK` (AGENTS.md, TODO, skills)
- a **workdir** (default `~/projects`)

**Want custom paths? Decide once, on the first bootstrap** — pass them as `--var`
on the same command rather than moving them after the fact:

```bash
cabin setup --var AI_CABIN_DESK=/path/to/desk --var AI_CABIN_WORKDIR=/path/to/workdir
```

Re-running `cabin setup` is safe at any time: it repairs missing pieces and
picks up low-risk additions from newer versions without overwriting your
existing desk or profile.

### 3. Register a cabin

```bash
cabin cabin add cabin/opencode-go
```

Registers `opencode-go` in the cabin registry (the name-to-directory map that lets `cabin <name> ...` resolve it, see [Documentation](#documentation)). The path is relative to the AI-Cabin clone, so run this from where you cloned in step 1.

### 4. Add your AI provider secret to Greyproxy

Open the Greyproxy UI — default bridge: `http://172.17.0.1:43080/settings#credentials`, custom pool: `http://100.64.0.1:43080/settings#credentials` — and add a
**Global Credential** with Label `SCW_SECRET_KEY` and Value your Scaleway secret
key. Greywall injects it at runtime into the agent container.

Verify Greyproxy is reachable from the host (pick the alternative matching your Docker configuration):

```bash
# default bridge:
curl http://172.17.0.1:43080/api/health
# custom pool:
curl http://100.64.0.1:43080/api/health
```

This only works if Greyproxy's **bind address** is set for Docker access (see Prerequisites) — otherwise the dashboard, credential injection, and domain allowlist are unreachable from the agent container.

### 5. Configure the profile

```bash
cabin profile set CREDENTIAL_INJECT "SCW_SECRET_KEY"
# if your Access Key is project scoped, set your project Id:
cabin profile set SCW_PROJECT_ID <your-scaleway-project-id>
```

`CREDENTIAL_INJECT` lists the Greyproxy credential labels that Greywall injects
into the agent, so the agent never sees your actual credentials.

Two other common provider vars:

- `DEFAULT_PROVIDER` — default AI provider (set in the agent's settings; omit to choose at runtime)
- `DEFAULT_MODEL` — default model for that provider (omit to use the provider's default)

### 6. Build, start, and run the cabin

The commands below assume steps 2-5 are done (a configured profile, a registered cabin, and Greyproxy reachable from Docker):

```bash
cabin build opencode-go              # build the image (also runs prepare + deps)
cabin up opencode-go                 # start the cabin in background
cd ~/projects/<your-project>         # cd into a directory inside the workdir
cabin task opencode-go opencode      # run the agent terminal UI (TUI)
```

`cabin task` (and `shell`/`greyshell`) shadow your host path inside the container and require the current directory to be inside the workdir; otherwise it fails fast with a `relpath` error. Work from a project under `~/projects` (or your `AI_CABIN_WORKDIR`), or pass `--no-relpath` to skip the check.

**Sandboxed shell** (within a running cabin):

```bash
cabin greyshell opencode-go
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
┌────────────────────────────────────────────────────┐
│                 SHARED WORKSPACE                   │
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
└────────────────────────────────────────────────────┘
```

### Available Cabins

| Cabin | Agent | Language | Use Case |
|-------|-------|----------|----------|
| `opencode-go` | OpenCode | Go | Production Go projects |
| `pi-go` | Pi.dev | Go | Alternative agent for Go |

---

## Documentation

### Setup

- `cabin setup` — Zero-config environment bootstrap. Creates a **default profile** (with `AI_CABIN_HOME`, `AI_CABIN_DESK`, `AI_CABIN_WORKDIR` and your git identity), a minimal **desk** at `AI_CABIN_DESK` (AGENTS.md, TODO, skills), and a **workdir**. Re-running is safe: it repairs missing pieces without overwriting your desk or profile. Decide custom paths once, on first run, with `--var AI_CABIN_DESK=... --var AI_CABIN_WORKDIR=...`.

### Profiles

A **profile** holds the cabin configuration (paths, credentials, provider) and is selected with `--profile` (default `default`) or `AI_CABIN_PROFILE`.

- `cabin profile init [name]` — create a profile and copy the desk skeleton (`--skeleton`, `--var`, `--force`)
- `cabin profile list` — list available profiles
- `cabin profile show` — show the active profile
- `cabin profile set <key> <value>` — set a variable on a profile
- `cabin profile use <name>` — select the active profile

The essential variables are listed under Profile Variables below.

### Cabin registry

The registry maps cabin names to their directory so `cabin <name> ...` resolves them.

- `cabin cabin add <path> [name]` — register or update a cabin
- `cabin cabin list` — list registered cabins
- `cabin cabin scan <path>` — recursively discover and register cabins under a path (e.g. `cabin cabin scan cabin/`)

### Lifecycle

- `cabin build <name>` — build the cabin image (also runs prepare + deps)
- `cabin up <name>` — start the cabin in background
- `cabin down <name>` — stop the cabin
- `cabin restart <name>` — restart the agent container
- `cabin logs <name>` — follow agent logs
- `cabin shell <name>` — get a bash shell inside the container
- `cabin greyshell <name>` — get a greywall-sandboxed shell
- `cabin ps` — list agent containers across cabins (`-a` for all, including stopped)

### Task

Runs a Taskfile target of a cabin, optionally forwarding parameters to the agent:

- `cabin task <cabin> <target> [params...]`

Examples:

```bash
cabin task opencode-go opencode    # run the OpenCode TUI
cabin task pi-go pi -c             # pass -c to the agent
```

The cabins ship with lifecycle Taskfiles that `task` can also run standalone, without the cabin CLI (the `cabin task` wrapper adds profile env, path shadowing, and registry resolution):

```bash
task opencode-go opencode
```

### Advanced: path shadowing

Launching the agent through `cabin task`, `shell`, or `greyshell` feels like running it directly on your machine: the directory you're in on the host is the directory the agent lands in inside the container, so it sees exactly the files you're working on — no copying, no path remapping. The security isn't diluted though: the agent stays confined to that sub-path of the workdir. The command fails fast if you launch it from outside the workdir, so the agent never silently starts somewhere unexpected (or at the workdir root). Pass `--no-relpath` to opt out and launch at the workdir root instead.

### Skeletons

Applies a skeleton (desk or project) by name:

- `cabin skeleton apply [desk=]desk/<skeleton> [<name>=projects/<skeleton>]`

`desk/` skeletons are copied to `AI_CABIN_DESK` — the minimal desk is already applied by `setup`/`profile init`. `projects/` skeletons scaffold a new project (e.g. `cabin skeleton apply projects/go_makefile`, passing `--attr module=...` for Go, `--force` to overwrite existing files).

### Authoring

Assembles a new cabin (Dockerfile + compose + Taskfile) from blueprints without touching an existing one:

- `cabin authoring show <dest>` — render the assembled files to stdout (non-destructive)
- `cabin authoring new <dest>` — write the assembled files (new files only; `--force` to overwrite)

Both accept `--agents pi,opencode` and `--features git-agent,go`.

### Profile Variables

The cabin reads its configuration from a **profile** (selected with `--profile`,
default `default`). Set variables per profile with `cabin profile set <key> <value>`
and view them with `cabin profile show`. The essential ones:

| Variable | Purpose | Default |
|----------|---------|---------|
| `AI_CABIN_HOME` | Host dir for agent data (bind-mount target) | `$HOME` |
| `AI_CABIN_DESK` | Desk dir (agent instructions, skills, TODO) | `<AI_CABIN_HOME>/Documents/desk` |
| `AI_CABIN_WORKDIR` | Host workdir (git repos) | `<AI_CABIN_HOME>/projects` |
| `CONTAINER_WORKDIR` | Container-side workdir (advanced) | `AI_CABIN_WORKDIR` |
| `GIT_AGENT_NAME` | Git name for agent commits | `AI Agent + <your git user.name>` |
| `GIT_AGENT_EMAIL` | Git email for agent commits | your git `user.email` |
| `CREDENTIAL_INJECT` | Greyproxy credential labels Greywall injects (CSV) | _(none)_ |
| `CREDENTIAL_IGNORE` | Env vars Greywall must not treat as credentials (CSV) | _(none)_ |
| `SCW_PROJECT_ID`, `DEFAULT_PROVIDER`, `DEFAULT_MODEL` | Provider-specific (see Quick Start) | _(none)_ |

---

## Greyproxy Integration

Greyproxy handles credentials and network access for the agent container:

- **Automatic Injection**: `SCW_SECRET_KEY`, API tokens
- **Domain Allowlist**: Only approved domains accessible
- **Audit Log**: All requests logged for review
- **Dashboard**: default bridge `http://172.17.0.1:43080`, custom pool `http://100.64.0.1:43080` (host only)

Greywall routes HTTP/SOCKS and DNS traffic through Greyproxy, which serves four endpoints on the host:

| Service | Port | Protocol |
|---------|------|----------|
| HTTP Proxy | `43051` | TCP |
| SOCKS5 Proxy | `43052` | TCP |
| DNS Proxy | `43053` | TCP + UDP |
| Dashboard / API | `43080` | HTTP |

### Prerequisites

- Greyproxy running on the host
- CA certificate at `~/.local/share/greyproxy/ca-cert.pem`

### Bind address

Since v0.4.4, Greyproxy binds every service to `127.0.0.1` by default, which keeps it local-only but **unreachable from Docker containers**. To expose it to the agent containers you must bind it to a Docker-reachable interface with `--host <ip>` (IP literal only; see the [CLI reference](https://github.com/GreyhavenHQ/greyproxy/blob/main/docs/cli-reference.md)).

Pick the host interface Docker containers can reach - the Docker bridge gateway:

1. Inspect Docker's networking. If `default-address-pools` is set in `/etc/docker/daemon.json`, the bridge is created from that pool and its gateway is the pool's first address:

   ```bash
   cat /etc/docker/daemon.json
   jq -r '."default-address-pools"[].base' < /etc/docker/daemon.json
   ```

2. Otherwise Docker uses the default bridge `172.17.0.0/16` whose gateway is `172.17.0.1`.

For example, a host using the `100.64.0.0/15` pool binds to `100.64.0.1`.

Apply the bind by editing the systemd user unit on Linux (`~/.config/systemd/user/greyproxy.service`) and reloading the service:

```diff
- ExecStart=~/.local/bin/greyproxy "serve"
+ ExecStart=~/.local/bin/greyproxy "serve" --host 100.64.0.1
```

```bash
systemctl --user daemon-reload
systemctl --user restart greyproxy
```

> Prefer binding only the Docker-reachable address. Passing `--host 0.0.0.0` binds every interface; Greyproxy logs a warning and the dashboard/proxies become reachable from your LAN.

### Firewall (optional)

With the default `127.0.0.1` bind, no firewall rule is needed. Add one only when you have bound a non-loopback interface, to let Docker containers through while keeping the ports closed to the rest of the network.

The rules below assume UFW's default incoming policy is `deny`, so anything not explicitly allowed (including your LAN) is blocked. Verify with:

```bash
sudo ufw status verbose
# look for: Default: deny (incoming)
```

Adapt the subnet to your Docker configuration:

1. Allow the Docker subnet to reach the Greyproxy ports.

   Default subnet (`172.17.0.0/16`):

   ```bash
   sudo ufw allow from 172.17.0.0/16 to any port 43051,43052,43053,43080 proto tcp comment "greyproxy docker"
   sudo ufw allow from 172.17.0.0/16 to any port 43053 proto udp comment "greyproxy dns docker"
   ```

   Custom subnet (e.g. `100.64.0.0/15`):

   ```bash
   sudo ufw allow from 100.64.0.0/15 to any port 43051,43052,43053,43080 proto tcp comment "greyproxy docker"
   sudo ufw allow from 100.64.0.0/15 to any port 43053 proto udp comment "greyproxy dns docker"
   ```

2. Reload UFW:

   ```bash
   sudo ufw reload
   ```

### Verification

Greyproxy is reachable on the interface you bound in *Bind address* — not on loopback — so test the `api/health` endpoint on that address. The two local-access commands are alternatives matching the subnet cases above; pick the one that matches your Docker configuration:

```bash
# Local access (should work) - default bridge:
curl http://172.17.0.1:43080/api/health
# Local access (should work) - custom pool:
curl http://100.64.0.1:43080/api/health

# Docker access (should work)
docker exec <container> curl http://172.17.0.1:43080/api/health
# or custom pool:
docker exec <container> curl http://100.64.0.1:43080/api/health

# External access (should fail)
curl --connect-timeout 2 http://<your-IP>:43080/api/health
```

---

## Troubleshooting

### First run

**The build fails with `install: cannot stat ~/.local/bin/greywall`** — the cabin image copies Greywall and the CA from exact host paths during `build`. Install Greywall at `~/.local/bin/greywall` and trust the CA at `~/.local/share/greyproxy/ca-cert.pem` (Prerequisites), then rebuild.

**The agent runs but can't reach the model / credentials aren't injected** — greyproxy is still bound to `127.0.0.1`, which is unreachable from the container. Set its **bind address** to the Docker bridge (see [Greyproxy Integration](#greyproxy-integration)) and rebuild/restart.

**`cabin task` exits with a `relpath` error** — the current directory is outside the workdir. `cd` into a project under `~/projects` (or your `AI_CABIN_WORKDIR`) before running the agent, or pass `--no-relpath`.

### Greyproxy Desktop Notifications Not Appearing

**Issue:** No desktop notification when a request is pending.

**Cause:** `notify-send` (from `libnotify`) is not installed on the host. Greyproxy detects the notification backend **only at startup** and disables notifications silently. Check Greyproxy's dashboard (Settings) — it shows "No notification backend available / Notification tool not found".

**Solution:**
```bash
sudo apt install libnotify-bin      # Debian/Ubuntu (or: sudo dnf install libnotify-utils)
greyproxy service restart           # restart so greyproxy re-detects the backend
```

**Verify:** after restart, Settings (or the API below) reports `notify-send` / `available: true`:

```bash
# default bridge:
curl -s http://172.17.0.1:43080/api/settings | jq '.notifications'
# custom pool:
curl -s http://100.64.0.1:43080/api/settings | jq '.notifications'
```

**Known limitation:** with the v0.4.4 setup above (Greyproxy bound to a non-loopback address), the notification's **action** link ("Open Dashboard") does **not** work: Greyproxy builds it with a hardcoded `http://localhost:43080/pending?...`, which is unreachable when greyproxy binds a Docker bridge address instead. The notification itself is delivered, but clicking it opens nothing. This is a Greyproxy upstream limitation, tracked to be contributed/fixed there.

### Ubuntu AppArmor Blocks Greywall

**Issue:** Bubblewrap cannot run (or cannot create the TUN device) due to
AppArmor user-namespace restrictions. The exact failure depends on the Ubuntu
version:

- **24.04 (kernel 6.8):** the unprivileged user namespace is blocked, so
  Bubblewrap fails with `bwrap: setting up uid map: Permission denied` and
  Greywall silently falls back to env-var proxying (no TUN).
- **26.04 (kernel 7.0):** the user namespace is created, but a dedicated
  AppArmor profile (`/etc/apparmor.d/bwrap-userns-restrict`) is stacked onto
  Bubblewrap and **strips all capabilities from its children**. The TUN
  creation then fails with `RTNETLINK answers: Operation not permitted` /
  `ioctl(TUNSETIFF): Operation not permitted` (audit shows `profile="unpriv_bwrap"
  ... capname="net_admin"` denied).

**Solution — 24.04:** allow unprivileged user namespaces:
```bash
sudo sysctl kernel.apparmor_restrict_unprivileged_userns=0
```

**Solution — 26.04:** the sysctl above is **not sufficient** on 26.04: it does
not remove the `bwrap-userns-restrict` profile, which keeps denying
capabilities (`net_admin`, `sys_admin`) no matter the sysctl value. Disable
that profile instead (run once, persists across reboots):
```bash
sudo apt install apparmor-utils
sudo aa-disable /etc/apparmor.d/bwrap-userns-restrict
```

**Verify:** after the fix, a sandboxed network command must reach the TUN setup
without `Operation not permitted` (you still need Greyproxy running for actual
network traffic).

---

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

---

## Contributing

Contributions welcome! Please read our development guide and open an issue before submitting PRs.

*This repo was written with an AI agent in the passenger seat, carefully reviewed by a human captain.* 🚢

---

**You're the captain. AI is just another passenger. Stay in control. 🚢**
