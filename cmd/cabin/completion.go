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

// completeVarNames completes the value of the global --var flag with known
// profile variable names: the union of the current profile's persisted vars
// (the profile selected by --profile, default: current) and the known settable
// keys — formatted as KEY= so the shell leaves the cursor after the '=' for the
// value. Keys already given on the same command line are excluded. Resolution
// errors are silently tolerated (the candidate set simply degrades to what is
// available) so the completion never crashes the shell: the user can still type
// the key by hand.
func completeVarNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	keys := make(map[string]bool, len(knownConfigVarKeys))
	for _, k := range knownConfigVarKeys {
		keys[k] = true
	}
	if prof, err := config.GetActiveProfile(profileFlag); err == nil {
		for k := range prof.Vars {
			keys[k] = true
		}
	}
	// Exclude vars already given on this command line (prior --var entries).
	if given, err := cmd.Flags().GetStringArray("var"); err == nil {
		for _, kv := range given {
			if k, _, ok := strings.Cut(kv, "="); ok {
				delete(keys, k)
			}
		}
	}
	var out []string
	for k := range keys {
		if strings.HasPrefix(k+"=", toComplete) {
			out = append(out, k+"=")
		}
	}
	sort.Strings(out)
	// NoSpace so the shell does not append a trailing space after the single
	// KEY= completion (the cursor is meant to stay right after the '=' for the
	// value). Combined with NoFileComp (a var name is never an arbitrary path).
	return out, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

// knownConfigVarKeys lists the known settable profile variables, forming the
// base of the --var completion so a key is suggested even when the current
// profile has not persisted it. It covers the structural keys and the
// BuildDefaultProfile defaults (AI_CABIN_HOME/DESK/WORKDIR + GIT_AGENT_*,
// the latter hardcoded as names because completion needs the key, not the value).
var knownConfigVarKeys = [...]string{
	config.HomeVar,
	config.DeskVar,
	config.WorkdirVar,
	config.ContainerWorkdirVar,
	config.FragmentsDirsEnvVar,
	config.LayerDirsEnvVar,
	config.SkeletonDirsEnvVar,
	config.CredentialInjectEnvVar,
	config.CredentialIgnoreEnvVar,
	"GIT_AGENT_NAME",
	"GIT_AGENT_EMAIL",
}

// completeProfileNames completes the value of the --profile flag and the
// positional of `cabin profile use|show <name>`. Same shape as
// completeCabinNames (prefix filter + exclude already-provided names +
// NoFileComp): a profile is a registry entry, never an arbitrary path.
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
