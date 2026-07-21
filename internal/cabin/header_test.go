package cabin_test

import (
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
