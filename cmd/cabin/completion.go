package main

import (
	"os"
	"sort"
	"strings"

	"github.com/JulienVdG/AI-Cabin/internal/config"
	"github.com/JulienVdG/AI-Cabin/internal/task"

	"github.com/spf13/cobra"
)

// completeCabinNames returns the registered cabin names for shell completion of
// the first positional arg of cabin-targeting commands (task, up, down, build,
// shell, greyshell, logs, restart). It reads the registry (config.ListCabins),
// filters by the prefix being typed, and excludes names already on the command
// line. Returns NoFileComp so the shell does not fall back to filenames — a
// cabin name is always a registry entry, never an arbitrary path here.
func completeCabinNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cabins, err := config.ListCabins()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	seen := make(map[string]bool, len(args))
	for _, a := range args {
		seen[a] = true
	}
	var out []string
	for _, c := range cabins {
		if seen[c.Name] {
			continue
		}
		if strings.HasPrefix(c.Name, toComplete) {
			out = append(out, c.Name)
		}
	}
	sort.Strings(out)
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeTaskArgs completes the positionals of `cabin task <cabin> <task>
// [params]`:
//   - 1st positional (<cabin>): registered cabin names.
//   - 2nd positional (<task>): the cabin's Taskfile targets (parsed via the
//     task lib after the lifecycle include is resolved, so the docker-* targets
//     appear too).
//   - 3rd+ positional (agent params): not completed (forwarded raw via
//     {{.CLI_ARGS}}).
//
// Task-target completion sets AI_CABIN_LIFECYCLE_TASKFILE on the process env
// (same channel as runCabinTask) so Setup() resolves the lifecycle include.
// On any error (unknown cabin, Taskfile parse failure) it returns NoFileComp
// rather than crashing the shell: a completion that errors loudly is worse than
// one that silently offers nothing (the user can still type the target).
func completeTaskArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		// Completing the cabin name (1st positional).
		return completeCabinNames(cmd, args, toComplete)
	case 1:
		// Completing the task name (2nd positional).
		cabinName := args[0]
		c, err := config.GetCabin(cabinName)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		lifecyclePath, err := ensureLifecycleArtifact()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if err := os.Setenv("AI_CABIN_LIFECYCLE_TASKFILE", lifecyclePath); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		targets, err := task.ListTargets(c.Path)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var out []string
		seen := make(map[string]bool, len(args))
		for _, a := range args[1:] {
			seen[a] = true
		}
		for _, t := range targets {
			if seen[t] {
				continue
			}
			if strings.HasPrefix(t, toComplete) {
				out = append(out, t)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	default:
		// 3rd+ positional: agent params, forwarded raw — no completion.
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// of the --profile flag value and `cabin profile use|show <name>`. Same shape as
// completeCabinNames (prefix filter + exclude already-provided positionals).
// of the --profile flag value and `cabin profile use|show <name>`. Same shape as
// completeCabinNames (prefix filter + exclude already-provided positionals).
func completeProfileNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	profiles, err := config.ListProfiles()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	seen := make(map[string]bool, len(args))
	for _, a := range args {
		seen[a] = true
	}
	var out []string
	for _, p := range profiles {
		if seen[p] {
			continue
		}
		if strings.HasPrefix(p, toComplete) {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, cobra.ShellCompDirectiveNoFileComp
}
