package config_test

import (
	"os"
	"strings"
	"testing"
)

// TestMain blanks the host environment once, before any test in this package
// runs, so a developer's exported vars cannot leak into assertions on the
// lower-precedence levels (defaults / profile file). The config code reads the
// whole environment as the env layer of the resolved view (EnvironMap), so
// only a clean slate protects every test, known vars or not (e.g. a layer var
// such as DEFAULT_MODEL, or AI_CABIN_PROFILE and OPENCODE_SERVER_PASSWORD
// carried by the shell running `go test`). Each `go test` run is a fresh
// process, so the slate is set once up front and never needs restoring; tests
// that exercise an env override set their var explicitly via t.Setenv, which
// still applies.
//
// A minimal allowlist keeps the process-level vars the test binary and the
// stdlib genuinely need: HOME (os.UserHomeDir), PATH, TMPDIR (os.TempDir for
// t.TempDir) and TERM/LANG/TZ (harmless display/tz defaults).
func TestMain(m *testing.M) {
	keep := map[string]bool{
		"HOME": true, "PATH": true, "TMPDIR": true,
		"TERM": true, "LANG": true, "TZ": true,
	}
	for _, e := range os.Environ() {
		if k, _, ok := strings.Cut(e, "="); ok && k != "_" && !keep[k] {
			_ = os.Unsetenv(k)
		}
	}
	os.Exit(m.Run())
}
