package config

import (
	"os"
	"path/filepath"
	"strings"
)

// FragmentsDirsEnvVar is the comma-separated list of fragment override
// directories (the conf layer of the fallback chain, highest priority). Each
// entry is ~-expanded (see ExpandHome); first in the list wins (like $PATH).
// Unset/empty means no conf layer (BuildLayers then falls back to cabin-local
// + embed). It is resolved as a profile var (a profile may set it, --var/env
// can override it), so it reaches Vars via the standard ResolveVars path.
const FragmentsDirsEnvVar = "AI_CABIN_FRAGMENTS_DIRS"

// LayerDirsEnvVar is the comma-separated list of layer roots (the root of a
// self-contained override set; a layer contributes <root>/fragments to the
// fragment chain, <root>/skeletons to the skeleton catalogue, and its
// layer.yaml vars: block as profile defaults). Each entry is ~-expanded; first
// in the list wins (like $PATH). Resolved as a profile var so it reaches Vars
// via the standard ResolveVars path and is activated per profile — never
// globally in config.yaml, so a profile does not import another profile's
// layers. Unset/empty means no layer (fragments/skeletons then resolve from
// their own vars + embedded alone).
const LayerDirsEnvVar = "AI_CABIN_LAYER_DIRS"

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

// EnvironMap returns the process environment as a map. It skips entries whose
// key is empty or whitespace-only (e.g. `=value`, seen in some sandboxed envs)
// and the shell's special `_` variable (bash's $_, the last command argument
// — not a real configuration var to propagate). Shared by ResolveVars (the
// env layer of the resolved view), EnvShadowed (profile-override warning) and
// `cabin setenv` (the shell delta), so the exclusion rule lives in one place.
// Pure: only reads the process environment.
func EnvironMap() map[string]string {
	out := make(map[string]string)
	for _, e := range os.Environ() {
		if k, v, ok := strings.Cut(e, "="); ok && k != "_" && strings.TrimSpace(k) != "" {
			out[k] = v
		}
	}
	return out
}

// EnvShadowed reports profile vars shadowed by a same-named process-env var,
// mapped name -> env value. A profile var is shadowed when the env carries the
// same key with a *different* value (an identical echo is not an override),
// because ResolveVars applies "set if not present" with the env as a
// higher-precedence layer than the profile. Used by `cabin profile show` to
// warn about the silent precedence before the resolved view is set on a
// subprocess. Pure: only reads the process environment (via EnvironMap), never
// resolves a profile or consults defaults, so an env-only var never appears
// (it is not a profile var).
func EnvShadowed(profileVars map[string]string) map[string]string {
	env := EnvironMap()
	out := make(map[string]string)
	for k, pv := range profileVars {
		if ev, present := env[k]; present && ev != pv {
			out[k] = ev
		}
	}
	return out
}

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

// LayerDirs resolves the active layer roots from AI_CABIN_LAYER_DIRS in the
// view. A layer is a self-contained override root: <root>/fragments feeds the
// fragment fallback chain, <root>/skeletons feeds the skeleton catalogue, and
// <root>/layer.yaml contributes profile-default vars. Each entry is ~-expanded
// via SplitPathList (same PATH-style convention as FragmentsDirs/SkeletonDirs).
// Returns nil when the var is unset/empty. Pure: no stat, no disk — existence
// of a <root>/<subdir> is checked (and a missing one tolerated) by the caller
// when it builds the layers.
func (v Vars) LayerDirs() []string {
	return SplitPathList(v[LayerDirsEnvVar])
}

// LayerDirsSubdir returns <root>/<subdir> for each active layer root (LayerDirs),
// preserving order (first in the list wins, PATH convention). A layer is a root
// that may carry only some subdirs (the fragment + skeleton contributions are
// independent), so these dirs are used tolerantly by the callers: a missing
// subdir is skipped, not an error. Pure: path join only, no stat.
func (v Vars) LayerDirsSubdir(sub string) []string {
	roots := v.LayerDirs()
	if len(roots) == 0 {
		return nil
	}
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		out = append(out, filepath.Join(r, sub))
	}
	return out
}

// LayerFragmentDirs returns the <root>/fragments dirs derived from the active
// layers (the fragment-chain contribution of a layer). See LayerDirsSubdir.
func (v Vars) LayerFragmentDirs() []string {
	return v.LayerDirsSubdir("fragments")
}

// LayerSkeletonDirs returns the <root>/skeletons dirs derived from the active
// layers (the catalogue contribution of a layer). See LayerDirsSubdir.
func (v Vars) LayerSkeletonDirs() []string {
	return v.LayerDirsSubdir("skeletons")
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
