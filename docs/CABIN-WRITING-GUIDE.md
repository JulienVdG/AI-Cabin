# Cabin Writing Guide

This guide explains how to equip a project with an **AI-Cabin**. A cabin builds
a container image, bind-mounts your desk, your workdir and your agent data, and
runs an AI coding agent inside a greywall sandbox. The agent works on your
existing code in place — inside the container, against the files mounted from
your host.

The guide is generic and works for any kind of project. Two companion tutorials
walk through concrete cases end to end:

- [`examples/tuto-1/`](../examples/tuto-1/) — one agent, no existing Dockerfile.
- [`examples/tuto-2/`](../examples/tuto-2/) — two agents, an existing Dockerfile
  and an existing compose with an auxiliary service.

Each tutorial ships a `before/` starting point (copy it to try the steps) and an
`after/` solution (the expected result).

---

## Before you start

You need a working environment first:

1. `cabin setup` — bootstraps the default profile (`AI_CABIN_HOME`,
   `AI_CABIN_DESK`, `AI_CABIN_WORKDIR`), a minimal desk, and a workdir.
2. **Greyproxy** running and reachable from Docker: add a credential label for
   your provider (e.g. `SCW_SECRET_KEY`) and bind Greyproxy to an address the
   container network can reach (see [Greyproxy
   Integration](../README.md#greyproxy-integration) for setup).
3. Your project inside the workdir (`AI_CABIN_WORKDIR`), so `cabin task` can
   shadow your host path into the container (path shadowing).

See the [README Quick Start](../README.md#quick-start) for the full setup.

---

## Step 1 — Choose your agents and features

AI-Cabin ships with two agents, both as **feature bundles** that a cabin
selects from. A feature bundle is a cohesive, reusable package: it brings the
wrapper, the install steps, the greywall profile and the agent configs for
one agent or feature, so selecting it gets that piece working end to end.
More agents are added the same way (a bundle per agent).

| Agent | What it runs |
|-------|--------------|
| `pi` | pi.dev — terminal TUI, session-based |
| `opencode` | OpenCode — terminal TUI and web UI at `http://localhost:9090` |

A cabin can enable one or both. With both, each agent gets its own wrapper and
config and they share the same sandboxed workspace — useful for comparing
agents or running a subagent pattern.

You also choose the **features** your cabin needs:

| Feature | What it adds |
|---------|--------------|
| `git-agent` | Git identity hook for agent commits (`GIT_AGENT_*` env) |
| `go` | Go toolchain PATH + shared GOPATH mounts (for a Go project) |
| `port-forward` | Reach auxiliary compose services from inside the sandbox |

The reference cabins in `cabin/` (`pi-go`, `opencode-go`) are ready-made
examples of these choices.

---

## Step 2 — Scaffold with `cabin authoring`

`cabin authoring` assembles the cabin files (Dockerfile + compose + Taskfile)
for you from your agent/feature selection. Pass the selection as flags:

```bash
cabin authoring new path/to/project --agents pi --features go
```

Preview the assembly without writing anything first:

```bash
cabin authoring show path/to/project --agents pi --features go
```

`show` renders three files to stdout; `new` writes them to disk:

- `ai-cabin.Dockerfile` — the agent image (base image, package installs,
  greywall + CA, the `useradd` to `ai_agent`, agent install steps, mounts).
- `docker-compose.yml` — the `agent` service (build context, mounts, env,
  `privileged`, `extra_hosts`, per-agent ports).
- `Taskfile.yml` — the cabin metadata header plus the `setup`, `up`, `down`,
  `info`, `help` and per-agent `pi`/`opencode` targets.

`new` is **non-destructive**: it writes only files that do not exist yet, so
existing files are preserved (see Step 4/5). Use `--force` to overwrite the
generated files.

---

## Step 3 — The `ai-cabin:` header

`cabin authoring` puts the selection into a header at the top of the
`Taskfile.yml`. The header is the **source of truth** for the cabin: the CLI
resolves the active bundles from it at build and runtime. Generated for
`--agents pi --features go`, it looks like:

```yaml
# Taskfile.yml
ai-cabin:
  agents: [pi]
  features: [go]
```

Edit it afterwards to adjust the cabin:

- change `agents` / add `features` (e.g. `git-agent`, `port-forward`);
- give the cabin a name (`cabin: myproject`) — otherwise the directory base
  name is used;
- carry per-feature attrs, e.g. the forwarded service for `port-forward`
  (`port-forward: {port: 5432, host: postgres}`, see Step 5), which flags
  cannot express.

---

## Step 4 — Adapt the image to your project

`cabin authoring` produces a working skeleton; adapting it to your project is
usually one or two edits. There are two cases.

### Case A — No existing Dockerfile (bring the toolchain)

When your project has no image yet, start from a base image and install the
toolchain in the cabin:

```yaml
# ai-cabin.Dockerfile
FROM ubuntu:24.04
# ... the generated "apt install + COPY .deps/ + RUN install.sh" block ...
```

The bundles materialize into `.deps/` at build and the numbered `install.sh`
steps install greywall, the agent, and the toolchain. On a bare `ubuntu` base
you add the project's runtime/toolchain packages (e.g. `python3`, `node`) to
the `apt-get install` list; the bundles' `install.d/*` steps install the agent.

### Case B — Existing Dockerfile (keep your base)

When your project already builds an image (a Dockerfile that produces your app,
or an image that already embeds the toolchain), **keep your base** and add the
cabin layers. Change the generated `FROM` to yours and keep the rest (the
greywall + CA `COPY`, the entrypoint, `useradd ai_agent`, the mount-point
`mkdir`) as-is:

```dockerfile
# Generated by cabin authoring. Adapt FROM to your project base image.
FROM your-project-base:latest
```

**Non-Debian bases.** The bundle install steps and the greyproxy CA trust
assume a **Debian-based** base (`apt-get` + `update-ca-certificates`). On a
non-Debian base (`apk add` / `dnf install` + `update-ca-trust` on
RedHat/Fedora), adapt two things:

1. the generated `RUN apt-get install` block — replace it with your distro's
   package command and the same package list;
2. the greyproxy CA installation — the bundle's `install.d/10-greywall.sh`
   copies the CA to `/usr/local/share/ca-certificates/` then runs
   `update-ca-certificates`. Override that fragment for your distro (cabin-local
   `fragments/base/deps/install.d/10-greywall.sh`) so it installs the CA into
   the right trust store and refreshes it (`update-ca-trust` on RedHat/Fedora),
   so greyproxy's HTTPS inspection is trusted by the agent.

### Match the container user to your host uid

The compose bind-mounts your host dirs (cache, go, desk, workdir) into the
container, and they keep their **host ownership** (the uid of your host user).
For the agent to read and write them, the container user the agent runs as must
have the **same uid** as the host user that owns those dirs — otherwise writes
fail with a permission error (e.g. `go` cannot create its build cache).

`cabin authoring` generates a dedicated `ai_agent` user (uid 1000), which
matches a host user with uid 1000 out of the box. Adapt it when your base image
already ships a default user, or when your host uid differs:

- keep the base image's default user and set `CONTAINER_HOME` to its home. The
  `ubuntu` images ship an `ubuntu` user (uid 1000, home `/home/ubuntu`) — drop
the generated `useradd`/`USER` block and use `WORKDIR /home/ubuntu`+
`USER ubuntu`;
- or give the container user your host's uid, so the mounts are writable
  whatever the host uid is.

`CONTAINER_HOME` (compose env + Taskfile) must match the home of the user the
agent runs as.

---

## Step 5 — Make auxiliary services reachable (`port-forward`)

An agent is sandboxed by greywall (deny-by-default), so it cannot reach sibling
compose services (e.g. a database the app uses) on its own. Declare a
`port-forward` for each service you want the agent to reach:

```yaml
ai-cabin:
  features:
    - port-forward: {port: 5432, host: postgres}
```

Each declaration opens a TCP bridge inside the container
(`localhost:5432` → the `postgres` service) and a greywall forward profile
that opens that port in the sandbox. The agent then connects to
`localhost:5432` as if the database were local.

The auxiliary services themselves (and the agent's `depends_on`) live in your
**project's own compose**, not in the cabin-generated one. When the project
already has a `docker-compose.yml` (e.g. a Postgres side container next to the
app), keep it: `cabin authoring new` will not overwrite it, and you add the
`agent` service (from the generated compose) alongside your existing services
in the same file — or keep two compose files and target the right one in the
Taskfile.

---

## Step 6 — Register, build, start, run

`cabin authoring` does not register the cabin — useful when you are trying
assemblies on throwaway directories. Register once before building so the cabin
is resolvable by name:

```bash
cabin cabin add path/to/project [name]
```

Then build, start and run through the `cabin` CLI — it wires up profile vars,
path shadowing, and the agent configs for you. The CLI invokes the internal
materialization commands automatically; you only use the public commands
below.

Build the image (this also prepares the agent dirs and materializes `.deps/`):

```bash
cabin build <cabin>
```

Start the cabin in the background (this runs the agent setup, rendering the
agent configs into `$AI_CABIN_HOME`):

```bash
cabin up <cabin>
```

Run the agent from inside your project directory (the current host path is
shadowed into the container, so the agent sees the files you are working on):

```bash
cd ~/projects/<your-project>     # must be inside AI_CABIN_WORKDIR
cabin task <cabin> opencode      # or: cabin task <cabin> pi
```

Other per-project lifecycle commands: `cabin down`, `cabin restart`,
`cabin logs`, `cabin shell` (plain container shell), `cabin greyshell`
(greywall-sandboxed shell), `cabin ps` (list agent containers).

---

## Anatomy of a cabin

```
<project>/
├── Taskfile.yml           # ai-cabin: header (source of truth) + targets
├── ai-cabin.Dockerfile    # agent image (adapt FROM to your base)
└── docker-compose.yml     # agent service (+ your project's services)
```

Shared content (greywall, the CA, wrappers, entrypoint hooks, installers,
port-forward scripts) lives in feature bundles and is installed into the image
at build time. A project carries only its cabin-specific deltas — the header,
its base image, and its own compose services.

### The agent service in compose

`cabin authoring` generates the `agent` service with an explicit `image:` tag
and **no `container_name:`**. Both matter when you run more than one instance
of a cabin (e.g. the same cabin under two profiles, or the same cabin checked
out twice):

- `image:` pins the built image name so it is **shared** across instances. The
  cabin name (header `cabin:` or the directory basename) is used as the tag —
  `image: mycabin`.
- `container_name:` is daemon-global, so two instances would **collide** and
  fail to start (or one would silently reuse the other's container). Leaving it
  out lets compose derive a per-project name (`<project>_<service>-1`), so each
  instance gets its own container.

The compose **project name** isolates instances further: `cabin` derives it
from the active profile and the cabin name (`<profile>_<cabin>`), so two
profiles operating the same cabin get distinct projects — distinct containers
and networks — while sharing the image build. You do not set this by hand; the
`docker compose` commands the cabin runs resolve it automatically (the CLI
injects it, and the standalone `task` path resolves it the same way).

---

## Next steps

- Follow the tutorials: [`examples/tuto-1/`](../examples/tuto-1/) (no Dockerfile)
  and [`examples/tuto-2/`](../examples/tuto-2/) (existing Dockerfile + auxiliary
  service). Each is a before/after exercise you can reproduce.
- To validate this guide on your own project, run through Steps 1-6 verbatim.
  If a step is unclear or wrong, please report it — it helps improve this
document.
