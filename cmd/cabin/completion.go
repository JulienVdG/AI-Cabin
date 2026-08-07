package main

import (
	"sort"
	"strings"

	"github.com/JulienVdG/AI-Cabin/internal/config"

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

// completeTaskCabin completes only the first positional (the cabin name) of
// `cabin task <cabin> <task> [params]`. The <task> positional (2nd) is not
// completed in v1 (wiring task's own target completion is a future
// enhancement, §7); trailing agent params are forwarded raw via
// {{.CLI_ARGS}} and are not completed either. Returns NoFileComp past the 1st
// positional so the shell does not fall back to filenames.
func completeTaskCabin(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) >= 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeCabinNames(cmd, args, toComplete)
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
