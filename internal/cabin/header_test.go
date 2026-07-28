package cabin_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/cabin"
)

// validTaskfile is a Taskfile with a complete ai-cabin header, used as the
// baseline; tests mutate it to exercise one field at a time.
const validTaskfile = `# ai-cabin metadata (read by the CLI at runtime, ignored by task)
ai-cabin:
  cabin: blog
  agents: [opencode, pi]

version: "3"

vars:
  CONTAINER_HOME: "/home/ai_agent"

tasks:
  pi:
    cmds:
      - docker compose exec agent pi {{.CLI_ARGS}}
`

// headerCase is one ParseHeader sub-case.
type headerCase struct {
	name      string
	input     string
	wantErr   bool
	wantNil   bool // whether ParseHeader should return a nil *AICabinHeader
	wantCabin string
}

func TestParseHeader(t *testing.T) {
	cases := []headerCase{
		{
			name:      "complete header",
			input:     validTaskfile,
			wantNil:   false,
			wantCabin: "blog",
		},
		{
			name: "no ai-cabin key",
			// A valid Taskfile but without the header: unmarshals fine,
			// returns nil (caller decides if missing header is an error).
			input: `version: "3"
tasks:
  pi:
    cmds: ["echo hi"]
`,
			wantNil: true,
		},
		{
			name: "empty ai-cabin map",
			// "ai-cabin: {}" (empty map): VALID cabin header (non-nil).
			// The cabin name falls back to the dir basename at validation time.
			// (Bare "ai-cabin:" is YAML null == absent in yaml.v3, not empty.)
			input: `ai-cabin: {}
version: "3"
`,
			wantNil:   false,
			wantCabin: "",
		},
		{
			name: "only cabin field",
			// Minimal populated header: just "cabin:".
			input: `ai-cabin:
  cabin: solo
`,
			wantNil:   false,
			wantCabin: "solo",
		},
		{
			name: "extra sub-fields ignored",
			// Unknown sub-fields under ai-cabin: are ignored (forward compat
			// for future metadata like "template:", "default_profile:").
			input: `ai-cabin:
  cabin: blog
  future-field: whatever
  nested:
    deep: value
`,
			wantNil:   false,
			wantCabin: "blog",
		},
		{
			name: "invalid yaml",
			// Malformed YAML must error (strict contract).
			input: `ai-cabin:
  cabin: blog
  agents: [unclosed
`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := cabin.ParseHeader([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatal("ParseHeader error = nil, want error")
				}
				if !strings.Contains(err.Error(), "parse Taskfile yaml") {
					t.Errorf("error = %v, want it to wrap \"parse Taskfile yaml\"", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseHeader error = %v", err)
			}
			if (h == nil) != tc.wantNil {
				t.Errorf("header nil = %v, want %v", h == nil, tc.wantNil)
			}
			if h != nil && h.Cabin != tc.wantCabin {
				t.Errorf("Cabin = %q, want %q", h.Cabin, tc.wantCabin)
			}
		})
	}
}

func TestParseHeader_CompleteFields(t *testing.T) {
	// Assert the full header (cabin + agents) parses, not just one field.
	h, err := cabin.ParseHeader([]byte(validTaskfile))
	if err != nil {
		t.Fatalf("ParseHeader error = %v", err)
	}
	if h == nil {
		t.Fatal("header = nil, want non-nil for validTaskfile")
	}
	if len(h.Agents) != 2 || h.Agents[0] != "opencode" || h.Agents[1] != "pi" {
		t.Errorf("Agents = %v, want [opencode pi]", h.Agents)
	}
}

// TestParseHeader_Features covers the two accepted YAML forms for a features:
// entry (bare string and single-key mapping) plus the strict-error cases.
func TestParseHeader_Features(t *testing.T) {
	t.Run("bare string feature", func(t *testing.T) {
		h, err := cabin.ParseHeader([]byte(`ai-cabin:
  features:
    - git-agent
`))
		if err != nil {
			t.Fatalf("ParseHeader error = %v", err)
		}
		if h == nil || len(h.Features) != 1 {
			t.Fatalf("Features = %v, want 1 entry", h)
		}
		if h.Features[0].Name != "git-agent" {
			t.Errorf("Name = %q, want %q", h.Features[0].Name, "git-agent")
		}
		if h.Features[0].Attrs != nil {
			t.Errorf("Attrs = %v, want nil", h.Features[0].Attrs)
		}
	})

	t.Run("single-key mapping with attrs", func(t *testing.T) {
		h, err := cabin.ParseHeader([]byte(`ai-cabin:
  features:
    - port-forward: {port: 3306, host: mariadb}
`))
		if err != nil {
			t.Fatalf("ParseHeader error = %v", err)
		}
		if h == nil || len(h.Features) != 1 {
			t.Fatalf("Features = %v, want 1 entry", h)
		}
		if h.Features[0].Name != "port-forward" {
			t.Errorf("Name = %q, want %q", h.Features[0].Name, "port-forward")
		}
		want := map[string]any{"port": 3306, "host": "mariadb"}
		if !reflect.DeepEqual(h.Features[0].Attrs, want) {
			t.Errorf("Attrs = %v, want %v", h.Features[0].Attrs, want)
		}
	})

	t.Run("null-value mapping feature", func(t *testing.T) {
		// "- git-agent:" (null value) is equivalent to "- git-agent".
		h, err := cabin.ParseHeader([]byte("ai-cabin:\n  features:\n    - git-agent:\n"))
		if err != nil {
			t.Fatalf("ParseHeader error = %v", err)
		}
		if h == nil || len(h.Features) != 1 {
			t.Fatalf("Features = %v, want 1 entry", h)
		}
		if h.Features[0].Name != "git-agent" {
			t.Errorf("Name = %q, want %q", h.Features[0].Name, "git-agent")
		}
		if h.Features[0].Attrs != nil {
			t.Errorf("Attrs = %v, want nil", h.Features[0].Attrs)
		}
	})

	t.Run("mixed agents and features", func(t *testing.T) {
		h, err := cabin.ParseHeader([]byte(`ai-cabin:
  agents: [pi]
  features:
    - git-agent
    - port-forward: {port: 3306, host: mariadb}
`))
		if err != nil {
			t.Fatalf("ParseHeader error = %v", err)
		}
		if h == nil {
			t.Fatal("header = nil")
		}
		if len(h.Agents) != 1 || h.Agents[0] != "pi" {
			t.Errorf("Agents = %v, want [pi]", h.Agents)
		}
		if len(h.Features) != 2 {
			t.Fatalf("Features len = %d, want 2", len(h.Features))
		}
		if h.Features[0].Name != "git-agent" {
			t.Errorf("Features[0].Name = %q, want git-agent", h.Features[0].Name)
		}
		if h.Features[1].Name != "port-forward" {
			t.Errorf("Features[1].Name = %q, want port-forward", h.Features[1].Name)
		}
	})

	t.Run("scalar value for mapping feature is rejected", func(t *testing.T) {
		// "- port-forward: 3306" (scalar instead of a mapping): the value is
		// not null and cannot decode into map[string]any, so UnmarshalYAML
		// surfaces a decode error naming the feature (useful UX for the
		// common "forgot the braces" mistake).
		_, err := cabin.ParseHeader([]byte(`ai-cabin:
  features:
    - port-forward: 3306
`))
		if err == nil {
			t.Fatal("ParseHeader error = nil, want error for scalar feature value")
		}
		if !strings.Contains(err.Error(), "port-forward") {
			t.Errorf("error = %v, want it to mention the feature name %q", err, "port-forward")
		}
	})

	t.Run("multi-key mapping is rejected", func(t *testing.T) {
		_, err := cabin.ParseHeader([]byte(`ai-cabin:
  features:
    - port-forward: {port: 3306}
      git-agent:
`))
		if err == nil {
			t.Fatal("ParseHeader error = nil, want error for multi-key feature")
		}
		if !strings.Contains(err.Error(), "single key") {
			t.Errorf("error = %v, want it to mention %q", err, "single key")
		}
	})

	t.Run("sequence feature item is rejected", func(t *testing.T) {
		_, err := cabin.ParseHeader([]byte(`ai-cabin:
  features:
    - [a, b]
`))
		if err == nil {
			t.Fatal("ParseHeader error = nil, want error for sequence feature")
		}
	})
}

// TestActiveBundles covers the resolution from header to the ordered, deduped
// active bundle list (base always first, agents -> agent-<name>, then features).
func TestActiveBundles(t *testing.T) {
	t.Run("nil header returns nil", func(t *testing.T) {
		if got := cabin.ActiveBundles(nil); got != nil {
			t.Errorf("ActiveBundles(nil) = %v, want nil", got)
		}
	})

	t.Run("empty header yields base only", func(t *testing.T) {
		h, err := cabin.ParseHeader([]byte(`ai-cabin: {}`))
		if err != nil {
			t.Fatalf("ParseHeader error = %v", err)
		}
		got := cabin.ActiveBundles(h)
		if len(got) != 1 || got[0].Name != "base" || got[0].Attrs != nil {
			t.Errorf("ActiveBundles = %v, want [{base}]", got)
		}
	})

	t.Run("agents resolved to agent-<name> with base first", func(t *testing.T) {
		h, err := cabin.ParseHeader([]byte(`ai-cabin:
  agents: [pi, opencode]
`))
		if err != nil {
			t.Fatalf("ParseHeader error = %v", err)
		}
		got := cabin.ActiveBundles(h)
		if len(got) != 3 {
			t.Fatalf("ActiveBundles len = %d, want 3", len(got))
		}
		wantNames := []string{"base", "agent-pi", "agent-opencode"}
		for i, w := range wantNames {
			if got[i].Name != w {
				t.Errorf("ActiveBundles[%d].Name = %q, want %q", i, got[i].Name, w)
			}
			if got[i].Attrs != nil {
				t.Errorf("ActiveBundles[%d].Attrs = %v, want nil (agents carry no attrs)", i, got[i].Attrs)
			}
		}
	})

	t.Run("features carry attrs and follow agents", func(t *testing.T) {
		h, err := cabin.ParseHeader([]byte(`ai-cabin:
  agents: [pi]
  features:
    - port-forward: {port: 3306, host: mariadb}
    - git-agent
`))
		if err != nil {
			t.Fatalf("ParseHeader error = %v", err)
		}
		got := cabin.ActiveBundles(h)
		if len(got) != 4 {
			t.Fatalf("ActiveBundles len = %d, want 4", len(got))
		}
		wantNames := []string{"base", "agent-pi", "port-forward", "git-agent"}
		for i, w := range wantNames {
			if got[i].Name != w {
				t.Errorf("ActiveBundles[%d].Name = %q, want %q", i, got[i].Name, w)
			}
		}
		wantAttrs := map[string]any{"port": 3306, "host": "mariadb"}
		if !reflect.DeepEqual(got[2].Attrs, wantAttrs) {
			t.Errorf("port-forward Attrs = %v, want %v", got[2].Attrs, wantAttrs)
		}
		if got[3].Attrs != nil {
			t.Errorf("git-agent Attrs = %v, want nil", got[3].Attrs)
		}
	})

	t.Run("port-forward multi-instance keeps distinct attrs", func(t *testing.T) {
		// port-forward models one instance per forwarded service, so two entries
		// with different attrs are both kept — dedup would silently drop the
		// second forward (a real, breaking bug). Both carry their own attrs.
		h, err := cabin.ParseHeader([]byte(`ai-cabin:
  agents: [pi]
  features:
    - port-forward: {port: 3306, host: mariadb}
    - port-forward: {port: 5432, host: postgres}
`))
		if err != nil {
			t.Fatalf("ParseHeader error = %v", err)
		}
		got := cabin.ActiveBundles(h)
		if len(got) != 4 {
			t.Fatalf("ActiveBundles len = %d, want 4 (base + agent-pi + 2 port-forwards)", len(got))
		}
		wantNames := []string{"base", "agent-pi", "port-forward", "port-forward"}
		for i, w := range wantNames {
			if got[i].Name != w {
				t.Errorf("ActiveBundles[%d].Name = %q, want %q", i, got[i].Name, w)
			}
		}
		wantAttrs1 := map[string]any{"port": 3306, "host": "mariadb"}
		wantAttrs2 := map[string]any{"port": 5432, "host": "postgres"}
		if !reflect.DeepEqual(got[2].Attrs, wantAttrs1) {
			t.Errorf("port-forward[0] Attrs = %v, want %v", got[2].Attrs, wantAttrs1)
		}
		if !reflect.DeepEqual(got[3].Attrs, wantAttrs2) {
			t.Errorf("port-forward[1] Attrs = %v, want %v", got[3].Attrs, wantAttrs2)
		}
	})
}
