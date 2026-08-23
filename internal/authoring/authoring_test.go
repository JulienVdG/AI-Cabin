package authoring_test

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/JulienVdG/AI-Cabin/internal/authoring"
	"github.com/JulienVdG/AI-Cabin/internal/cabin"
	"github.com/JulienVdG/AI-Cabin/internal/fragments"
)

// bundleFS builds a fallback FS holding one blueprint.yaml per bundle name.
func bundleFS(bodies map[string]string) *fstest.MapFS {
	m := &fstest.MapFS{}
	for name, body := range bodies {
		(*m)[name+"/blueprint.yaml"] = &fstest.MapFile{Data: []byte(body)}
	}
	return m
}

// refs builds the feature refs for a base + named bundles, base first.
func refs(names ...string) []cabin.FeatureRef {
	out := []cabin.FeatureRef{{Name: cabin.BaseBundle}}
	for _, n := range names {
		if n == cabin.BaseBundle {
			continue
		}
		out = append(out, cabin.FeatureRef{Name: n})
	}
	return out
}

const baseBP = `
apt: [git, curl]
dockerfile: |
  COPY .deps/ /opt/ai-cabin-deps/
  ENTRYPOINT ["/docker-entrypoint.sh"]
compose:
  stdin_open: true
  privileged: true
  environment:
    - TERM=xterm-256color
  volumes:
    - desk:/desk:rw
taskfile:
  version: '3'
  env:
    CONTAINER_HOME: /home/ai_agent
  tasks:
    setup:
      desc: agent setup
      preconditions:
        - sh: '[ -v AI_CABIN_HOME ]'
          msg: "missing AI_CABIN_HOME"
      cmds:
        - touch {{.AI_CABIN_HOME}}/.local/state/ai-cabin/container_bash_history
    info:
      cmds:
        - 'echo "greyproxy (host): http://localhost:43080"'
`

const piBP = `
args: |
  ARG PI_VERSION=v0.72.3
dockerfile: |
  # Agent pi
  RUN mkdir -p .pi/agent
compose:
  environment:
    - SCW_PROJECT_ID=${SCW_PROJECT_ID}
  volumes:
    - ${AI_CABIN_HOME}/.pi:${CONTAINER_HOME}/.pi:rw
taskfile:
  tasks:
    setup:
      cmds:
        - mkdir -p {{.AI_CABIN_HOME}}/.pi/agent
    pi:
      desc: Continue pi.dev session
      cmds:
        - docker compose exec -e CABIN_REL_PATH="${CABIN_REL_PATH}" agent pi {{.CLI_ARGS}}
    info:
      cmds:
        - 'echo "AI Agent (pi.dev TUI): Use ''task pi'''
`

// TestAssemble groups the Assemble writer cases under one function with a
// sub-test per artifact and per edge case.
func TestAssemble(t *testing.T) {
	t.Run("DockerfileMerge", func(t *testing.T) {
		fs := bundleFS(map[string]string{cabin.BaseBundle: baseBP, "agent-pi": piBP})
		bps := fragments.ResolveBlueprints(fs, refs("agent-pi"))
		sel := authoring.Selection{Name: "x", Agents: []string{"pi"}}
		var out strings.Builder
		if err := authoring.Assemble(bps, sel, &authoring.Files{Dockerfile: &out}); err != nil {
			t.Fatalf("assemble: %v", err)
		}
		df := out.String()

		for _, want := range []string{
			"FROM " + authoring.DefaultBaseImage,
			"ARG PI_VERSION=v0.72.3", // pi args appended after base (base has none here)
			"RUN apt-get update && apt-get install -y \\",
			"    git \\",
			"    && rm -rf /var/lib/apt/lists/*",
			"COPY .deps/ /opt/ai-cabin-deps/",
			"# Agent pi",
			"RUN mkdir -p .pi/agent",
			`CMD ["sleep", "infinity"]`, // pi declares no CMD: default appended
		} {
			if !strings.Contains(df, want) {
				t.Errorf("Dockerfile missing %q\n---\n%s", want, df)
			}
		}
		if strings.Count(df, "RUN apt-get") != 1 {
			t.Errorf("expected exactly one apt-get RUN")
		}
	})

	t.Run("ComposeMerge", func(t *testing.T) {
		opencodeBP := `
compose:
  environment:
    - OPENCODE_SERVER_PASSWORD=${OPENCODE_SERVER_PASSWORD}
  volumes:
    - ${AI_CABIN_HOME}/.config/opencode:${CONTAINER_HOME}/.config/opencode:rw
  ports:
    - "127.0.0.1:9090:9090"
`
		fs := bundleFS(map[string]string{cabin.BaseBundle: baseBP, "agent-opencode": opencodeBP})
		bps := fragments.ResolveBlueprints(fs, refs("agent-opencode"))
		sel := authoring.Selection{Name: "mycabin", Agents: []string{"opencode"}}
		var cf strings.Builder
		if err := authoring.Assemble(bps, sel, &authoring.Files{Compose: &cf}); err != nil {
			t.Fatalf("assemble: %v", err)
		}
		got := cf.String()

		for _, want := range []string{
			"services:",
			"  agent:",
			"    build:",
			"      context: .",
			"      dockerfile: ai-cabin.Dockerfile",
			"    image: mycabin",
			"    hostname: mycabin",
			"    stdin_open: true",
			"    privileged: true",
			"    environment:",
			"      - TERM=xterm-256color",
			"      - OPENCODE_SERVER_PASSWORD=${OPENCODE_SERVER_PASSWORD}",
			"    volumes:",
			"      - desk:/desk:rw",
			"      - ${AI_CABIN_HOME}/.config/opencode:${CONTAINER_HOME}/.config/opencode:rw",
			"    ports:",
			`      - "127.0.0.1:9090:9090"`,
		} {
			if !strings.Contains(got, want) {
				t.Errorf("Compose missing %q\n---\n%s", want, got)
			}
		}
		// Section membership: the opencode port and volume land in the right sections.
		envBlock, _, _ := strings.Cut(strings.SplitN(got, "    environment:\n", 2)[1], "\n    volumes:")
		if strings.Contains(envBlock, "9090:9090") || strings.Contains(envBlock, "opencode") {
			t.Errorf("agent port/volume leaked under environment: %q", envBlock)
		}
	})

	t.Run("TaskfileMerge", func(t *testing.T) {
		fs := bundleFS(map[string]string{cabin.BaseBundle: baseBP, "agent-pi": piBP})
		bps := fragments.ResolveBlueprints(fs, refs("agent-pi"))
		sel := authoring.Selection{Name: "x", Agents: []string{"pi"}}
		var tf strings.Builder
		if err := authoring.Assemble(bps, sel, &authoring.Files{Taskfile: &tf}); err != nil {
			t.Fatalf("assemble: %v", err)
		}
		tfStr := tf.String()

		for _, want := range []string{
			"ai-cabin:",
			"  agents:",
			"    - pi",
			"version: '3'",
			"  CONTAINER_HOME: /home/ai_agent",
			"      - touch {{.AI_CABIN_HOME}}/.local/state/ai-cabin/container_bash_history",
			"      - mkdir -p {{.AI_CABIN_HOME}}/.pi/agent", // pi setup appended to base setup cmds
			"  pi:",
			"    desc: Continue pi.dev session",
			"agent pi {{.CLI_ARGS}}", // run cmd verbatim ({{.CLI_ARGS}} preserved)
			"greyproxy (host): http://localhost:43080",
			"AI Agent (pi.dev TUI)",
		} {
			if !strings.Contains(tfStr, want) {
				t.Errorf("Taskfile missing %q\n---\n%s", want, tfStr)
			}
		}
		if strings.Contains(tfStr, "<no value>") {
			t.Errorf("task-time vars must stay literal, got <no value>")
		}
	})

	// DeclaredCMD covers the writeDockerfile hasCMD branch: when a body line
	// already declares a CMD, the default sleep-infinity command is not appended.
	t.Run("DeclaredCMD", func(t *testing.T) {
		webBP := `
dockerfile: |
  RUN echo starting
  CMD ["npm", "start"]
`
		fs := bundleFS(map[string]string{cabin.BaseBundle: baseBP, "web": webBP})
		bps := fragments.ResolveBlueprints(fs, refs("web"))
		sel := authoring.Selection{Name: "x", Agents: []string{"web"}}
		var out strings.Builder
		if err := authoring.Assemble(bps, sel, &authoring.Files{Dockerfile: &out}); err != nil {
			t.Fatalf("assemble: %v", err)
		}
		df := out.String()
		if !strings.Contains(df, `CMD ["npm", "start"]`) {
			t.Errorf("authored CMD missing\n---\n%s", df)
		}
		if strings.Contains(df, `CMD ["sleep", "infinity"]`) {
			t.Errorf("default sleep CMD must not be appended when a body CMD is declared\n---\n%s", df)
		}
	})

	// TaskfileFeatures covers the writeTaskfile features branch: a selection
	// with features emits a features list in the ai-cabin header.
	t.Run("TaskfileFeatures", func(t *testing.T) {
		fs := bundleFS(map[string]string{cabin.BaseBundle: baseBP})
		bps := fragments.ResolveBlueprints(fs, refs())
		sel := authoring.Selection{Name: "x", Agents: []string{"pi"}, Features: []string{"go"}}
		var tf strings.Builder
		if err := authoring.Assemble(bps, sel, &authoring.Files{Taskfile: &tf}); err != nil {
			t.Fatalf("assemble: %v", err)
		}
		out := tf.String()
		for _, want := range []string{
			"  features:",
			"    - go",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("Taskfile header missing %q\n---\n%s", want, out)
			}
		}
	})

	// PropagatesWriteError covers the Assemble error branch: a failing Dockerfile
	// writer surfaces as a non-nil error.
	t.Run("PropagatesWriteError", func(t *testing.T) {
		fs := bundleFS(map[string]string{cabin.BaseBundle: baseBP})
		bps := fragments.ResolveBlueprints(fs, refs())
		sel := authoring.Selection{Name: "x"}
		if err := authoring.Assemble(bps, sel, &authoring.Files{Dockerfile: failWriter{}}); err == nil {
			t.Fatal("expected a write error to propagate from Assemble")
		}
	})

	// AptDedup covers the writeApt de-duplication: a package contributed by
	// several bundles (or repeated within one) appears once in the RUN.
	t.Run("AptDedup", func(t *testing.T) {
		dupBP := `
apt: [git, dupe, dupe]
dockerfile: |
  RUN echo dup
`
		fs := bundleFS(map[string]string{cabin.BaseBundle: baseBP, "dup": dupBP})
		bps := fragments.ResolveBlueprints(fs, refs("dup"))
		sel := authoring.Selection{Name: "x"}
		var out strings.Builder
		if err := authoring.Assemble(bps, sel, &authoring.Files{Dockerfile: &out}); err != nil {
			t.Fatalf("assemble: %v", err)
		}
		df := out.String()
		for _, pkg := range []string{"git", "curl", "dupe"} {
			line := "    " + pkg + " \\"
			if n := strings.Count(df, line); n != 1 {
				t.Errorf("apt package %q appears %d time(s), want 1\n---\n%s", pkg, n, df)
			}
		}
	})

	// AptEmpty covers the writeApt early return: with no bundle contributing
	// packages, no apt-get RUN is emitted.
	t.Run("AptEmpty", func(t *testing.T) {
		noApt := `
dockerfile: |
  RUN echo no apt
`
		fs := bundleFS(map[string]string{"x": noApt})
		bps := fragments.ResolveBlueprints(fs, []cabin.FeatureRef{{Name: "x"}})
		sel := authoring.Selection{Name: "x"}
		var out strings.Builder
		if err := authoring.Assemble(bps, sel, &authoring.Files{Dockerfile: &out}); err != nil {
			t.Fatalf("assemble: %v", err)
		}
		if strings.Contains(out.String(), "apt-get") {
			t.Errorf("no apt RUN expected without packages\n---\n%s", out.String())
		}
	})
}

// failWriter is an io.Writer that fails every write, to exercise Assemble's
// error propagation.
var _ io.Writer = failWriter{}

type failWriter struct{}

func (failWriter) Write(p []byte) (int, error) { return 0, errors.New("boom") }
