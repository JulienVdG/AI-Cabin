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

## Install modes

`cabin` installs two ways. The Quick Start below follows the **clone** mode
because it unlocks the full feature set.

**`go install` — minimal**

- `go install github.com/JulienVdG/AI-Cabin/cmd/cabin@latest`
- Single binary, no repo checkout.
- Gives: `cabin setup` (minimal desk) and `cabin authoring` to build a cabin —
  [Cabin Writing Guide](docs/CABIN-WRITING-GUIDE.md).
- Does not give: reference cabins to `scan`, rich FR/EN desks, project
  skeletons (these live in the repo only).

**Clone — full (recommended)**

- `git clone https://github.com/JulienVdG/AI-Cabin.git` then
  `go install ./cmd/cabin`
- Register the clone as a **layer** at first setup (see [Layers](#layers)):
  `cabin setup --var AI_CABIN_LAYER_DIRS=$PWD/AI-Cabin` — unlocking rich FR/EN
  desks and project skeletons for that profile.

**Note on reproducibility:** pin the clone if you need the binary and the
clone to stay in sync.

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

### 3. Register the reference cabins

```bash
cabin scan cabin/
```

`cabin scan <path>` walks the directory and registers every valid cabin it finds — here the two reference cabins `opencode-go` and `pi-go` — so `cabin use` and `--cabin <name>` can select them by name (see [Documentation](#documentation)). The path is relative to the AI-Cabin clone, so run this from where you cloned in step 1. To register a single cabin instead, use `cabin add <path> [name]`.

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
cabin use opencode-go                # make opencode-go the current cabin
cabin build                          # build the image (also runs prepare + deps)
cabin up                             # start the cabin in background
cd ~/projects/<your-project>         # cd into a directory inside the workdir
cabin task opencode                  # run the agent terminal UI (TUI)
```

`cabin task` (and `shell`/`greyshell`) shadow your host path inside the container and require the current directory to be inside the workdir; otherwise it fails fast with a `relpath` error. Work from a project under `~/projects` (or your `AI_CABIN_WORKDIR`), or pass `--no-relpath` to skip the check.

**Sandboxed shell** (within a running cabin):

```bash
cabin greyshell
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

### Desk

The **desk** is the shared, cross-project side of your workspace. It holds the
instructions and knowledge every cabin relies on: `AGENTS.md` (agent
instructions), `TODO.md` (task tracking), and `skills/` (reusable workflow
guides). It lives outside any single project, so conventions apply across all
your work. The desk is bind-mounted into every cabin (read-write): it is the
source of the `AGENTS.md` and skills injected into each agent's config where
the software expects them, and the canonical copy the agent updates during
retros. `cabin setup` and `cabin profile init` create a minimal desk at
`AI_CABIN_DESK` (default `~/Documents/desk`), and you can seed it from a **desk
skeleton** (see [Skeletons](#skeletons)).

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

A **profile** gathers everything the cabins read once per user context:

- where your **desk** and workdir live (`AI_CABIN_DESK`, `AI_CABIN_WORKDIR`, `AI_CABIN_HOME`) — a profile therefore selects which desk the cabins use;
- the configuration **shared across cabins** (credentials, provider, git identity);
- **variables injected into the environment** of a cabin's commands: when you run a task, they land in the `task` process env, which `docker compose` reads as `${VAR}` at launch.

The active profile is selected with `--profile` (default: the active profile, initially `default`) or `AI_CABIN_PROFILE`.

- `cabin profile init [name]` — create a profile and copy the desk skeleton (`--skeleton`, `--var`, `--force`)
- `cabin profile list` — list available profiles
- `cabin profile show` — show the active profile
- `cabin profile set <key> <value>` — set a variable on a profile
- `cabin profile use <name>` — select the active profile

Value resolution, highest to lowest: `--var KEY=VAL` (repeatable global flag), environment variables, the profile file, then built-in defaults. Since environment variables outrank the profile file, a profile variable can be silently shadowed by a same-named shell variable; `cabin profile show` warns when that happens (it prints each shadowed variable with its environment value), so it doubles as a debug tool for precedence surprises.

#### Env-var mode (`cabin setenv`)

There is also a manual, **advanced** mode: `cabin setenv <shell> [<profile>]` prints the resolved profile variables in your shell's syntax (`bash`, `zsh`, or `fish`), typically loaded from a direnv `.envrc` or your shell rc — for running `task` standalone in a cabin directory. It exports `AI_CABIN_PROFILE`, so the standalone `task` path still selects the right profile.

You normally never need it: `cabin task` / `cabin up` already set the resolved variables on the command itself. See `cabin setenv <shell> --help` for the exact source/eval idioms.

Precedence is unchanged, and `cabin profile show` warns when a profile variable is shadowed by one of these exported variables.

The essential variables are listed under Profile Variables below.

### Cabin registry

The registry maps cabin names to their directory so `cabin use` and `--cabin <name>` select a cabin by name.

- `cabin add <path> [name]` — register or update a cabin
- `cabin list` — list registered cabins
- `cabin scan <path>` — recursively discover and register cabins under a path (e.g. `cabin scan cabin/`)

### Current cabin

Cabin-scoped commands (`build`, `up`, `down`, `restart`, `logs`, `shell`, `greyshell`, `task`) target a **current cabin**, so you never repeat a name. Set it once per profile with `cabin use` (it is sugar for `cabin profile set AI_CABIN_CURRENT_CABIN <name>`; the current cabin is a profile variable, so each profile has its own):

```bash
cabin use opencode-go    # make opencode-go the current cabin of the active profile
cabin up                 # operates opencode-go
```

Override it for a single command with `--cabin <name>` (before any positional). Selection order:

`--cabin <name>` > `AI_CABIN_CURRENT_CABIN` env > current cabin of the active profile

Global flags (`--cabin`, `--profile`, `--var`) always come before positional arguments; on `cabin task`, everything after the first positional belongs to the task. At `cabin profile init`, seed the current cabin with `--var AI_CABIN_CURRENT_CABIN=<name>`.

### Lifecycle

The cabin commands below operate on the current cabin; pass `--cabin <name>` (before any positional) to target another (`cabin ps` lists all containers and takes no cabin).

- `cabin build` — build the current cabin's image (also runs prepare + deps)
- `cabin up` — start the current cabin in background
- `cabin down` — stop the current cabin
- `cabin restart` — restart the current cabin's container
- `cabin logs` — follow the current cabin's logs
- `cabin shell` — get a bash shell inside the current cabin's container
- `cabin greyshell` — get a greywall-sandboxed shell in the current cabin
- `cabin ps` — list agent containers across cabins (`-a` for all, including stopped)

### Task

Runs a Taskfile target of the current cabin (or the one given by `--cabin`), optionally forwarding parameters to the agent:

- `cabin task <target> [params...]`

Examples:

```bash
cabin task opencode    # run the OpenCode TUI of the current cabin
cabin --cabin pi-go task pi -c    # pass -c to the agent of pi-go
```

The cabins ship with lifecycle Taskfiles that `task` can also run standalone, without the cabin CLI (the `cabin task` wrapper adds profile env, path shadowing, and registry resolution):

```bash
task opencode-go opencode
```

### Advanced: path shadowing

Launching the agent through `cabin task`, `shell`, or `greyshell` feels like running it directly on your machine: the directory you're in on the host is the directory the agent lands in inside the container, so it sees exactly the files you're working on — no copying, no path remapping. The security isn't diluted though: the agent stays confined to that sub-path of the workdir. The command fails fast if you launch it from outside the workdir, so the agent never silently starts somewhere unexpected (or at the workdir root). Pass `--no-relpath` to opt out and launch at the workdir root instead.

### Skeletons

Skeletons are ready-made template sets you apply to scaffold a **desk** or
a **project**:

- **`desk/` skeletons** seed an `AI_CABIN_DESK` — the minimal desk is already
  applied by `setup`/`profile init`; apply another to replace it.
- **`projects/` skeletons** scaffold a new project (e.g.
  `cabin skeleton apply projects/go_makefile`, passing `--attr module=...` for
  Go; `--force` to overwrite existing files).

Apply one by name:

- `cabin skeleton apply [desk=]desk/<skeleton> [<name>=projects/<skeleton>]`

### Layers

A **layer** is a self-contained override root — a git repo or a directory —
carrying everything an organization or user needs to redirect AI-Cabin
defaults to their context (richer desks, project skeletons, profile defaults),
activated by a single var. A layer root mirrors the embedded tree:

- `fragments/` — fragment bundles, prepended to the fallback chain (above the
  cabin-local and embedded layers): the usual per-file override by priority
  across the whole chain (earlier dir wins per file, later layers still
  contribute their unshadowed files);
- `skeletons/` — desk/project skeletons, added the same way: each
  `<layer>/skeletons` dir joins the catalogue as a per-file priority layer;
- `layer.yaml` — optional; a `vars:` block. Only the **first** layer with a
  `layer.yaml` (in `AI_CABIN_LAYER_DIRS` order) contributes its vars — a
  file-level first-wins, not a per-key merge.

**Activation is per profile.** `AI_CABIN_LAYER_DIRS` is a profile var,
persisted when set: pass it via `--var` or an exported env var at
`cabin setup`/`cabin profile init`, or `cabin profile set AI_CABIN_LAYER_DIRS=...`.
The first active layer's `layer.yaml`
`vars:` are then persisted as profile defaults, ranked below the profile's own
keys.

A bare fragments root (`AI_CABIN_FRAGMENTS_DIRS`) keeps working unchanged: a
layer only adds a richer root type and does not break the classic dirs vars.

### Authoring

Assembles a new cabin (Dockerfile + compose + Taskfile) from blueprints without touching an existing one:

- `cabin authoring show <dest>` — render the assembled files to stdout (non-destructive)
- `cabin authoring new <dest>` — write the assembled files (new files only; `--force` to overwrite)

Both accept `--agents pi,opencode` and `--features git-agent,go`.

### Profile Variables

The cabin reads its configuration from a **profile** (selected with `--profile`, defaulting to the active profile, initially `default`). Set variables per profile with `cabin profile set <key> <value>`
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

**Changing the base directories** — `AI_CABIN_DESK`, `AI_CABIN_WORKDIR`, and
`AI_CABIN_HOME` point at directories that already exist. Changing one of them
(after `cabin setup`) updates the variable but does **not** move or populate the
new location — you handle that yourself:

- **Move or copy** the existing content to the new path. For the **workdir**
  that is all there is to it (it is just where your projects live; no
  regeneration exists).
- For a **desk**, instead of copying you can **regenerate** it: `cabin profile
  init <name> --force` re-copies the desk skeleton to the profile's
  `AI_CABIN_DESK`, or re-apply any skeleton with `cabin skeleton apply
  desk/<name>` (`--force` to overwrite).

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
