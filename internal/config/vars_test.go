package config_test

import (
	"path/filepath"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/config"

	"github.com/stretchr/testify/assert"
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
