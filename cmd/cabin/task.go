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
//	cabin task <task> [params...]
//
// The cabin is resolved by --cabin / current-cabin (resolveTargetCabin); the
// first positional is the target. The CLI resolves the var view (set on the
// process so docker-compose ${VAR} resolves) and forwards extra params to the
// task's {{.CLI_ARGS}}. A `task` subcommand (not root fallback) keeps internal
// commands (up/build/ps/...) and Taskfile targets from colliding.
//
// The shared runCabinTask helper also materializes the lifecycle Taskfile and
// injects AI_CABIN_CMD/AI_CABIN_LIFECYCLE_TASKFILE, the same path the
// `cabin up|down|...` wrappers use, so `cabin task` and the wrappers behave
// identically.
var taskCmd = &cobra.Command{
	Use:   "task <task> [params...]",
	Short: "Run a Taskfile target of a cabin",
	Long: `Run a Taskfile target of a cabin, forwarding extra params to the
Taskfile target's {{.CLI_ARGS}} (e.g. agent flags like --port).

The cabin must be registered first with 'cabin add'. It is selected with
--cabin (before any positional) or the current cabin of the active profile
('cabin use <cabin>'); pass --profile to pick the profile. The active
profile's env vars are set on the process so docker-compose ${VAR} resolves.

Examples:
  cabin task pi           # run the default target of the current cabin
  cabin --cabin blog task opencode web --port 9090
`,
	Args: cobra.MinimumNArgs(1),
	// <task> (1st positional) is completed from the target cabin's Taskfile;
	// the 2nd+ positional (agent params) is forwarded raw via {{.CLI_ARGS}} and
	// is not completed.
	ValidArgsFunction: completeTaskArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cabinName, err := resolveTargetCabin()
		if err != nil {
			exitOnRunError(os.Stderr, cabinName, err)
			return
		}
		taskName := args[0]
		rawArgs := args[1:]

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
