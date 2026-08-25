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

### 2. Scaffold the cabin files

```bash
cabin authoring new . --agents opencode --features go
```

This writes three new files: `ai-cabin.Dockerfile`, `docker-compose.yml` and
`Taskfile.yml`.

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

### 4. Point the base image at ubuntu

Open `ai-cabin.Dockerfile` and replace the generated base with ubuntu:

```diff
-FROM golang:1.26-trixie
+FROM ubuntu:24.04
```

The Go toolchain is fetched at build time by the bundle (Step 3 powers it), so
the bare `ubuntu` base is enough.

### 5. Use the image's default user

The generated Dockerfile creates a dedicated `ai_agent` user and the compose/
Taskfile set `CONTAINER_HOME=/home/ai_agent`. The official `ubuntu` images
already ship an `ubuntu` user (uid 1000) whose home `/home/ubuntu` matches the
host uid, so the bind-mounted dirs stay writable. Drop `ai_agent` and use it:

In `ai-cabin.Dockerfile`, replace the whole user setup block (useradd, home
paths, prompt line, mount-points):

```diff
-# User setup (the root-to-user transition stays visible here on purpose)
-RUN useradd -m ai_agent
-WORKDIR /home/ai_agent
-# Own the whole home so the agent can write anywhere it needs
-RUN chown -R ai_agent:ai_agent /home/ai_agent
-USER ai_agent
-WORKDIR /home/ai_agent
-
-# Add a greywall sandbox indicator to the prompt
-RUN echo 'if [ "$GREYWALL_SANDBOX" = "1" ]; then debian_chroot="🔒"; fi' >> /home/ai_agent/.bashrc
-
-# Create future mount-points so their owner is ai_agent
-RUN mkdir -p .local/share .local/state .local/bin .cache .config/greywall desk go
+# User setup: the image's default `ubuntu` user (uid 1000) matches the host uid,
+# so the bind-mounted dirs (cache, go, desk) are writable. No extra user is created.
+WORKDIR /home/ubuntu
+USER ubuntu
+
+# Add a greywall sandbox indicator to the prompt
+RUN echo 'if [ "$GREYWALL_SANDBOX" = "1" ]; then debian_chroot="🔒"; fi' >> /home/ubuntu/.bashrc
+
+# Create future mount-points so their owner is the default user
+RUN mkdir -p .local/share .local/state .local/bin .cache .config/greywall desk go
```

In `docker-compose.yml`, set the container home:

```diff
-      - CONTAINER_HOME=/home/ai_agent
+      - CONTAINER_HOME=/home/ubuntu
```

And in `Taskfile.yml`:

```diff
-  CONTAINER_HOME: /home/ai_agent
+  CONTAINER_HOME: /home/ubuntu
```

### 6. Register the cabin

```bash
cabin add . tuto1
```

### 7. Select, build, start

Make `tuto1` the current cabin of your profile (`cabin use`), so the lifecycle
commands below target it without repeating the name:

```bash
cabin use tuto1
cabin build
cabin up
```

### 8. Verify the Go toolchain is inside the sandbox

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

### 9. Run the agent

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
