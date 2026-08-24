package main

import (
	"reflect"
	"testing"
)

// TestEmitEnvVar covers the per-shell statement format of `cabin setenv
// <shell>`: bash and zsh share the export form, fish uses set -gx. Values are
// single-quoted so $, backticks and history are never expanded — critical
// because setenv materializes credentials.
func TestEmitEnvVar(t *testing.T) {
	cases := []struct {
		name, shell, key, value, want string
	}{
		{"BashExport", "bash", "AI_CABIN_HOME", "/home/ai_agent", `export AI_CABIN_HOME='/home/ai_agent'`},
		{"ZshExport", "zsh", "SCW_PROJECT_ID", "proj-1", `export SCW_PROJECT_ID='proj-1'`},
		{"FishSetGx", "fish", "AI_CABIN_HOME", "/home/ai_agent", `set -gx AI_CABIN_HOME '/home/ai_agent'`},
		{"BashDoubleQuoteLiteral", "bash", "K", `a"b`, `export K='a"b'`},
		{"BashDollarNotExpanded", "bash", "K", `pa$ss`, `export K='pa$ss'`},
		{"BashBacktickNotExpanded", "bash", "K", "`id`", "export K='`id`'"},
		{"BashEmbeddedQuote", "bash", "K", `it's`, `export K='it'\''s'`},
		{"FishEmbeddedQuote", "fish", "K", `it's`, `set -gx K 'it\'s'`},
		{"FishBackslashLiteral", "fish", "K", `a\b`, `set -gx K 'a\\b'`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := emitEnvVar(tc.shell, tc.key, tc.value); got != tc.want {
				t.Errorf("emitEnvVar(%q, %q, %q) = %q, want %q", tc.shell, tc.key, tc.value, got, tc.want)
			}
		})
	}
}

// TestSetenvDelta covers the shell-delta selection: a variable the shell env
// already carries unchanged is a no-op, a resolved empty value is skipped, and
// only what materializes or changes is emitted, sorted.
func TestSetenvDelta(t *testing.T) {
	cases := []struct {
		name string
		view map[string]string
		env  map[string]string
		want []string
	}{
		{
			"absent var is emitted",
			map[string]string{"A": "x"},
			map[string]string{},
			[]string{"A"},
		},
		{
			"same-value env var is skipped",
			map[string]string{"A": "x"},
			map[string]string{"A": "x"},
			[]string{},
		},
		{
			"different value is emitted (override)",
			map[string]string{"A": "x"},
			map[string]string{"A": "y"},
			[]string{"A"},
		},
		{
			"empty resolved value is skipped",
			map[string]string{"A": ""},
			map[string]string{},
			[]string{},
		},
		{
			"results are sorted",
			map[string]string{"Z": "1", "A": "2", "M": "3"},
			map[string]string{"A": "2"},
			[]string{"M", "Z"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := setenvDelta(tc.view, tc.env); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("setenvDelta(%v, %v) = %v, want %v", tc.view, tc.env, got, tc.want)
			}
		})
	}
}
