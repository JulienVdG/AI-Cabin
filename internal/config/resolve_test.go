package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/config"

	mock_config "github.com/JulienVdG/AI-Cabin/internal/mocks/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newResolveService builds a ConfigService on a temp config dir with mocked
// git/home providers, so BuildDefaultProfile returns deterministic defaults
// (home=/tmp/cabin-home, git identity fixed) rather than the real host's.
func newResolveService(t *testing.T) *config.ConfigService {
	t.Helper()
	setupTestConfig(t)

	configDir, err := config.GetConfigDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, config.ProfilesDirName), 0755))

	mockGit := &mock_config.GitConfigProvider{}
	mockGit.On("GetUserName").Return("Test User", nil)
	mockGit.On("GetUserEmail").Return("test@example.com", nil)
	mockHomeDir := &mock_config.HomeDirProvider{}
	mockHomeDir.On("GetHomeDir").Return("/tmp/cabin-home", nil)

	return config.NewConfigService(mockGit, mockHomeDir, os.DirFS(configDir), config.AtomicFileWriter{})
}

// cabinEnvVars are the AI-Cabin vars the test process may carry (from a
// .envrc loaded by the shell running `go test`). They must be unset to assert
// the lower-precedence levels (defaults / profile file) — env wins per axis B
// (correct behavior), so a leaked env var masks the value under test.
var cabinEnvVars = []string{
	"AI_CABIN_HOME", "AI_CABIN_DESK", "AI_CABIN_WORKDIR",
	"CONTAINER_WORKDIR",
	"GIT_AGENT_NAME", "GIT_AGENT_EMAIL", "SCW_PROJECT_ID",
	"AI_CABIN_PROFILE", "CABIN_TEST_VAR", "AI_CABIN_CURRENT_CABIN",
	"CREDENTIAL_INJECT", "CREDENTIAL_IGNORE",
}

// unsetCabinEnv unsets the AI-Cabin env vars for the test duration, restoring
// their previous state (set or unset) on cleanup. t.Setenv cannot unset (only
// set to a value, which still overrides), so this manual capture/restore is
// the only way to verify defaults/profile levels under a polluted test env.
func unsetCabinEnv(t *testing.T) {
	t.Helper()
	saved := make(map[string]string, len(cabinEnvVars))
	wasSet := make(map[string]bool, len(cabinEnvVars))
	for _, k := range cabinEnvVars {
		if v, ok := os.LookupEnv(k); ok {
			saved[k] = v
			wasSet[k] = true
		}
		_ = os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for _, k := range cabinEnvVars {
			if wasSet[k] {
				_ = os.Setenv(k, saved[k])
			}
		}
	})
}

// writeProfile writes a profile yaml to the temp config dir.
func writeProfile(t *testing.T, name, varsYAML string) {
	t.Helper()
	profilesDir, err := config.GetProfilesDir()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(profilesDir, name+".yaml"), []byte(varsYAML), 0644))
}

// resolveCase is one TestResolveVars sub-case. The setup (profiles written,
// current profile, env, clean-env flag) is shared by the loop; each case
// asserts on a single want var/err to keep the table readable.
type resolveCase struct {
	name          string
	profileFlag   string            // --profile flag
	cliVars       []string          // --var KEY=VAL flags
	profiles      map[string]string // name -> vars yaml body to write
	current       string            // profile to select via SetCurrentProfile
	env           map[string]string // env vars to set (t.Setenv)
	cleanEnv      bool              // unset cabinEnvVars first (assert lower levels)
	wantVar       string            // var to assert
	wantVal       string            // expected value for wantVar
	wantErrSubstr string            // non-empty -> expect an error containing this
}

// TestResolveVars covers the two-axis model: axis A selects the profile file
// (--profile > AI_CABIN_PROFILE env > config.yaml currentProfile), axis B
// merges defaults < profile < env < --var (highest first, "set if not
// present"). Table-driven so adding a case is one entry.
func TestResolveVars(t *testing.T) {
	cases := []resolveCase{
		{
			name:     "defaults only when no profile and no env override",
			cleanEnv: true,
			wantVar:  "AI_CABIN_HOME",
			wantVal:  "/tmp/cabin-home",
		},
		{
			name:     "defaults only — git identity derived",
			cleanEnv: true,
			wantVar:  "GIT_AGENT_NAME",
			wantVal:  "AI Agent + Test User",
		},
		{
			// Profile file merges on top of defaults. Uses a synthetic var so
			// the assertion does not collide with a leaked env var.
			name:     "profile file overrides defaults",
			cleanEnv: true,
			profiles: map[string]string{"perso": "name: perso\nvars:\n  CABIN_TEST_VAR: profile-val\n  SCW_PROJECT_ID: proj-123\n"},
			current:  "perso",
			wantVar:  "CABIN_TEST_VAR",
			wantVal:  "profile-val",
		},
		{
			// Env wins over profile and defaults (axis B level 2 > 3 > 4).
			name:     "env overrides profile and defaults",
			profiles: map[string]string{"perso": "name: perso\nvars:\n  CABIN_TEST_VAR: profile-val\n"},
			current:  "perso",
			env:      map[string]string{"CABIN_TEST_VAR": "env-val"},
			wantVar:  "CABIN_TEST_VAR",
			wantVal:  "env-val",
		},
		{
			// --var wins over env, profile, defaults (axis B level 1, highest).
			name:    "--var overrides env",
			env:     map[string]string{"CABIN_TEST_VAR": "env-val"},
			cliVars: []string{"CABIN_TEST_VAR=var-val"},
			wantVar: "CABIN_TEST_VAR",
			wantVal: "var-val",
		},
		{
			// --var format validation.
			name:          "--var rejects malformed input",
			cliVars:       []string{"NO_EQUALS_SIGN"},
			wantErrSubstr: "expected KEY=VAL",
		},
		{
			// --var with empty key is rejected (same guard the env path uses
			// to skip malformed `=value` / `   =value` entries).
			name:          "--var rejects empty key",
			cliVars:       []string{"=value"},
			wantErrSubstr: "expected KEY=VAL",
		},
		{
			// Axis A: --profile > AI_CABIN_PROFILE env.
			name:     "axis A --profile wins over AI_CABIN_PROFILE",
			cleanEnv: true,
			profiles: map[string]string{
				"via-env":  "name: via-env\nvars:\n  CABIN_TEST_VAR: env-picked\n",
				"via-flag": "name: via-flag\nvars:\n  CABIN_TEST_VAR: flag-picked\n",
			},
			env:         map[string]string{"AI_CABIN_PROFILE": "via-env"},
			profileFlag: "via-flag",
			wantVar:     "CABIN_TEST_VAR",
			wantVal:     "flag-picked",
		},
		{
			// Axis A: AI_CABIN_PROFILE env > config.yaml currentProfile.
			name:     "axis A AI_CABIN_PROFILE wins over currentProfile",
			cleanEnv: true,
			profiles: map[string]string{
				"current": "name: current\nvars:\n  CABIN_TEST_VAR: current-picked\n",
				"via-env": "name: via-env\nvars:\n  CABIN_TEST_VAR: env-picked\n",
			},
			current: "current",
			env:     map[string]string{"AI_CABIN_PROFILE": "via-env"},
			wantVar: "CABIN_TEST_VAR",
			wantVal: "env-picked",
		},
		{
			// Explicit selection of a missing profile file is an error (no
			// silent fallback — the user asked for it).
			name:          "explicit missing profile errors",
			cleanEnv:      true,
			profileFlag:   "does-not-exist",
			wantErrSubstr: "does not exist",
		},
		{
			// --profile sets AI_CABIN_PROFILE in the resolved view (same
			// mechanism as --var), so a subprocess (e.g. `cabin internal
			// setup` under the $MAKE pattern) inherits the selected profile.
			// This unifies --profile and AI_CABIN_PROFILE env into one mech.
			name:        "--profile sets AI_CABIN_PROFILE in the view",
			cleanEnv:    true,
			profiles:    map[string]string{"perso": "name: perso\nvars:\n  CABIN_TEST_VAR: from-profile\n"},
			profileFlag: "perso",
			wantVar:     "AI_CABIN_PROFILE",
			wantVal:     "perso",
		},
		{
			// Same reflection for the current-profile fallback (no --profile /
			// AI_CABIN_PROFILE): `cabin setenv` must still export
			// AI_CABIN_PROFILE so the standalone `task` path selects the same
			// profile for its compose project name.
			name:     "current profile sets AI_CABIN_PROFILE in the view",
			cleanEnv: true,
			profiles: map[string]string{"perso": "name: perso\nvars:\n  CABIN_TEST_VAR: from-profile\n"},
			current:  "perso",
			wantVar:  "AI_CABIN_PROFILE",
			wantVal:  "perso",
		},
		{
			// AI_CABIN_PROFILE sourced from the env is reflected too (axis A), so
			// `cabin setenv` exports back the profile the env selected.
			name:     "env AI_CABIN_PROFILE reflected in the view",
			cleanEnv: true,
			profiles: map[string]string{"via-env": "name: via-env\nvars:\n  CABIN_TEST_VAR: x\n"},
			env:      map[string]string{"AI_CABIN_PROFILE": "via-env"},
			wantVar:  "AI_CABIN_PROFILE",
			wantVal:  "via-env",
		},
		{
			// An exported-but-empty AI_CABIN_PROFILE is not a selection (it falls
			// back to the current profile); the view carries the resolved name.
			name:     "empty AI_CABIN_PROFILE env falls back to current profile",
			cleanEnv: true,
			profiles: map[string]string{"current": "name: current\nvars:\n  CABIN_TEST_VAR: x\n"},
			current:  "current",
			env:      map[string]string{"AI_CABIN_PROFILE": ""},
			wantVar:  "AI_CABIN_PROFILE",
			wantVal:  "current",
		},
		{
			// CONTAINER_WORKDIR unset falls back to AI_CABIN_WORKDIR
			// (transparent mode) — resolved in sanitizeTypedVars so templates
			// and the Taskfile read CONTAINER_WORKDIR directly.
			name:     "container_workdir falls back to AI_CABIN_WORKDIR when unset",
			cleanEnv: true,
			wantVar:  "CONTAINER_WORKDIR",
			wantVal:  filepath.Join("/tmp/cabin-home", "projects"),
		},
		{
			// CONTAINER_WORKDIR set (container-side remap, advanced mode)
			// wins over AI_CABIN_WORKDIR.
			name:     "container_workdir set wins over AI_CABIN_WORKDIR",
			cleanEnv: true,
			env:      map[string]string{"CONTAINER_WORKDIR": "/workspace"},
			wantVar:  "CONTAINER_WORKDIR",
			wantVal:  "/workspace",
		},
		{
			// CREDENTIAL_INJECT unset defaults to empty (renders [] in the
			// template), never <no value> — sanitizeTypedVars always sanitizes.
			name:     "credential_inject defaults to empty when unset",
			cleanEnv: true,
			wantVar:  "CREDENTIAL_INJECT",
			wantVal:  "",
		},
		{
			// CREDENTIAL_IGNORE unset defaults to empty (same as inject).
			name:     "credential_ignore defaults to empty when unset",
			cleanEnv: true,
			wantVar:  "CREDENTIAL_IGNORE",
			wantVal:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cleanEnv {
				unsetCabinEnv(t)
			}
			svc := newResolveService(t)
			for name, body := range tc.profiles {
				writeProfile(t, name, body)
			}
			if tc.current != "" {
				require.NoError(t, svc.SetCurrentProfile(tc.current))
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			vars, err := svc.ResolveVars(tc.profileFlag, tc.cliVars)
			if tc.wantErrSubstr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantVal, vars[tc.wantVar])
		})
	}
}

// TestResolveCabin covers the cabin-scoped target resolution used by
// up/down/build/shell/greyshell/logs/restart/task: --cabin flag > env
// AI_CABIN_CURRENT_CABIN > active profile var (set with `cabin use`). The
// active profile is the one selected by --profile / AI_CABIN_PROFILE /
// config.yaml (axis A), so the current cabin is genuinely per-profile.
func TestResolveCabin(t *testing.T) {
	cases := []struct {
		name        string
		cabinFlag   string
		profileFlag string
		profiles    map[string]string // name -> vars yaml body
		current     string
		env         map[string]string
		want        string
		wantErr     bool
	}{
		{
			// The --cabin flag is explicit and wins over every other level.
			name:      "--cabin flag wins over env and profile var",
			cabinFlag: "pi-go",
			env:       map[string]string{"AI_CABIN_CURRENT_CABIN": "opencode-go"},
			profiles:  map[string]string{"default": "name: default\nvars:\n  AI_CABIN_CURRENT_CABIN: blog\n"},
			current:   "default",
			want:      "pi-go",
		},
		{
			// Env outranks the profile file, matching ResolveVars' axis B.
			name:     "env AI_CABIN_CURRENT_CABIN wins over profile var",
			env:      map[string]string{"AI_CABIN_CURRENT_CABIN": "opencode-go"},
			profiles: map[string]string{"default": "name: default\nvars:\n  AI_CABIN_CURRENT_CABIN: blog\n"},
			current:  "default",
			want:     "opencode-go",
		},
		{
			// No flag, no env: the active profile's var is the current cabin.
			name:     "current cabin from active profile var",
			profiles: map[string]string{"default": "name: default\nvars:\n  AI_CABIN_CURRENT_CABIN: blog\n"},
			current:  "default",
			want:     "blog",
		},
		{
			// The current cabin is per-profile: --profile selects which
			// profile's var is read.
			name:        "current cabin from an explicitly selected profile",
			profileFlag: "work",
			profiles:    map[string]string{"work": "name: work\nvars:\n  AI_CABIN_CURRENT_CABIN: prod-go\n"},
			want:        "prod-go",
		},
		{
			// Nothing names a cabin: ResolveCabin errors with guidance.
			name:    "no cabin selected errors",
			current: "default",
			profiles: map[string]string{
				"default": "name: default\nvars:\n  AI_CABIN_WORKDIR: /tmp/work\n",
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			unsetCabinEnv(t)
			svc := newResolveService(t)
			for name, body := range tc.profiles {
				writeProfile(t, name, body)
			}
			if tc.current != "" {
				require.NoError(t, svc.SetCurrentProfile(tc.current))
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			got, err := svc.ResolveCabin(tc.cabinFlag, tc.profileFlag)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
