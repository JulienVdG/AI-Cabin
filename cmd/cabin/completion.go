package main

import (
	"os"
	"sort"
	"strings"

	"github.com/JulienVdG/AI-Cabin/internal/config"
	"github.com/JulienVdG/AI-Cabin/internal/task"

	"github.com/spf13/cobra"
)

// completeCabinNames returns the registered cabin names for the --cabin flag,
// `cabin use <cabin>`, and (previously) the cabin positionals of
// cabin-targeting commands. It reads the registry (config.ListCabins),
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

// completeTaskArgs completes the positionals of `cabin task <task>
// [params]`:
//   - 1st positional (<task>): the target cabin's Taskfile targets (the cabin
//     is the --cabin flag or the current cabin).
//   - 2nd+ positional (agent params): not completed (forwarded raw via
//     {{.CLI_ARGS}}).
//
// Task-target completion sets AI_CABIN_LIFECYCLE_TASKFILE on the process env
// (same channel as runCabinTask) so Setup() resolves the lifecycle include.
// On any error (no cabin selected, unknown cabin, Taskfile parse failure) it
// returns NoFileComp rather than crashing the shell: a completion that errors
// loudly is worse than one that silently offers nothing (the user can still
// type the target).
func completeTaskArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cabinName, err := resolveTargetCabin()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
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
	for _, a := range args {
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
