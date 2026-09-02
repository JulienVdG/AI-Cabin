# Tuto 1 — One agent, no existing Dockerfile

Equip a Go project (no Dockerfile) with a cabin on an `ubuntu` base, letting the
`go` feature bundle install the Go toolchain at build time.

> This is a walkthrough tutorial: each step is a ready-to-run command. For the
> technical explanations and concepts behind these steps, refer to the
> [Cabin Writing Guide](../../docs/CABIN-WRITING-GUIDE.md). `before/` is the
> starting point, `after/` is the finished result to compare against.

## Prerequisites

A working `cabin setup` (default profile ready) and Greyproxy reachable from
Docker.

## Steps

### 1. Copy the starting point into your workdir

```bash
cp -r before ~/projects/tuto1
cd ~/projects/tuto1
```

Check you have the two project files:

```bash
ls
```

### 2. Scaffold the cabin files with the image and user

This tutorial targets a bare `ubuntu` base that already ships an `ubuntu` user
(uid 1000). Set the image, the container user and its home **at scaffold time**
with the authoring params, so the generated files point at them from the start:

```bash
cabin authoring new . --agents opencode --features go \
  --image ubuntu:24.04 --user ubuntu --home /home/ubuntu
```

This writes three new files — `ai-cabin.Dockerfile`, `docker-compose.yml` and
`Taskfile.yml` — and records the choice in the `authored_with:` header:

```yaml
ai-cabin:
  agents:
    - opencode
  features:
    - go
  authored_with:
    image: ubuntu:24.04
    user: ubuntu
    home: /home/ubuntu
```

The bare `ubuntu` base is enough: the `go` feature bundle fetches and installs
the Go toolchain at build time (Step 3 powers it).

### 3. Turn on the Go toolchain install in the header

Open `Taskfile.yml` and make the `features:` entry a map with the install
attribute (the generated header lists `go` without it):

```yaml
ai-cabin:
  agents:
    - opencode
  features:
    - go: {install: true, version: "1.26.3"}
```

### 4. Drop the `useradd` (the image already ships the user)

Because we used `--user ubuntu --home /home/ubuntu`, every home path (prompt,
mount-points) and the compose/Taskfile `CONTAINER_HOME` already point to
`/home/ubuntu`. The only thing left is the dedicated-user setup that would
create `ubuntu` — but the base image already ships it, so `useradd` would fail
("user already exists"). Drop that block:

In `ai-cabin.Dockerfile`, replace the user-setup block:

```diff
-# User setup (the root-to-user transition stays visible here on purpose)
-# HOST_UID aligns the container user's uid with the host so the bind-mounts
-# stay writable: the lifecycle resolves it (HOST_UID > id -u > 1000) and
-# compose passes it through build.args; the Dockerfile default is 1000.
-ARG HOST_UID=1000
-RUN useradd -m -u ${HOST_UID} ubuntu
-WORKDIR /home/ubuntu
-# Own the whole home so the agent can write anywhere it needs
-RUN chown -R ubuntu:ubuntu /home/ubuntu
-USER ubuntu
-WORKDIR /home/ubuntu
+# User setup: the image's default `ubuntu` user (uid 1000) matches the host uid,
+# so the bind-mounted dirs (cache, go, desk) are writable. No extra user is created.
+USER ubuntu
+WORKDIR /home/ubuntu
```

Nothing else to change: the prompt and mount-point lines below already use
`/home/ubuntu`, and the compose/Taskfile `CONTAINER_HOME` is already
`/home/ubuntu` — no edits there.

### 5. Register the cabin

```bash
cabin add . tuto1
```

### 6. Select, build, start

Make `tuto1` the current cabin of your profile (`cabin use`), so the lifecycle
commands below target it without repeating the name:

```bash
cabin use tuto1
cabin build
cabin up
```

### 7. Verify the Go toolchain is inside the sandbox

From another terminal, open a sandboxed shell (a real shell, not the agent TUI)
and run the project inside it — this proves the bundle installed Go and the
project builds in the sandbox:

```bash
cd ~/projects/tuto1
cabin greyshell
# inside the sandboxed shell:
go run .
```

You should see `Hello from the sample Go project`.

### 8. Run the agent

```bash
cd ~/projects/tuto1
cabin task opencode
```

From the opencode TUI, you can also ask the agent to run the project and
report:

```
Run `go run ./` in this project and report the output.
```

#### Verify

On the host, open another terminal:

```bash
cd ~/projects/tuto1
go run .
```

The agent and the host share the same files — what the agent works on is what
you see on the host.

## Compare with the solution

`after/` is the finished cabin. Your `~/projects/tuto1` should match it:
same header, same `FROM ubuntu:24.04`, same compose and Taskfile.

```bash
# from the repo root
diff -r --exclude=.gitignore --exclude=hello ~/projects/tuto1 examples/tuto-1/after
```
