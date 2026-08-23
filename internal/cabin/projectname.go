package cabin

import "strings"

// CanonicalName returns the cabin's canonical name derived from its path: the
// ai-cabin.cabin header field when set, else the directory basename. It is the
// identity used to namespace per-instance artifacts (the compose project name,
// the shared image name) so two profiles operating the same cabin share the
// build but not the runtime instance. Derived from the path rather than the
// registry so the CLI and standalone `task` resolve the same name.
func CanonicalName(path string) (string, error) {
	name, _, err := ValidateCabin(path, "")
	return name, err
}

// ComposeProjectName derives the docker compose project name from the active
// profile name and the cabin's canonical name, isolating instances per
// (profile, cabin): profile 1 and profile 2 operating the same cabin get
// distinct projects (distinct containers and networks) while sharing the
// image build.
//
// When no profile is active (no current profile selected), the project name is
// the canonical name alone. A single-instance setup does not need the profile
// disambiguator, and compose's own default is also the directory basename, so
// the CLI and the lifecycle `sh:` fallback resolve the same name. This
// preserves the CLI/`task` convergence: the CLI never errors merely for a
// missing profile, and both invocation paths produce the same project name.
func ComposeProjectName(profile, canonical string) string {
	if profile == "" {
		return sanitizeProjectName(canonical)
	}
	return sanitizeProjectName(profile) + "_" + sanitizeProjectName(canonical)
}

// DeriveProfile extracts the profile name from a compose project name built by
// ComposeProjectName (the com.docker.compose.project label). Returns "" when the
// project carries no profile (canonical-only form, e.g. a standalone `task` run
// with no current profile) or when the project does not match the expected
// <profile>_<canonical> shape (a manually-set COMPOSE_PROJECT_NAME), so `cabin
// ps` shows an empty profile rather than a misleading one. The canonical name
// is the cabin name as written in the header (possibly uppercase or dotted);
// the project name holds its sanitized form, so the comparison sanitizes the
// canonical rather than string-comparing raw.
func DeriveProfile(project, canonical string) string {
	canon := sanitizeProjectName(canonical)
	if project == canon {
		return "" // canonical-only form: no profile selected
	}
	suffix := "_" + canon
	if !strings.HasSuffix(project, suffix) {
		return "" // not a <profile>_<canonical> project: cannot infer safely
	}
	return strings.TrimSuffix(project, suffix)
}

// sanitizeProjectName lowercases s and replaces every character outside the
// compose project name charset [a-z0-9_-] with a dash, then trims leading and
// trailing separators so the first character is alphanumeric (compose rejects
// a project name starting with '_' or '-'). An empty result falls back to
// "cabin" so the name is never blank.
func sanitizeProjectName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "cabin"
	}
	return out
}
