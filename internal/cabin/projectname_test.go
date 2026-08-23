package cabin_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/cabin"
)

// TestComposeProjectName covers the project name derivation: profile and
// canonical are joined with a separator, both sanitized to the compose charset,
// and a missing profile yields the canonical name alone (the convergence
// fallback).
func TestComposeProjectName(t *testing.T) {
	cases := []struct {
		name      string
		profile   string
		canonical string
		want      string
	}{
		{name: "no profile canonical only", profile: "", canonical: "pi-go", want: "pi-go"},
		{name: "profile and cabin", profile: "1", canonical: "pi-go", want: "1_pi-go"},
		{name: "two profiles distinct", profile: "2", canonical: "pi-go", want: "2_pi-go"},
		{name: "named profile", profile: "default", canonical: "after", want: "default_after"},
		{name: "lowercases both parts", profile: "My-Prof", canonical: "Pi.Go", want: "my-prof_pi-go"},
		{name: "lowercase no profile", profile: "", canonical: "PI-GO", want: "pi-go"},
		{name: "trim leading trailing dash", profile: "", canonical: "-pi-", want: "pi"},
		{name: "trim leading trailing underscore", profile: "", canonical: "_pi_", want: "pi"},
		{name: "spaces to dash", profile: "", canonical: "My Cabin 2", want: "my-cabin-2"},
		{name: "profile with spaces", profile: "1", canonical: "My Cabin 2", want: "1_my-cabin-2"},
		{name: "dot to dash", profile: "", canonical: "pi.go", want: "pi-go"},
		{name: "fallback cabin on all invalid canonical", profile: "", canonical: "...", want: "cabin"},
		{name: "fallback on empty canonical", profile: "", canonical: "", want: "cabin"},
		{name: "profile all invalid falls back, canonical kept", profile: "...", canonical: "pi-go", want: "cabin_pi-go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cabin.ComposeProjectName(tc.profile, tc.canonical); got != tc.want {
				t.Errorf("ComposeProjectName(%q, %q) = %q, want %q",
					tc.profile, tc.canonical, got, tc.want)
			}
		})
	}
}

// TestCanonicalName covers the name derivation from a cabin path: the
// ai-cabin.cabin header field wins, the directory basename is the fallback, and
// an invalid cabin (no Taskfile, no header) errors so the caller can skip the
// injection instead of producing a blank name.
func TestCanonicalName(t *testing.T) {
	t.Run("header cabin field used", func(t *testing.T) {
		dir := t.TempDir()
		writeTaskfile(t, dir, validTaskfile) // cabin: blog
		name, err := cabin.CanonicalName(dir)
		if err != nil {
			t.Fatalf("CanonicalName error = %v", err)
		}
		if name != "blog" {
			t.Errorf("CanonicalName = %q, want %q (header cabin field)", name, "blog")
		}
	})

	t.Run("basename fallback when no cabin field", func(t *testing.T) {
		dir := t.TempDir()
		writeTaskfile(t, dir, `ai-cabin:
  agents: [pi]
`)
		name, err := cabin.CanonicalName(dir)
		if err != nil {
			t.Fatalf("CanonicalName error = %v", err)
		}
		if want := filepath.Base(dir); name != want {
			t.Errorf("CanonicalName = %q, want %q (dir basename)", name, want)
		}
	})

	t.Run("missing taskfile errors", func(t *testing.T) {
		dir := t.TempDir() // no Taskfile
		_, err := cabin.CanonicalName(dir)
		if err == nil {
			t.Fatal("CanonicalName error = nil, want error")
		}
		if !strings.Contains(err.Error(), cabin.TaskfileName) {
			t.Errorf("error = %v, want it to mention %q", err, cabin.TaskfileName)
		}
	})

	t.Run("missing ai-cabin block returns ErrNoHeader", func(t *testing.T) {
		dir := t.TempDir()
		writeTaskfile(t, dir, `version: "3"
tasks:
  pi:
    cmds: ["echo hi"]
`)
		_, err := cabin.CanonicalName(dir)
		if err == nil {
			t.Fatal("CanonicalName error = nil, want ErrNoHeader")
		}
		if !errors.Is(err, cabin.ErrNoHeader) {
			t.Errorf("error = %v, want errors.Is(err, ErrNoHeader)", err)
		}
	})
}

// TestDeriveProfile covers the reverse of ComposeProjectName: extracting the
// profile from a compose project label. The canonical-only form yields ""
// (no profile), the <profile>_<canonical> form yields the profile, and a
// manually-set project that does not match the expected shape yields "" rather
// than a misleading guess.
func TestDeriveProfile(t *testing.T) {
	cases := []struct {
		name      string
		project   string
		canonical string
		want      string
	}{
		{name: "profile stripped", project: "default_pi-go", canonical: "pi-go", want: "default"},
		{name: "two-digit profile", project: "12_pi-go", canonical: "pi-go", want: "12"},
		{name: "canonical-only no profile", project: "pi-go", canonical: "pi-go", want: ""},
		{name: "uppercase canonical sanitized", project: "default_pi-go", canonical: "PI-GO", want: "default"},
		{name: "dotted canonical sanitized", project: "default_my-blog", canonical: "my.blog", want: "default"},
		{name: "profile with dash kept", project: "my-prof_pi-go", canonical: "pi-go", want: "my-prof"},
		{name: "manual project not inferred", project: "custom", canonical: "pi-go", want: ""},
		{name: "manual project with underscore not inferred", project: "a_b_c", canonical: "pi-go", want: ""},
		{name: "profile sanitized too", project: "my-prof_my-blog", canonical: "my.blog", want: "my-prof"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cabin.DeriveProfile(tc.project, tc.canonical); got != tc.want {
				t.Errorf("DeriveProfile(%q, %q) = %q, want %q",
					tc.project, tc.canonical, got, tc.want)
			}
		})
	}
}
