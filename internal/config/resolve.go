package config

import (
	"fmt"
	"os"
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
func (s *ConfigService) ResolveVars(profileFlag string, cliVars []string) (map[string]string, error) {
	view := make(map[string]string)

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

	// 2. Process env (set if not present, so --var/--profile win). Skip empty
	// or whitespace-only keys (e.g. `=value`, seen in some sandboxed envs).
	for _, e := range os.Environ() {
		if k, v, ok := strings.Cut(e, "="); ok && strings.TrimSpace(k) != "" {
			if _, present := view[k]; !present {
				view[k] = v
			}
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
