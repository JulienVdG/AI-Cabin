package config

import (
	"fmt"
	"strings"
)

// ProfileEnvVar selects the profile file. --profile sets it (same mechanism as
// --var), so --profile and AI_CABIN_PROFILE env are one mechanism, not two.
const ProfileEnvVar = "AI_CABIN_PROFILE"

// ResolveVars returns the variable view the CLI sets on its task subprocess:
// CLI overrides (--var, --profile), process env, selected profile file, and
// internal defaults — applied with "set if not present" semantics (highest
// precedence first).
//
// Never errors just because no profile file is selected: a legacy .envrc
// exporting the vars works alone (the env is always part of the view). Errors
// only on an explicitly selected missing/unparseable profile, or a malformed
// --var. The whole process env is included, so system vars (PATH, HOME, ...)
// are preserved; the runner sets the view via os.Setenv without clearing.
//
// InitProfile follows the same precedence rule (--var > env > existing >
// defaults) but is not a caller of ResolveVars: it loads the named profile under
// creation/update (not the selected one) and persists a bounded key set (env
// overrides values but does not enlarge the set), whereas this view includes
// the whole env. See ConfigService.InitProfile for the persistence rule.
func (s *ConfigService) ResolveVars(profileFlag string, cliVars []string) (Vars, error) {
	view := make(Vars)

	// 1. CLI overrides (--var, --profile sets AI_CABIN_PROFILE).
	for _, kv := range cliVars {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --var %q: expected KEY=VAL", kv)
		}
		view[k] = v
	}
	if profileFlag != "" {
		view[ProfileEnvVar] = profileFlag
	}

	// 2. Process env (set if not present, so --var/--profile win). EnvironMap
	// skips empty/whitespace keys (`=value`, seen in some sandboxed envs) and
	// the shell's special `_` variable.
	for k, v := range EnvironMap() {
		if _, present := view[k]; !present {
			view[k] = v
		}
	}

	// Which profile file to load: --profile > AI_CABIN_PROFILE env > current.
	name := view[ProfileEnvVar]
	if name == "" {
		current, err := s.GetCurrentProfile()
		if err != nil {
			return nil, fmt.Errorf("get current profile: %w", err)
		}
		name = current
	}

	// 3. Selected profile file (if any). Missing selection is skipped; an
	// explicitly selected missing file is an error.
	if name != "" {
		// Reflect the resolved profile in the view so setenv exports
		// AI_CABIN_PROFILE even when the profile came from the current-profile
		// fallback — the standalone `task` path then selects the same profile
		// for its compose project name. Direct assignment: `name` already
		// embodies the env/flag precedence, and an empty AI_CABIN_PROFILE in
		// the env is not a selection (it falls back to the current profile).
		view[ProfileEnvVar] = name

		exists, err := s.ProfileExists(name)
		if err != nil {
			return nil, fmt.Errorf("check profile existence: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("profile %q does not exist\nRun 'cabin profile init %s' to create it, or 'cabin profile use <name>' to select another", name, name)
		}
		profile, err := s.LoadProfile(name)
		if err != nil {
			return nil, fmt.Errorf("load profile %q: %w", name, err)
		}
		setIfAbsent(view, profile.Vars)
	}

	// 4. Internal defaults — last resort.
	defaults, err := s.BuildDefaultProfile("")
	if err != nil {
		return nil, fmt.Errorf("build default profile: %w", err)
	}
	setIfAbsent(view, defaults.Vars)

	// 5. Normalize typed vars whose profile/env form is permissive (CSV,
	// JSON array, ...) into the canonical form templates expect. Applied
	// after assembly so --var and env are normalized too, not just profile.
	sanitizeTypedVars(view)

	return view, nil
}

// setIfAbsent copies src into dst for keys not already present (first-set wins).
func setIfAbsent(dst, src map[string]string) {
	for k, v := range src {
		if _, present := dst[k]; !present {
			dst[k] = v
		}
	}
}

// sanitizeTypedVars normalizes vars whose input form is permissive (CSV,
// JSON array, quoted or not) into the canonical form templates expect, so the
// template can consume the var directly. Applied at the end of ResolveVars so
// --var, env, and profile values are all covered. Today: CREDENTIAL_INJECT
// and CREDENTIAL_IGNORE (list normalization via SanitizeNameList, defaulting
// to empty when unset so the template renders []), CONTAINER_WORKDIR (fallback
// to AI_CABIN_WORKDIR).
func sanitizeTypedVars(view Vars) {
	// CREDENTIAL_INJECT/IGNORE are optional: always sanitize so an unset var
	// defaults to empty (renders []), never <no value>.
	view[CredentialInjectEnvVar] = SanitizeNameList(view[CredentialInjectEnvVar])
	view[CredentialIgnoreEnvVar] = SanitizeNameList(view[CredentialIgnoreEnvVar])
	// CONTAINER_WORKDIR is an optional container-side remap of the workdir
	// path (advanced mode); when unset, it falls back to AI_CABIN_WORKDIR
	// (transparent mode). Resolving it here makes the fallback a single
	// source of truth consumed by templates ({{.Vars.CONTAINER_WORKDIR}})
	// and the Taskfile alike, instead of duplicating the shell idiom
	// ${CONTAINER_WORKDIR:-$AI_CABIN_WORKDIR} at each use site.
	if view[ContainerWorkdirVar] == "" {
		view[ContainerWorkdirVar] = view[WorkdirVar]
	}
}
