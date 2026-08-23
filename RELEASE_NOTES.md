# Release Notes

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
