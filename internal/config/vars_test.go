package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

// TestSanitizeNameList covers the normalization of the CREDENTIAL_INJECT
// and CREDENTIAL_IGNORE profile/env vars into the raw content of a JSON array
// (the greywall.json.tmpl wraps them as "inject": [{{.Vars.CREDENTIAL_INJECT}}]
// and "ignore": [{{.Vars.CREDENTIAL_IGNORE}}]). Accepts CSV, JSON array,
// or bracketed CSV — quoted or not; quotes are stripped unconditionally
// (env var names carry none) and empty entries drop.
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

// TestEnvShadowed covers the warning helper behind `cabin profile show`:
// a profile var is shadowed when a same-named process-env var carries a value
// (env wins over profile in the resolved view). Keys are isolated to the test
// so the surrounding process env (or a leak from another case) cannot make
// an assertion flaky.
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

// TestLayerDirs covers the layer-root parsing (AI_CABIN_LAYER_DIRS)
// and the <root>/<subdir> derivation shared by the fragment-chain
// and skeleton-catalogue contributions of a layer. LayerDirs reuses
// SplitPathList (comma split + ~ expansion, covered there), so here we only
// pin the subdir join and the empty/no-layer cases.
func TestLayerDirs(t *testing.T) {
	t.Run("EmptyIsNoLayer", func(t *testing.T) {
		v := config.Vars{}
		assert.Nil(t, v.LayerDirs())
		assert.Nil(t, v.LayerFragmentDirs())
		assert.Nil(t, v.LayerSkeletonDirs())
	})

	t.Run("ParsesAndJoinsSubdirs", func(t *testing.T) {
		v := config.Vars{config.LayerDirsEnvVar: "/layers/a,/layers/b"}
		assert.Equal(t, []string{"/layers/a", "/layers/b"}, v.LayerDirs())
		assert.Equal(t, []string{"/layers/a/fragments", "/layers/b/fragments"}, v.LayerFragmentDirs())
		assert.Equal(t, []string{"/layers/a/skeletons", "/layers/b/skeletons"}, v.LayerSkeletonDirs())
	})
}

// TestOrderedVarKeys covers the canonical key order shared by the profile
// YAML marshal and every CLI display of a var map: HomeVar and DeskVar
// first, then the remaining AI_CABIN_* keys, then everything else —
// alphabetical within each rank.
func TestOrderedVarKeys(t *testing.T) {
	t.Run("CanonicalThenAlphabetical", func(t *testing.T) {
		vars := map[string]string{
			"SCW_PROJECT_ID":         "id",
			config.WorkdirVar:        "/projects",
			config.HomeVar:           "/home",
			"GIT_AGENT_NAME":         "agent",
			config.DeskVar:           "/desk",
			"AI_CABIN_CURRENT_CABIN": "blog",
			"CREDENTIAL_INJECT":      "SCW",
		}
		assert.Equal(t, []string{
			config.HomeVar,
			config.DeskVar,
			"AI_CABIN_CURRENT_CABIN",
			config.WorkdirVar,
			"CREDENTIAL_INJECT",
			"GIT_AGENT_NAME",
			"SCW_PROJECT_ID",
		}, config.OrderedVarKeys(vars))
	})

	t.Run("MissingCanonicalKeysKeepPrefixRank", func(t *testing.T) {
		vars := map[string]string{"Z": "1", "AI_CABIN_X": "2", "A": "3"}
		assert.Equal(t, []string{"AI_CABIN_X", "A", "Z"}, config.OrderedVarKeys(vars))
	})

	t.Run("Empty", func(t *testing.T) {
		assert.Empty(t, config.OrderedVarKeys(map[string]string{}))
	})
}

// TestVars_MarshalYAML pins the persisted profile var order: the mapping
// leads with AI_CABIN_HOME and AI_CABIN_DESK instead of the alphabetical
// order yaml.v3 picks for a bare map, and special values still round-trip
// through the hand-built node tree.
func TestVars_MarshalYAML(t *testing.T) {
	t.Run("CanonicalOrder", func(t *testing.T) {
		vars := config.Vars{
			"GIT_AGENT_NAME":  "agent",
			config.WorkdirVar: "/projects",
			config.DeskVar:    "/desk",
			config.HomeVar:    "/home",
		}
		data, err := yaml.Marshal(vars)
		require.NoError(t, err)
		assert.Equal(t,
			"AI_CABIN_HOME: /home\nAI_CABIN_DESK: /desk\nAI_CABIN_WORKDIR: /projects\nGIT_AGENT_NAME: agent\n",
			string(data))
	})

	t.Run("EmptyMarshalsAsFlowMapping", func(t *testing.T) {
		data, err := yaml.Marshal(config.Vars{})
		require.NoError(t, err)
		assert.Equal(t, "{}\n", string(data))
	})

	t.Run("SpecialValuesRoundTrip", func(t *testing.T) {
		vars := config.Vars{"K": "a: b", "J": "#notcomment"}
		data, err := yaml.Marshal(vars)
		require.NoError(t, err)
		back := map[string]string{}
		require.NoError(t, yaml.Unmarshal(data, &back))
		assert.Equal(t, map[string]string(vars), back)
	})
}
