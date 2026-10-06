# Release Notes

Changes accumulated since the last release. Published as a versioned section at
the next release. Until then, items live under **Unreleased**.

A **breaking change** is one that requires action from an existing user before
they can keep using AI-Cabin as before (migration step, removed command, changed
format). Add it here as soon as it lands so the upgrade guide is ready at
release time.

## Unreleased

### Changes

- **Canonical key order for profile variables** — profile files written by the
  CLI (`cabin profile init`, `set`, `append/prepend`, `use`) now list
  `AI_CABIN_HOME` and `AI_CABIN_DESK` first, then the other `AI_CABIN_*`
  variables alphabetically, then the rest alphabetically, instead of plain
  alphabetical order: the variables a profile edit starts from lead the
  `vars:` block for human editors. The same order applies everywhere the CLI
  shows a variable map (`cabin profile show`, the `cabin setup`/`profile init`
  output, the `profile set` confirmation, the environment-shadowing warning,
  `cabin setenv`), so files and console listings agree. Existing profiles keep
  working unchanged: the order applies when the CLI next saves the file,
  hand-edited files are left untouched.

## v1.4.0

### Changes

- **`cabin profile append|prepend <var> <value>`** — add a value at the end or
  beginning of a comma-separated profile list variable (`CREDENTIAL_INJECT`,
  `AI_CABIN_LAYER_DIRS`, `AI_CABIN_FRAGMENTS_DIRS`, ...) without retyping the
  whole list: `cabin profile append CREDENTIAL_INJECT MY_CRED` is a one-liner.
  Only the target variable is persisted (the global `--var` entries stay a
  temporary override, unlike `cabin profile set` which persists them), and an
  already-present value is skipped, so re-running the same append/prepend is a
  no-op for scripts and idempotent setup lines.

- **`cabin profile get <var>`** — print the raw value of a profile variable on
  stdout, without labels, so a command substitution can use it directly:
  `$(cabin --profile work get SCW_PROJECT_ID)`. The value read is the persisted
  one from the profile selected by `--profile` (default: the current one). A
  missing variable exits non-zero with a message on stderr; a variable set to
  an empty value prints an empty line and exits 0. When a same-named
  environment variable shadows the printed value (the runtime view would
  override it), a warning goes to stderr without polluting the value.

- **New `cabin compose <args...>` passthrough** — run an arbitrary `docker compose` command on the current cabin's stack without eval-ing `cabin setenv` and cd-ing into the cabin dir: `cabin compose restart apache` after editing a sidecar service's conf files, `cabin compose logs -f apache`, .... The command resolves the same env as the other lifecycle commands (compose project name, greywall profile, profile vars) so it targets the current `(profile, cabin)` instance, and forwards args verbatim to `docker compose`. No `prepare` step runs (raw passthrough; `cabin up|down|build|restart` remain the managed lifecycle). Also available standalone as the new shared lifecycle target `task docker-compose -- <args>`.

- **`port-forward` decoupled local and target ports** — the bundle accepts an
  optional `listen` attr (`port-forward: {port: 80, host: apache, listen: 8080}`):
  socat listens on `listen` (default: the target `port`) and still forwards to
  `host:port`. A target port already bound inside the container no longer
  blocks the bridge, and the local port can be picked to match the one
  published to the host.

- **`install.d` download steps are now architecture-aware** — the fragment
  deps install scripts (pi, opencode, and the opt-in Go toolchain) resolve
  the target architecture once via `uname -m` and pick the matching release
  asset instead of hardcoding x86_64/amd64 URLs, so the same fragments build
  amd64 and arm64 images. ripgrep is bumped from 14.1.1 to 15.2.0 (the only
  pin without an arm64 asset).

- **`BackupCreator` backups are now timestamped** — `<file>.cabin-bak.<YYYYMMDD-HHMMSS>` (UTC, lexicographically sortable), disambiguated with a `-2`/`-3`... suffix when two backups land in the same second, instead of the former single-slot `.cabin-bak` that was overwritten at each setup re-run. A diff no longer erases the previous backup: each change keeps its own generation. The `.cabin-bak` prefix is kept stable, so backups stay findable by pattern (`find`, `gitignore`). The mark for cleanup is left to the user (`find`/`git status`) — cleanup is not automated.

- **Agents can no longer edit their own AGENTS.md** — the instruction files
  the cabins generate (`~/.pi/agent/AGENTS.md` for pi,
  `~/.config/opencode/AGENTS.md` for opencode) are read-only inside the
  greywall sandbox: the agent keeps reading them, edits belong to the user.
  The masters stay the desk `AGENTS.md` and the shipped workspace fragment,
  which `cabin up` concatenates into the generated file at every start.

### Fixes

- **`profile set` nothing-to-persist hint** — the error printed when
  `cabin profile set` is invoked with nothing to persist now suggests only the
  canonical `KEY=VALUE` form, dropping the `--var KEY=VALUE` mention.

- **Authored Dockerfiles force root for install steps** — the Dockerfile
  generated by `cabin authoring new` now starts with `USER root` right after
  `FROM` (a no-op on root-ending bases), so the apt/`install.d`/`useradd`
  steps no longer fail with permission denied on base images that end on a
  non-root user. The redundant first `WORKDIR` before the chown is dropped —
  the final `WORKDIR` after the `USER` switch already carries the state. The
  writing guide documents the forced `USER root` and warns that the generated
  `useradd` fails when the base image already ships that user.

## v1.3.0

### Features

- **`cabin version`** — prints the cabin version derived from the Go build
  info: the VCS tag when the revision is exactly a release tag, else the
  module version from a `go install @VERSION` build, else `(devel)`. When
  built from a checkout it also prints the revision, build date and
  dirty-tree status, so the exact source revision is always pinned.

- **Selected profile override markers** — `cabin profile list` and
  `cabin profile show` now display the profile *actually* in effect. When
  the active profile is overridden by `--profile` or `AI_CABIN_PROFILE`, it
  is flagged `(current, from <source>)` and the overridden use-selected
  profile is shown `(overridden)`, so the reader sees which one wins.

- **Authoring parametrization** — `cabin authoring show`/`new` accept
  `--image <base>`, `--user <name>` and `--home <path>` to set the base image
  `FROM`, the container user and its home at assembly time. The resolved
  values (defaults included) are recorded in the
  `ai-cabin: {authored_with: {image, user, home}}` header with precedence
  flag > header > default, and rendered into the blueprint via
  `{<.Image>}/{<.User>}/{<.Home>}` delims (distinct from the `${...}`/`{{...}}`
  runtime vars).

- **`cabin authoring show --write-prefix <p>`** — materializes the three
  assembled files (instead of stdout) with the prefix prepended to each
  canonical name (`folder/` writes into that folder, `.new.` yields
  no-collision files beside the originals). It creates the folder and
  truncates existing files, independent of `--force`.

- **`profile set` accepts `KEY=VALUE` and `--var` batching** — `cabin profile
  set` now takes `KEY=VALUE` entries (single or many, for copy-pasting
  `VAR=value` lines) in addition to the `KEY VALUE` pair, merged with the global
  `--var` flags on the same command line (they win on key conflict); all are
  persisted in one atomic write, and an empty command line errors instead of
  silently doing nothing.

- **`--var` tab completion** — the global `--var` flag now completes with known
  profile variable names: the active profile's persisted vars (the
  `--profile`-selected profile if given) plus the known default keys
  (`AI_CABIN_HOME/DESK/WORKDIR`, `CONTAINER_WORKDIR`, `AI_CABIN_LAYER_DIRS`,
  `CREDENTIAL_*`, ...), formatted as `KEY=` so the cursor lands right after the
  `=`. Vars already given on the same command line are excluded.

- **`HOST_UID` build arg** — the generated Dockerfile aligns the container
  user's uid with the host at build time (`ARG HOST_UID=1000` + `useradd -u
  ${HOST_UID}`), passed by compose `build.args: HOST_UID=${HOST_UID:-1000}` and
  resolved by the lifecycle Taskfile (`HOST_UID` env > `id -u` > `1000`). The
  bind-mounted host dirs stay writable whatever the host uid, from committed
  files that never change.

### Fixes

- **Greywall profile denies `.env` in any directory** — the default workspace
  profile now denies reads of `.env`, `.env.*` via the `**/.env`, `**/.env.*`
  glob, so the exclusion holds in any subdirectory, not just at the workspace
  root.

## v1.2.0

### Features

- **Layer activation** — `AI_CABIN_LAYER_DIRS` activates a self-contained
  override root (per profile): `<layer>/fragments` prepends the fragment
  chain (below `AI_CABIN_FRAGMENTS_DIRS`), `<layer>/skeletons` joins the
  skeleton catalogue, and an optional `<layer>/layer.yaml` `vars:` block
  contributes **profile defaults**. Setting it at profile creation — via
  `--var AI_CABIN_LAYER_DIRS=...` or an exported env var at `cabin setup`/
  `cabin profile init` — persists it in the profile var. A layer may
  carry only some subdirs (a fragments-only root is valid and tolerated).

## v1.1.0

### Breaking changes

- **Containers must be torn down before upgrading.** Instances are now isolated
  per `(profile, cabin)` via a derived `COMPOSE_PROJECT_NAME` (`<profile>_<cabin>`,
  or `<cabin>` alone when no profile is selected). A cabin started before this
  change was created under the compose-default project (the directory basename),
  so the next `cabin up` starts it under a new project and the previous container
  is orphaned (not stopped, not found by `docker compose`).
  - **Migration**: run `cabin down <cabin>` (or `docker compose down` from the
    cabin dir) for every running cabin **before** updating. After the update,
    `cabin up` recreates each instance under its now-stable project name.

- **Cabin commands no longer take a positional `<cabin>`.** The target is
  resolved by the `--cabin` flag or the **current cabin** of the active
  profile (`cabin use <cabin>` sets it; it is sugar for
  `cabin profile set AI_CABIN_CURRENT_CABIN <cabin>`, resolution
  `--cabin` > env > profile var). The cabin registry moves to the root:
  `cabin add` / `cabin list` / `cabin scan` (the `cabin cabin` namespace is
  removed).
  - **Migration**: `cabin build <name>` / `cabin up <name>` / ... become
    `cabin --cabin <name> build` / `cabin --cabin <name> up` / ..., or set the
    current cabin once with `cabin use <name>` then run `cabin build` /
    `cabin up` / ...; `cabin task <name> <target>` becomes
    `cabin --cabin <name> task <target>` (or `cabin task <target>` after
    `cabin use <name>`); `cabin cabin add/list/scan` become
    `cabin add/list/scan`.

### Features

- **Per-profile instance isolation.** `cabin --profile A up <cabin>` and
  `cabin --profile B up <cabin>` now operate distinct instances (containers and
  networks) of the same cabin while sharing the image build. The compose project
  name is derived from the active profile and the cabin canonical name; the CLI
  injects it, and the standalone `task` path resolves it the same way via a
  `cabin internal compose-project-name` fallback.
- **`cabin ps` shows the active profile.** Each agent container is now listed
  with its profile, derived from the compose project label.
- **Authoring emits `image:` instead of `container_name:`.** The generated
  agent service pins the image name (shared across instances) and no longer
  sets a daemon-global `container_name` (which would collide across instances).

### Fixes

- **`GetActiveProfile` now honors `AI_CABIN_PROFILE`.** It previously resolved
  only `--profile` then the current profile from `config.yaml`, skipping the
  `AI_CABIN_PROFILE` env var that `cabin setenv` exports — so `cabin profile
  show`/`set` and the compose-project-name resolution diverged from
  `ResolveVars` on the standalone `task` path. The precedence is now uniform:
  `--profile` > `AI_CABIN_PROFILE` env > current profile.

## v1.0.0

Initial public release, see README.md
