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
