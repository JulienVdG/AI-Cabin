package embedded_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/JulienVdG/AI-Cabin/internal/render"
)

// TestAgentOpencodeBlueprintWebModes renders the embedded agent-opencode
// blueprint with the full range of `web` attr values and asserts the
// resulting assembly shape: the web on-modes (cmd/true, case-insensitive,
// YAML bool or string alias) contribute the web CMD, the EXPOSE and the
// compose ports/password env, and GREYWALL_ARGS=-p 9090; the off-modes
// (absent attr, explicit no/false, or the reserved svc value which is not
// implemented yet) leave the cabin exec-only (TUI via task opencode).
func TestAgentOpencodeBlueprintWebModes(t *testing.T) {
	raw, err := os.ReadFile("root/fragments/agent-opencode/blueprint.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		attrs   map[string]any
		wantCMD bool
		wantPwd bool
	}{
		{"Absent", nil, false, false},
		{"Cmd", map[string]any{"web": "cmd"}, true, true},
		{"CmdUpper", map[string]any{"web": "CMD"}, true, true},
		{"True", map[string]any{"web": true}, true, true},
		{"TrueString", map[string]any{"web": "true"}, true, true},
		{"No", map[string]any{"web": "no"}, false, false},
		{"Svc", map[string]any{"web": "svc"}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := render.RenderString(string(raw), nil, tc.attrs, render.Delims{Left: "{<", Right: ">}"})
			if err != nil {
				t.Fatalf("render: %v\noutput:\n%s", err, out)
			}
			var bp map[string]any
			if err := yaml.Unmarshal([]byte(out), &bp); err != nil {
				t.Fatalf("yaml parse: %v\nrendered:\n%s", err, out)
			}
			dockerfile, _ := bp["dockerfile"].(string)
			if strings.Contains(dockerfile, "CMD [") != tc.wantCMD {
				t.Errorf("CMD presence = %v, want %v\ndockerfile:\n%s", !tc.wantCMD, tc.wantCMD, dockerfile)
			}
			if strings.Contains(dockerfile, "EXPOSE 9090") != tc.wantCMD {
				t.Errorf("EXPOSE presence = %v, want %v\ndockerfile:\n%s", !tc.wantCMD, tc.wantCMD, dockerfile)
			}
			compose := bp["compose"].(map[string]any)
			_, hasPwd := compose["environment"]
			_, hasPorts := compose["ports"]
			if hasPwd != tc.wantPwd || hasPorts != tc.wantPwd {
				t.Errorf("compose env/ports = %v/%v, want %v", hasPwd, hasPorts, tc.wantPwd)
			}
			tf := bp["taskfile"].(map[string]any)
			env, hasEnv := tf["env"].(map[string]any)
			if hasEnv != tc.wantPwd {
				t.Errorf("taskfile env = %v, want %v", hasEnv, tc.wantPwd)
			}
			if hasEnv {
				if env["GREYWALL_ARGS"] != "-p 9090" {
					t.Errorf("GREYWALL_ARGS = %v", env["GREYWALL_ARGS"])
				}
			}
			tasks := tf["tasks"].(map[string]any)
			info := tasks["info"].(map[string]any)
			infoOut := info["cmds"].([]any)[0].(string)
			if tc.wantPwd && !strings.Contains(infoOut, "localhost:9090") {
				t.Errorf("info (web): %q", infoOut)
			}
			if !tc.wantPwd && !strings.Contains(infoOut, "task opencode") {
				t.Errorf("info (tui): %q", infoOut)
			}
		})
	}
}
