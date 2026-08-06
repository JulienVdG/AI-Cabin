package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/JulienVdG/AI-Cabin/internal/config"
	"github.com/JulienVdG/AI-Cabin/internal/embedded"
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
func runCabinTask(ctx context.Context, cabinName, taskName string, rawArgs []string, stdout, stderr io.Writer) error {
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

	// Compute the host CWD sub-path relative to the workdir and inject it as
	// CABIN_REL_PATH (path shadowing: the agent launches into the matching
	// sub-directory inside the greywall sandbox). The Taskfile forwards it to
	// `docker compose exec -e CABIN_REL_PATH=...` so the container-side
	// wrapper does a two-step cd (root anchor + relpath inside the sandbox).
	// Refuses paths outside the workdir (fail-fast, no silent fallback). Empty
	// when CWD is the workdir root (the agent launches at the root).
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current working directory: %w", err)
	}
	rel, err := config.RelPath(wd, vm[config.WorkdirVar])
	if err != nil {
		// Not fatal: log to stderr and fall back to the root (relpath empty)
		// rather than block the agent on a host-side path quirk. The
		// container-side cd remains the last line of defense.
		fmt.Fprintf(os.Stderr, "Warning: %v; launching agent at the workdir root\n", err)
	}
	vm["CABIN_REL_PATH"] = rel

	// Ensure the lifecycle Taskfile is materialized to XDG state and inject its
	// path so the cabin's includes: resolves to where the file was actually
	// written (matters when XDG_STATE_HOME is redirected, e.g. the dev pattern).
	stateFS, err := embedded.State()
	if err != nil {
		return fmt.Errorf("load embedded state: %w", err)
	}
	lifecyclePath, err := state.EnsureArtifact(stateFS, lifecycleTaskfileName)
	if err != nil {
		return fmt.Errorf("materialize lifecycle taskfile: %w", err)
	}
	vm["AI_CABIN_LIFECYCLE_TASKFILE"] = lifecyclePath

	return task.Run(ctx, c.Path, taskName, rawArgs, vm, stdout, stderr)
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
// docker-* names.
func lifecycleWrapper(name, target, short string) *cobra.Command {
	return &cobra.Command{
		Use:   name + " <cabin>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cabinName := args[0]
			if err := runCabinTask(cmd.Context(), cabinName, target, nil, os.Stdout, os.Stderr); err != nil {
				exitOnRunError(os.Stderr, cabinName, err)
			}
		},
	}
}

func init() {
	rootCmd.AddCommand(
		lifecycleWrapper("up", "docker-up", "Start the cabin in background"),
		lifecycleWrapper("down", "docker-down", "Stop the cabin"),
		lifecycleWrapper("build", "docker-build", "Build the cabin image"),
		lifecycleWrapper("shell", "docker-shell", "Get a bash shell inside the running agent container"),
		lifecycleWrapper("greyshell", "docker-greyshell", "Get a greywall sandboxed shell inside the agent container"),
		lifecycleWrapper("logs", "docker-logs", "Follow agent container logs"),
		lifecycleWrapper("restart", "docker-restart", "Restart the agent container"),
	)
}
