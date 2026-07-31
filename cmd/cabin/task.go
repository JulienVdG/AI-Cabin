package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JulienVdG/AI-Cabin/internal/config"
	"github.com/JulienVdG/AI-Cabin/internal/task"

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

		c, err := config.GetCabin(cabinName)
		if err != nil {
			if errors.Is(err, config.ErrCabinNotFound) {
				fmt.Fprintf(os.Stderr,
					"Error: cabin %q is not registered.\n"+
						"Run 'cabin cabin add <path> %s' to register it, or 'cabin cabin list' to see known cabins.\n",
					cabinName, cabinName)
			} else {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			os.Exit(1)
		}

		vars, err := config.ResolveVars(profileFlag, cliVars)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Inject the CLI's own path so the Taskfile can self-delegate via
		// $AI_CABIN_CMD (e.g. `deps` target runs "$AI_CABIN_CMD internal deps").
		// An absolute path avoids PATH lookups failing on a freshly built binary.
		if exe, err := resolveExecutable(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: resolve cabin executable: %v\n", err)
			os.Exit(1)
		} else {
			vars.AsMap()["AI_CABIN_CMD"] = exe
		}

		if err := task.Run(context.Background(), c.Path, taskName, rawArgs, vars.AsMap(), os.Stdout, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
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
