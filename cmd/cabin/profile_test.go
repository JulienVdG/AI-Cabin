package main

import (
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/config"
)

// TestProfileLine covers the list marker rendering: a plain entry, the active
// profile marked current, the source annotation when the active profile
// overrides the use-selected one (--profile / AI_CABIN_PROFILE), and the
// (overridden) flag on the use-selected profile that is not in effect.
func TestProfileLine(t *testing.T) {
	configSelection := config.ProfileSelection{Name: "work", Source: config.ProfileSourceConfig, Use: "work"}
	envSelection := config.ProfileSelection{Name: "envprof", Source: config.ProfileSourceEnv, Use: "work"}
	flagSelection := config.ProfileSelection{Name: "flagprof", Source: config.ProfileSourceFlag, Use: "work"}
	argSelection := config.ProfileSelection{Name: "perso", Source: config.ProfileSourceArg, Use: "work"}
	emptySelection := config.ProfileSelection{}

	// For each selection, list a few profile names and assert the rendered line.
	cases := []struct {
		name    string
		profile string
		sel     config.ProfileSelection
		want    string
	}{
		{"inactive plain entry", "envprof", configSelection, "  envprof"},
		{"active from config", "work", configSelection, "* work (current)"},
		{"active from env overriding", "envprof", envSelection, "* envprof (current, from AI_CABIN_PROFILE)"},
		{"active from flag overriding", "flagprof", flagSelection, "* flagprof (current, from --profile)"},
		{"overridden use-selected profile (env)", "work", envSelection, "  work (overridden)"},
		{"overridden use-selected profile (flag)", "work", flagSelection, "  work (overridden)"},
		{"other inactive entry unaffected", "perso", envSelection, "  perso"},
		{"explicit positional active (no origin)", "perso", argSelection, "* perso (current)"},
		{"positional does not flag use as overridden", "work", argSelection, "  work"},
		{"empty selection marks nothing", "work", emptySelection, "  work"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := profileLine(tc.profile, tc.sel); got != tc.want {
				t.Errorf("profileLine(%q, %+v) = %q, want %q", tc.profile, tc.sel, got, tc.want)
			}
		})
	}
}

// TestParseProfileSetArgs covers the positional parsing of `profile set`: the
// copy-paste KEY=VALUE spellings (single and mass), the backwards-compatible
// KEY VALUE pair, the empty case (everything from --var), and the ambiguous or
// malformed shapes that must be rejected.
func TestParseProfileSetArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    config.Vars
		wantErr bool
	}{
		{"empty args", nil, config.Vars{}, false},
		{"single KEY=VALUE", []string{"A=1"}, config.Vars{"A": "1"}, false},
		{"empty value KEY=", []string{"A="}, config.Vars{"A": ""}, false},
		{"mass KEY=VALUE", []string{"A=1", "B=2", "C=3"}, config.Vars{"A": "1", "B": "2", "C": "3"}, false},
		{"backwards-compatible pair", []string{"KEY", "value1"}, config.Vars{"KEY": "value1"}, false},
		{"pair with value containing equal", []string{"KEY", "a=b"}, config.Vars{"KEY": "a=b"}, false},
		{"single bare arg rejected", []string{"A"}, config.Vars{}, true},
		{"KV then bare rejected", []string{"A=1", "bare"}, config.Vars{}, true},
		{"bare pair plus extra rejected", []string{"A", "B", "C"}, config.Vars{}, true},
		{"more than two bare rejected", []string{"A=1", "B=2", "C"}, config.Vars{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseProfileSetArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseProfileSetArgs(%q) expected error, got %v", tc.args, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseProfileSetArgs(%q) error = %v", tc.args, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseProfileSetArgs(%q) = %v, want %v", tc.args, got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("parseProfileSetArgs(%q)[%q] = %q, want %q", tc.args, k, got[k], v)
				}
			}
		})
	}
}
