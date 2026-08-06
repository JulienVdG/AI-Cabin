package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// taskCmd runs a Taskfile target of a registered cabin:
//
//	cabin task <cabin> <task> [params...]
//
// Resolves the cabin via the registry, resolves the var view (set on the
// process so docker-compose ${VAR} resolves), and forwards extra params to
// the task's {{.CLI_ARGS}}. A `task` subcommand (not root fallback) keeps
// internal commands (up/build/ps/...) and Taskfile targets from colliding.
//
// The shared runCabinTask helper also materializes the lifecycle Taskfile and
// injects AI_CABIN_CMD/AI_CABIN_LIFECYCLE_TASKFILE, the same path the
// `cabin up|down|...` wrappers use, so `cabin task` and the wrappers behave
// identically.
var taskCmd = &cobra.Command{
	Use:   "task <cabin> <task> [params...]",
	Short: "Run a Taskfile target of a cabin",
	Long: `Run a Taskfile target of a registered cabin, forwarding extra params to the
task's {{.CLI_ARGS}} (e.g. agent flags like --port).

The cabin must be registered first with 'cabin cabin add'. The active
profile's env vars are set on the process so docker-compose ${VAR} resolves.

Examples:
  cabin task pi-go pi
  cabin task blog opencode web --port 9090
`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cabinName := args[0]
		taskName := args[1]
		rawArgs := args[2:]

		if err := runCabinTask(context.Background(), cabinName, taskName, rawArgs, true, os.Stdout, os.Stderr); err != nil {
			exitOnRunError(os.Stderr, cabinName, err)
		}
	},
}

func init() {
	// Stop parsing flags as soon as a positional arg appears, so agent flags
	// like --port 9090 are captured raw (forwarded via {{.CLI_ARGS}}) instead
	// of being interpreted by Cobra.
	taskCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(taskCmd)
}

// resolveExecutable returns the resolved absolute path of the running cabin
// binary (os.Executable + symlink resolution). Used to inject AI_CABIN_CMD so
// the Taskfile can self-delegate via $AI_CABIN_CMD without depending on PATH.
func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get executable path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}
