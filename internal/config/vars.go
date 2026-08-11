package config

import "strings"

// FragmentsDirsEnvVar is the comma-separated list of fragment override
// directories (the conf layer of the fallback chain, highest priority). Each
// entry is ~-expanded (see ExpandHome); first in the list wins (like $PATH).
// Unset/empty means no conf layer (BuildLayers then falls back to cabin-local
// + embed). It is resolved as a profile var (a profile may set it, --var/env
// can override it), so it reaches Vars via the standard ResolveVars path.
const FragmentsDirsEnvVar = "AI_CABIN_FRAGMENTS_DIRS"

// SkeletonDirsEnvVar is the comma-separated list of skeleton directories (the
// conf layer of the Class 1 skeleton fallback chain, highest priority). Each
// entry is ~-expanded; first in the list wins (like $PATH). Unset/empty means
// no conf layer (BuildLayers then falls back to the embedded catalogue alone,
// e.g. the `minimal` desk). Resolved as a profile var so it reaches Vars via
// the standard ResolveVars path. Skeletons are resolved by name from the union
// of these dirs and embedded.Skeletons(); there is no path mode.
const SkeletonDirsEnvVar = "AI_CABIN_SKELETON_DIRS"

// CredentialInjectEnvVar is the list of credential labels greywall asks greyproxy
// to inject into the sandbox (greywall.json "credentials.inject"). It is set
// as a profile var in a permissive form (CSV, JSON array, or bracketed CSV —
// quoted or not) and normalized at resolve time into the raw content of a
// JSON array ("A","B", or empty) by SanitizeNameList, so the
// greywall.json.tmpl renders it as "inject": [{{.Vars.CREDENTIAL_INJECT}}]
// (empty -> "inject": []). Env var names carry no quotes, so quotes in the
// input are stripped unconditionally.
const CredentialInjectEnvVar = "CREDENTIAL_INJECT"

// CredentialIgnoreEnvVar is the list of env var names greywall must NOT treat
// as credentials (greywall.json "credentials.ignore") — e.g. an env var
// that looks like a secret label but is a benign config the agent needs in
// the clear. Same permissive input form and same normalization as inject.
const CredentialIgnoreEnvVar = "CREDENTIAL_IGNORE"

// HomeVar is the agent home directory (bind-mount target for pi/opencode
// agent configs, the setup-facet destination root). DeskVar is the desk
// directory (agent instructions, skills). WorkdirVar is the host-side workdir
// (git repositories). ContainerWorkdirVar is the optional container-side remap
// of that path (advanced mode, e.g. /workspace); when unset it falls back to
// WorkdirVar (transparent mode) — resolved in sanitizeTypedVars so templates
// read CONTAINER_WORKDIR directly.
const (
	HomeVar             = "AI_CABIN_HOME"
	DeskVar             = "AI_CABIN_DESK"
	WorkdirVar          = "AI_CABIN_WORKDIR"
	ContainerWorkdirVar = "CONTAINER_WORKDIR"
)

// SanitizeNameList normalizes a list-of-env-var-names value into the raw
// content of a JSON array ("A","B", or empty when no entries survive).
// Accepts CSV (A,B), JSON array (["A","B"]), or bracketed CSV ([A,B]);
// quotes (" or ') are stripped unconditionally from each entry (env var
// names carry no quotes), whitespace is trimmed, and empty entries are dropped.
// Pure: no I/O. Used for CREDENTIAL_INJECT and CREDENTIAL_IGNORE, which the
// greywall.json.tmpl renders as "inject": [{{.Vars.CREDENTIAL_INJECT}}] and
// "ignore": [{{.Vars.CREDENTIAL_IGNORE}}] (empty -> []).
func SanitizeNameList(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return ""
	}
	var items []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, "\"'")
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	if len(items) == 0 {
		return ""
	}
	quoted := make([]string, len(items))
	for i, it := range items {
		quoted[i] = "\"" + it + "\""
	}
	return strings.Join(quoted, ",")
}

// Vars is the resolved variable view the CLI sets on its subprocesses: profile
// vars, process env, and --var overrides merged with first-set-wins semantics
// (see ResolveVars). Methods derive higher-level values from the view, keeping
// the derivation logic next to the data it reads instead of spreading it
// across callers.
type Vars map[string]string

// AsMap returns the underlying map for pass-through to APIs that take a raw
// map[string]string (e.g. task.Run sets each entry on os.Setenv without reading
// any key, so internal/task does not need to depend on the Vars type).
func (v Vars) AsMap() map[string]string { return v }

// FragmentsDirs resolves the fragment override directories from
// AI_CABIN_FRAGMENTS_DIRS in the view. Each entry is ~-expanded via
// SplitPathList. Returns nil when the var is unset/empty (BuildLayers then
// falls back to cabin-local + embed). Pure: no stat, no disk — existence is
// validated by BuildLayers when it builds the os.DirFS layers.
func (v Vars) FragmentsDirs() []string {
	return SplitPathList(v[FragmentsDirsEnvVar])
}

// SkeletonDirs resolves the skeleton directories from AI_CABIN_SKELETON_DIRS in
// the view. Each entry is ~-expanded via SplitPathList. Returns nil when the var
// is unset/empty (BuildLayers then falls back to the embedded catalogue alone).
// Pure: no stat, no disk — existence is validated by BuildLayers.
func (v Vars) SkeletonDirs() []string {
	return SplitPathList(v[SkeletonDirsEnvVar])
}

// SplitPathList splits a comma-separated path list, trims whitespace, and
// expands a leading "~"/"~user" per entry (ExpandHome). Empty entries are
// dropped. Returns nil for an empty string so the caller's range is a no-op.
// This is the shared primitive for PATH-style config vars (e.g.
// AI_CABIN_FRAGMENTS_DIRS), kept pure and side-effect-free for unit testing.
func SplitPathList(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, ExpandHome(part))
	}
	return out
}
