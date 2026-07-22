package task_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/task"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testdataDir is the Taskfile dir for integration tests. go test runs with
// CWD set to the package dir, so the relative path resolves (same convention
// as the go-task/task/v3 library's own tests).
const testdataDir = "testdata"

func TestBuildCLIArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "nil", in: nil, want: nil},
		{name: "empty", in: []string{}, want: []string{}},
		{name: "no dash", in: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "leading dash stripped", in: []string{"--", "a", "b"}, want: []string{"a", "b"}},
		{name: "single leading dash stripped", in: []string{"--"}, want: []string{}},
		{name: "dash not first kept", in: []string{"a", "--", "b"}, want: []string{"a", "--", "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := task.BuildCLIArgs(tc.in)
			assert.Equal(t, tc.want, got, "BuildCLIArgs(%v)", tc.in)
		})
	}
}

// TestRun exercises Run end-to-end against the testdata Taskfile. It validates
// {{.CLI_ARGS}} forwarding (incl. quoting of args with spaces) and ${VAR}
// resolution from the profile env (env is the only channel for docker-compose
// ${VAR}, since the task Executor has no WithEnv and reads os.Environ()).
func TestRun(t *testing.T) {
	// Forwards the raw agent args to {{.CLI_ARGS}}. Also covers quoting: an arg
	// containing a space is shell-quoted by args.ToQuotedString and unquoted
	// back to "with space" by echo.
	t.Run("forwards CLI args to {{.CLI_ARGS}}", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		err := task.Run(context.Background(), testdataDir, "echo-args",
			[]string{"hello", "world", "with space"}, nil, &stdout, &stderr)
		require.NoError(t, err, "stderr: %s", stderr.String())

		got := stdout.String()
		for _, want := range []string{"hello", "world", "with space"} {
			assert.Contains(t, got, want, "CLI_ARGS not forwarded")
		}
	})

	// Injects the profile env so the task's shell command sees ${SOME_VAR}.
	// Non-parallel: Run mutates the process env (os.Setenv). SOME_VAR is restored
	// via t.Cleanup to avoid leaking into other tests.
	t.Run("injects profile env to ${VAR}", func(t *testing.T) {
		old, hadOld := os.LookupEnv("SOME_VAR")
		t.Cleanup(func() {
			if hadOld {
				_ = os.Setenv("SOME_VAR", old)
			} else {
				_ = os.Unsetenv("SOME_VAR")
			}
		})

		var stdout, stderr bytes.Buffer
		err := task.Run(context.Background(), testdataDir, "echo-env",
			nil, map[string]string{"SOME_VAR": "testvalue"}, &stdout, &stderr)
		require.NoError(t, err, "stderr: %s", stderr.String())

		assert.Contains(t, stdout.String(), "SOME_VAR=testvalue", "profile env not injected")
	})
}
