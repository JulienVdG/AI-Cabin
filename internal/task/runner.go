// Package task executes Taskfile targets via the go-task/task/v3 library:
// the runtime layer for `cabin task <cabin> <task> [params]`. It sets the var
// view on the process (so docker-compose ${VAR} resolves), forwards agent
// params as {{.CLI_ARGS}} / {{.CLI_ARGS_LIST}}, and runs the target.
//
// The go-task/task/v3 dependency is encapsulated here (aliased as tasklib).
package task

import (
	"context"
	"fmt"
	"io"
	"os"

	tasklib "github.com/go-task/task/v3"
	"github.com/go-task/task/v3/args"
	"github.com/go-task/task/v3/taskfile/ast"
)

// Run executes a Taskfile target located in cabinPath.
//
// envVars are set on the process env before execution; the task compiler reads
// os.Environ(), and tasks run `docker compose exec` which resolves ${VAR} from
// there (the Executor has no WithEnv, so env is the only channel). One-shot
// process, so mutating the global env is acceptable.
//
// rawArgs (agent params captured raw by Cobra) are forwarded as
// {{.CLI_ARGS}} (shell-quoted) and {{.CLI_ARGS_LIST}} (raw []string), like the
// task CLI does. Without this, {{.CLI_ARGS}} is empty when using the lib.
func Run(ctx context.Context, cabinPath, taskName string, rawArgs []string, envVars map[string]string, stdout, stderr io.Writer) error {
	for k, v := range envVars {
		if err := os.Setenv(k, v); err != nil {
			return fmt.Errorf("set env var %s: %w", k, err)
		}
	}

	exec := tasklib.NewExecutor(
		tasklib.WithDir(cabinPath),
		tasklib.WithStdout(stdout),
		tasklib.WithStderr(stderr),
	)
	if err := exec.Setup(); err != nil {
		return fmt.Errorf("setup taskfile in %s: %w", cabinPath, err)
	}

	if err := injectCLIArgs(exec, rawArgs); err != nil {
		return err
	}

	if err := exec.Run(ctx, &tasklib.Call{Task: taskName}); err != nil {
		return fmt.Errorf("run task %q: %w", taskName, err)
	}
	return nil
}

// injectCLIArgs forwards agent params as CLI_ARGS / CLI_ARGS_LIST on the parsed
// Taskfile (the task CLI does this; the lib does not populate them itself).
func injectCLIArgs(exec *tasklib.Executor, rawArgs []string) error {
	cliArgs := BuildCLIArgs(rawArgs)

	quoted, err := args.ToQuotedString(cliArgs)
	if err != nil {
		return fmt.Errorf("quote cli args: %w", err)
	}

	specialVars := ast.NewVars()
	specialVars.Set("CLI_ARGS", ast.Var{Value: quoted})
	specialVars.Set("CLI_ARGS_LIST", ast.Var{Value: cliArgs})
	exec.Taskfile.Vars.ReverseMerge(specialVars, nil)
	return nil
}

// BuildCLIArgs strips a leading "--" if the user typed one by habit
// (SetInterspersed(false) keeps it in args, but no separator is required). A
// "--" that is not first is preserved as a literal agent arg.
func BuildCLIArgs(rawArgs []string) []string {
	if len(rawArgs) > 0 && rawArgs[0] == "--" {
		return rawArgs[1:]
	}
	return rawArgs
}
