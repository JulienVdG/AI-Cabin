package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSplitPathList covers the PATH-style primitive shared by config vars
// (today: AI_CABIN_FRAGMENTS_DIRS via Vars.FragmentsDirs): comma split,
// whitespace trim, empty-entry drop, ~ expansion, and the shell-consistent
// behavior of leaving an unresolved ~user untouched.
func TestSplitPathList(t *testing.T) {
	// ~ expansion reads $HOME; pin it so the expanded path is deterministic.
	home := t.TempDir()
	t.Setenv("HOME", home)

	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"EmptyIsNil", "", nil},
		{"Single", "/etc/fragments", []string{"/etc/fragments"}},
		{"CommaSeparated", "/a,/b,/c", []string{"/a", "/b", "/c"}},
		{"TrimsWhitespace", " /a , /b ,", []string{"/a", "/b"}},
		{"DropsEmptyEntries", "/a,,/b,", []string{"/a", "/b"}},
		{"TildeExpanded", "~/fragments", []string{filepath.Join(home, "fragments")}},
		{"UnresolvedTildeUserKeptAsIs", "~__no_such_user__/fragments", []string{"~__no_such_user__/fragments"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, config.SplitPathList(tc.in))
		})
	}
}

// TestSanitizeNameList covers the normalization of the CREDENTIAL_INJECT and
// CREDENTIAL_IGNORE profile/env vars into the raw content of a JSON array
// (the greywall.json.tmpl wraps them as "inject": [{{.Vars.CREDENTIAL_INJECT}}]
// and "ignore": [{{.Vars.CREDENTIAL_IGNORE}}]). Accepts CSV, JSON array, or
// bracketed CSV — quoted or not; quotes are stripped unconditionally (env
// var names carry none) and empty entries drop.
func TestSanitizeNameList(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"EmptyIsEmpty", "", ""},
		{"Single", "A", "\"A\""},
		{"Csv", "A,B", "\"A\",\"B\""},
		{"JsonArray", `["A","B"]`, "\"A\",\"B\""},
		{"BracketedCsv", "[A,B]", "\"A\",\"B\""},
		{"DropsEmptyEntries", "A,,B,", "\"A\",\"B\""},
		{"TrimsWhitespace", " A , B ", "\"A\",\"B\""},
		{"StripsQuotes", "'A',\"B\"", "\"A\",\"B\""},
		{"QuotedCommaInsideSplits", `["A,B"]`, "\"A\",\"B\""},
		{"OnlyEmptyIsEmpty", ",,", ""},
		{"BracketedEmpty", "[]", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, config.SanitizeNameList(tc.in))
		})
	}
}

// TestEnvShadowed covers the warning helper behind `cabin profile show`: a
// profile var is shadowed when a same-named process-env var carries a value
// (env wins over profile in the resolved view). Keys are isolated to the test
// so the surrounding process env (or a leak from another case) cannot make an
// assertion flaky.
func TestEnvShadowed(t *testing.T) {
	const (
		shadowKey  = "CABIN_TEST_SHADOWED"
		atanKey    = "CABIN_TEST_ABSENT"
		envOnlyKey = "CABIN_TEST_ENV_ONLY"
	)
	clearKeys := func() { unsetEnv(t, shadowKey, atanKey, envOnlyKey) }

	t.Run("same-named env var shadows a profile var", func(t *testing.T) {
		clearKeys()
		t.Setenv(shadowKey, "from-env")
		out := config.EnvShadowed(map[string]string{shadowKey: "from-profile"})
		require.Equal(t, map[string]string{shadowKey: "from-env"}, out)
	})

	t.Run("profile var without env var is not shadowed", func(t *testing.T) {
		clearKeys()
		out := config.EnvShadowed(map[string]string{atanKey: "profile-only"})
		assert.Empty(t, out)
	})

	t.Run("identical env echo is not an override", func(t *testing.T) {
		clearKeys()
		t.Setenv(shadowKey, "same")
		out := config.EnvShadowed(map[string]string{shadowKey: "same"})
		assert.Empty(t, out)
	})

	t.Run("env-only var (not a profile var) never appears", func(t *testing.T) {
		clearKeys()
		t.Setenv(envOnlyKey, "x")
		out := config.EnvShadowed(map[string]string{atanKey: "y"})
		assert.Empty(t, out)
	})

	t.Run("multiple shadowed vars returned as a map", func(t *testing.T) {
		clearKeys()
		t.Setenv(shadowKey, "e1")
		t.Setenv(envOnlyKey, "e2")
		out := config.EnvShadowed(map[string]string{shadowKey: "p1", envOnlyKey: "p2"})
		assert.Equal(t, map[string]string{shadowKey: "e1", envOnlyKey: "e2"}, out)
	})

	t.Run("empty profile has no shadowed vars", func(t *testing.T) {
		out := config.EnvShadowed(nil)
		assert.Empty(t, out)
	})
}

// TestEnvironMap covers the shared process-env reader: normal vars pass
// through and the shell's special `_` is excluded.
func TestEnvironMap(t *testing.T) {
	t.Setenv("CABIN_TEST_KEEP", "v")
	t.Setenv("_", "junk")
	defer func() {
		_ = os.Unsetenv("CABIN_TEST_KEEP")
		_ = os.Unsetenv("_")
	}()

	env := config.EnvironMap()
	assert.Equal(t, "v", env["CABIN_TEST_KEEP"])
	_, hasUnderscore := env["_"]
	assert.False(t, hasUnderscore)
}
