# Tuto 2 — Two agents, existing Dockerfile + auxiliary service

Equip a Flask app (already has a Dockerfile + a compose with a Postgres side
container) with a two-agent cabin. The cabin image is built **on top of the
app's built image**, so the agent inherits the app's full environment (Python +
its pip deps), and the Postgres service is made reachable to the agent through
`port-forward`.

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
cp -r before ~/projects/tuto2
cd ~/projects/tuto2
```

This is a normal project — a Flask app with its own `Dockerfile`
(`python:3.12-slim`) and a `docker-compose.yml` (`web` + `postgres` services).
It works on its own with `docker compose up --build` (see `before/README.md`).

### 2. Scaffold the cabin files

Set the base image at scaffold time so the cabin Dockerfile already builds on
**top of the app image** (the agent inherits Python and its pip deps):

```bash
cabin authoring new . --agents opencode,pi --features git-agent --image tuto2-web
```

Unlike tuto 1, the project already owns a `docker-compose.yml`, and `new` is
non-destructive — so the output is:

```
  skip  docker-compose.yml (exists)
  write ai-cabin.Dockerfile
  write Taskfile.yml
```

The `agent` service is **not** written (the filename is taken by the project's
compose). We re-add it by merging it into the existing compose later (Step 7).
The `--image tuto2-web` value is recorded in the `authored_with:` header, like
the default user/home:

```yaml
ai-cabin:
  agents:
    - opencode
    - pi
  features:
    - git-agent
  authored_with:
    image: tuto2-web
    user: ai_agent
    home: /home/ai_agent
```

### 3. Add the Postgres forward to the header

`port-forward` carries attrs (the service `{port, host}`) that flags cannot
express, so add it by editing `Taskfile.yml`. The generated header lists
`git-agent`; append a `port-forward` entry.

In `Taskfile.yml`:

```yaml
ai-cabin:
  agents:
    - opencode
    - pi
  features:
    - git-agent
    - port-forward: {port: 5432, host: postgres}
```

### 4. Give the app image a name

The cabin image builds `FROM tuto2-web` (set in Step 2 via `--image`); for the
agent to build on it, the app's `web` service must have a **named image** with
a tag. In `docker-compose.yml`, add the `image` line to your existing `web`
service:

```diff
   web:
     build: .
+    image: tuto2-web
```

`tuto2-web` is Debian-based (from `python:3.12-slim`), so the cabin's
`apt-get` and greyproxy CA steps work unchanged.

### 5. Add the Postgres client

The agent needs `psql`/`pg_isready` to talk to Postgres from its sandbox. Add
Add the `postgresql-client` package to the `apt-get install` list:

In `ai-cabin.Dockerfile`:

```diff
     bubblewrap \
     socat \
     strace \
+    postgresql-client \
     && rm -rf /var/lib/apt/lists/*
```

### 6. Keep the generated user

The cabin creates the `ai_agent` user from the blueprint as-is. Its uid follows
the host at build time via the `HOST_UID` build arg (default 1000), so the
bind-mounted dirs stay writable for a host user with uid 1000; a host with a
different uid can override `HOST_UID` in the compose `.env`. The default user/home
are recorded in the `authored_with:` header (Step 2).

### 7. Merge the `agent` service into the compose

`new` skipped the compose, so the `agent` service must be added by hand to the
project's `docker-compose.yml`. Grab the generated service from the rendered assembly (the `docker-compose.yml`
section of the output):

```bash
cabin authoring show .
```

Paste the `agent` service into `docker-compose.yml` next to `web` and
`postgres`, with `depends_on` on the services it relies on:

In `docker-compose.yml`:

```yaml
    depends_on:
      - web
      - postgres
```

Because the `agent` sits in the same compose file as `postgres`, it is on the
same network — and `port-forward` (Step 3) bridges `localhost:5432` inside the
agent's sandbox to the `postgres` service.

### 8. Register the cabin

```bash
cabin add . tuto2
```

### 9. Select, build, then start everything

Make `tuto2` the current cabin of your profile (`cabin use`), so the lifecycle
commands below target it without repeating the name:

```bash
cabin use tuto2
```

`cabin build` prepares the agent dirs, **materializes `.deps/`** (the
Dockerfile `COPY .deps/` needs it) and runs `docker compose build` — which
builds the `web` image (`tuto2-web`, tagged in its service) first and then the
`agent` image `FROM` it, in the right order because `agent` depends on `web`:

```bash
cabin build
```

Then `cabin up` runs `docker compose up -d`, which starts **all** services
of the merged compose — `web`, `postgres` and `agent` — in one command:

```bash
cabin up
```

### 10. Verify the agent reaches Postgres

From a sandboxed shell, `localhost:5432` is bridged to the `postgres` service:

```bash
cd ~/projects/tuto2
cabin greyshell
# inside the sandboxed shell:
pg_isready -h localhost -p 5432
```

And the app is still reachable on the host at http://localhost:8000.

### 11. Run the agents

```bash
cd ~/projects/tuto2
cabin task opencode
# or: cabin task pi
```

From opencode, ask it to work with the database through the forwarded
Postgres:

```
Connect to Postgres on localhost:5432, insert a new item into the items table, and report what you did.
```

The agent reaches the same Postgres the web app uses — via the `port-forward`
(`localhost:5432` in its sandbox). Insert a row through psql in the agent, then
refresh http://localhost:8000 on the host to see it appear.

## Compare with the solution

`after/` is the finished cabin. Your `~/projects/tuto2` should match it.

```bash
# from the repo root
diff -r ~/projects/tuto2 examples/tuto-2/after
```

## What this walkthrough surfaced

Being written from a real run, these points are intentional findings rather
than accidents:

- **`authoring new` skips an existing compose** — non-destructive is good, but
  it means the `agent` service is not auto-written when the project owns the
  compose; the merge is manual (Step 7). A future improvement could write the
  `agent` service to a second compose file automatically.
- **Build ordering** — `cabin build` runs `docker compose build` on both `web`
  and `agent`; the `agent` image builds `FROM tuto2-web` **after** `web` because
  `agent` depends on `web` in the compose (validated live).
- **One "up" command** — `cabin up` runs `docker compose up -d`, which starts
  every service of the merged compose, so `web`/`postgres`/`agent` come up
  together.
- **User/uid alignment** — an arbitrary app image ships no non-root user, so the
  blueprint's `ai_agent` is created with an uid that follows the host at build
time via the `HOST_UID` build arg (default 1000; override it in the compose
`.env` for a host with a different uid).
