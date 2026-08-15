package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/JulienVdG/AI-Cabin/internal/config"
	"github.com/JulienVdG/AI-Cabin/internal/embedded"
	"github.com/JulienVdG/AI-Cabin/internal/fragments"
	"github.com/JulienVdG/AI-Cabin/internal/state"
	"github.com/JulienVdG/AI-Cabin/internal/task"

	"github.com/spf13/cobra"
)

// lifecycleTaskfileName is the embedded state artifact each cabin Taskfile
// includes (flatten: true, optional: true). The CLI materializes it to XDG
// state and injects its path as AI_CABIN_LIFECYCLE_TASKFILE.
const lifecycleTaskfileName = "Taskfile.lifecycle.yml"

// runCabinTask resolves a registered cabin, materializes the shared lifecycle
// Taskfile to XDG state, injects the runtime vars (AI_CABIN_CMD,
// AI_CABIN_LIFECYCLE_TASKFILE) on the task subprocess, and runs the named
// Taskfile target. Shared by `cabin task` and the `cabin up|down|...`
// wrappers (which map to the docker-* lifecycle targets).
//
// Materializing the lifecycle on every cabin task keeps the docker-* targets
// available to `task` standalone and is idempotent (content-compare no-op when
// up to date).
func runCabinTask(ctx context.Context, cabinName, taskName string, rawArgs []string, needRelpath bool, stdout, stderr io.Writer) error {
	c, err := config.GetCabin(cabinName)
	if err != nil {
		return err
	}

	vars, err := config.ResolveVars(profileFlag, cliVars)
	if err != nil {
		return err
	}
	vm := vars.AsMap()

	// Inject the CLI's own path so the Taskfile can self-delegate via
	// $AI_CABIN_CMD (same pattern as `make` passing $MAKE). An absolute path
	// avoids PATH lookups failing on a freshly built binary.
	exe, err := resolveExecutable()
	if err != nil {
		return fmt.Errorf("resolve cabin executable: %w", err)
	}
	vm["AI_CABIN_CMD"] = exe

	// Path shadowing (relpath): inject the host CWD sub-path relative to the
	// workdir as CABIN_REL_PATH so the agent launches into the matching
	// sub-directory inside the greywall sandbox (the Taskfile forwards it via
	// `docker compose exec -e CABIN_REL_PATH=...`; the container-side wrapper
	// does a two-step cd: root anchor + relpath inside the sandbox).
	//
	// Computed only for targets that drop the user into the container (task,
	// shell, greyshell); skipped for container-level actions (up/down/build/
	// logs/restart). The user can opt out with --no-relpath to launch at the
	// workdir root explicitly (e.g. from a CWD outside the workdir tree).
	//
	// Fail-fast when CWD is outside the workdir: a silent fallback to the root
	// would make the agent run in the wrong directory while the user believes
	// it is in the sub-path. The container-side cd remains the last line of
	// defense.
	rel := ""
	if needRelpath && !noRelpathFlag {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get current working directory: %w", err)
		}
		rel, err = config.RelPath(wd, vm[config.WorkdirVar])
		if err != nil {
			return fmt.Errorf("%w (cd into the workdir tree, or pass --no-relpath to launch at the workdir root)", err)
		}
	}
	vm["CABIN_REL_PATH"] = rel

	// Ensure the lifecycle Taskfile is materialized to XDG state and inject its
	// path so the cabin's includes: resolves to where the file was actually
	// written (matters when XDG_STATE_HOME is redirected, e.g. the dev pattern).
	lifecyclePath, err := ensureLifecycleArtifact()
	if err != nil {
		return fmt.Errorf("materialize lifecycle taskfile: %w", err)
	}
	vm["AI_CABIN_LIFECYCLE_TASKFILE"] = lifecyclePath

	// Resolve the greywall profile list for this cabin and inject it on the
	// process (from the cabin's path, independent of the caller's CWD).
	// Best-effort: a resolution failure leaves the var unset instead of
	// blocking task execution.
	if bundles, vars, merged, _, err := resolveCabinFragments(c.Path); err == nil {
		if profiles, err := fragments.ResolveGreywallProfiles(merged, bundles, vars.AsMap()); err == nil {
			vm["GREYWALL_PROFILE"] = strings.Join(profiles, ",")
		}
	}

	return task.Run(ctx, c.Path, taskName, rawArgs, vm, stdout, stderr)
}

// ensureLifecycleArtifact materializes the embedded lifecycle Taskfile to XDG
// state (idempotent: no-op when the on-disk copy is up to date) and returns its
// absolute path. Shared by runCabinTask (sets it on the task subprocess env) and
// task-target completion (sets it on the process env so Setup() resolves the
// lifecycle include and the docker-* targets appear).
func ensureLifecycleArtifact() (string, error) {
	stateFS, err := embedded.State()
	if err != nil {
		return "", fmt.Errorf("load embedded state: %w", err)
	}
	return state.EnsureArtifact(stateFS, lifecycleTaskfileName)
}

// exitOnRunError prints a run error to stderr with actionable guidance and
// exits non-zero. ErrCabinNotFound gets a richer message (how to register);
// other errors are printed as-is. Shared by `cabin task` and the wrappers.
func exitOnRunError(w io.Writer, cabinName string, err error) {
	if errors.Is(err, config.ErrCabinNotFound) {
		fmt.Fprintf(w,
			"Error: cabin %q is not registered.\n"+
				"Run 'cabin cabin add <path> %s' to register it, or 'cabin cabin list' to see known cabins.\n",
			cabinName, cabinName)
	} else {
		fmt.Fprintf(w, "Error: %v\n", err)
	}
	os.Exit(1)
}

// lifecycleWrapper builds a `cabin <cmd> <cabin>` command that delegates to the
// shared docker-<cmd> Taskfile target. The `docker-` prefix avoids collision
// with cabin-owned targets: task errors on a flatten include with a duplicate
// name, so the cabin owns setup/deps/agent targets and the lifecycle owns the
// docker-* names. needRelpath selects whether the host CWD sub-path is
// injected (shell/greyshell drop the user into the container) or skipped
// (up/down/build/logs/restart are container-level actions with no CWD to
// propagate).
func lifecycleWrapper(name, target, short string, needRelpath bool) *cobra.Command {
	return &cobra.Command{
		Use:   name + " <cabin>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		// <cabin> is completed from the registry (config.ListCabins).
		ValidArgsFunction: completeCabinNames,
		Run: func(cmd *cobra.Command, args []string) {
			cabinName := args[0]
			if err := runCabinTask(cmd.Context(), cabinName, target, nil, needRelpath, os.Stdout, os.Stderr); err != nil {
				exitOnRunError(os.Stderr, cabinName, err)
			}
		},
	}
}

func init() {
	rootCmd.AddCommand(
		lifecycleWrapper("up", "docker-up", "Start the cabin in background", false),
		lifecycleWrapper("down", "docker-down", "Stop the cabin", false),
		lifecycleWrapper("build", "docker-build", "Build the cabin image", false),
		lifecycleWrapper("shell", "docker-shell", "Get a bash shell inside the running agent container", true),
		lifecycleWrapper("greyshell", "docker-greyshell", "Get a greywall sandboxed shell inside the agent container", true),
		lifecycleWrapper("logs", "docker-logs", "Follow agent container logs", false),
		lifecycleWrapper("restart", "docker-restart", "Restart the agent container", false),
	)
}
