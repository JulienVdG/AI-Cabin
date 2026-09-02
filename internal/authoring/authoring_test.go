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
		bps := fragments.ResolveBlueprints(fs, refs("agent-pi"), cabin.AICabinHeader{})
		h := cabin.AICabinHeader{Cabin: "x", Agents: []string{"pi"}, AuthoredWith: cabin.AuthoringParams{
			Image: authoring.DefaultBaseImage, User: authoring.DefaultUser, Home: authoring.DefaultHome,
		}}
		var out strings.Builder
		if err := authoring.Assemble(bps, h, &authoring.Files{Dockerfile: &out}); err != nil {
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
		bps := fragments.ResolveBlueprints(fs, refs("agent-opencode"), cabin.AICabinHeader{})
		h := cabin.AICabinHeader{Cabin: "mycabin", Agents: []string{"opencode"}}
		var cf strings.Builder
		if err := authoring.Assemble(bps, h, &authoring.Files{Compose: &cf}); err != nil {
			t.Fatalf("assemble: %v", err)
		}
		got := cf.String()

		for _, want := range []string{
			"services:",
			"  agent:",
			"    build:",
			"      context: .",
			"      dockerfile: ai-cabin.Dockerfile",
			"      args:",
			"        - HOST_UID=${HOST_UID:-1000}",
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
		bps := fragments.ResolveBlueprints(fs, refs("agent-pi"), cabin.AICabinHeader{})
		h := cabin.AICabinHeader{Cabin: "x", Agents: []string{"pi"}}
		var tf strings.Builder
		if err := authoring.Assemble(bps, h, &authoring.Files{Taskfile: &tf}); err != nil {
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
		bps := fragments.ResolveBlueprints(fs, refs("web"), cabin.AICabinHeader{})
		h := cabin.AICabinHeader{Cabin: "x", Agents: []string{"web"}}
		var out strings.Builder
		if err := authoring.Assemble(bps, h, &authoring.Files{Dockerfile: &out}); err != nil {
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
		bps := fragments.ResolveBlueprints(fs, refs(), cabin.AICabinHeader{})
		h := cabin.AICabinHeader{Cabin: "x", Agents: []string{"pi"}, Features: []cabin.FeatureRef{{Name: "go"}}}
		var tf strings.Builder
		if err := authoring.Assemble(bps, h, &authoring.Files{Taskfile: &tf}); err != nil {
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

	// TaskfileAuthoredWith covers the authored_with record: the resolved params
	// are written into the ai-cabin: header so re-running authoring on the
	// cabin reuses the same image/user/home.
	t.Run("TaskfileAuthoredWith", func(t *testing.T) {
		fs := bundleFS(map[string]string{cabin.BaseBundle: baseBP})
		bps := fragments.ResolveBlueprints(fs, refs(), cabin.AICabinHeader{})
		h := cabin.AICabinHeader{Cabin: "x", Agents: []string{"opencode"}, AuthoredWith: cabin.AuthoringParams{
			Image: "ubuntu:24.04", User: "ubuntu", Home: "/home/ubuntu",
		}}
		var tf strings.Builder
		if err := authoring.Assemble(bps, h, &authoring.Files{Taskfile: &tf}); err != nil {
			t.Fatalf("assemble: %v", err)
		}
		out := tf.String()
		for _, want := range []string{
			"  authored_with:",
			"    image: ubuntu:24.04",
			"    user: ubuntu",
			"    home: /home/ubuntu",
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
		bps := fragments.ResolveBlueprints(fs, refs(), cabin.AICabinHeader{})
		h := cabin.AICabinHeader{Cabin: "x"}
		if err := authoring.Assemble(bps, h, &authoring.Files{Dockerfile: failWriter{}}); err == nil {
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
		bps := fragments.ResolveBlueprints(fs, refs("dup"), cabin.AICabinHeader{})
		h := cabin.AICabinHeader{Cabin: "x"}
		var out strings.Builder
		if err := authoring.Assemble(bps, h, &authoring.Files{Dockerfile: &out}); err != nil {
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
		bps := fragments.ResolveBlueprints(fs, []cabin.FeatureRef{{Name: "x"}}, cabin.AICabinHeader{})
		h := cabin.AICabinHeader{Cabin: "x"}
		var out strings.Builder
		if err := authoring.Assemble(bps, h, &authoring.Files{Dockerfile: &out}); err != nil {
			t.Fatalf("assemble: %v", err)
		}
		if strings.Contains(out.String(), "apt-get") {
			t.Errorf("no apt RUN expected without packages\n---\n%s", out.String())
		}
	})

	// AuthoringParams covers the authoring substitutions: image/user/home are
	// rendered into the blueprint (at parse time, before YAML unmarshal, via
	// {<.Image>}/{<.User>}/{<.Home>} actions) and so appear in the assembled
	// dockerfile/compose/taskfile scalar values, while task-time {{...}} vars
	// stay literal (the {< >} delimiters cannot collide with them).
	t.Run("AuthoringParams", func(t *testing.T) {
		paramsBP := `
dockerfile: |
  FROM {<.Image>}
  RUN useradd -m {<.User>}
  WORKDIR {<.Home>}
compose:
  environment:
    - CONTAINER_HOME={<.Home>}
    - LITERAL_TASK={{.AI_CABIN_HOME}}
taskfile:
  env:
    CONTAINER_HOME: {<.Home>}
    TASK_VAR: '{{.AI_CABIN_HOME}}'
`
		fs := bundleFS(map[string]string{cabin.BaseBundle: paramsBP})
		h := cabin.AICabinHeader{Cabin: "x", AuthoredWith: cabin.AuthoringParams{Image: "ubuntu:24.04", User: "ubuntu", Home: "/home/ubuntu"}}
		bps := fragments.ResolveBlueprints(fs, refs(), h)
		var df, cf, tf strings.Builder
		if err := authoring.Assemble(bps, h, &authoring.Files{Dockerfile: &df, Compose: &cf, Taskfile: &tf}); err != nil {
			t.Fatalf("assemble: %v", err)
		}
		for _, want := range []string{
			"FROM ubuntu:24.04",
			"RUN useradd -m ubuntu",
			"WORKDIR /home/ubuntu",
		} {
			if !strings.Contains(df.String(), want) {
				t.Errorf("Dockerfile missing %q\n---\n%s", want, df.String())
			}
		}
		if !strings.Contains(cf.String(), "CONTAINER_HOME=/home/ubuntu") {
			t.Errorf("Compose missing rendered CONTAINER_HOME\n---\n%s", cf.String())
		}
		tfStr := tf.String()
		if !strings.Contains(tfStr, "CONTAINER_HOME: /home/ubuntu") || !strings.Contains(tfStr, "{{.AI_CABIN_HOME}}") {
			t.Errorf("Taskfile mismatch\n---\n%s", tfStr)
		}
		if strings.Contains(tfStr, "<no value>") || strings.Contains(cf.String(), "<no value>") {
			t.Errorf("task-time vars must stay literal, got <no value>")
		}
	})

	// AuthoringParamsDefaults covers the fallback: an empty Image/User/Home in
	// the selection renders the package defaults, matching the reference cabins.
	t.Run("AuthoringParamsDefaults", func(t *testing.T) {
		paramsBP := `
dockerfile: |
  FROM {<.Image>}
  RUN useradd -m {<.User>}
  WORKDIR {<.Home>}
`
		fs := bundleFS(map[string]string{cabin.BaseBundle: paramsBP})
		h := cabin.AICabinHeader{Cabin: "x", AuthoredWith: cabin.AuthoringParams{
			Image: authoring.DefaultBaseImage, User: authoring.DefaultUser, Home: authoring.DefaultHome,
		}}
		bps := fragments.ResolveBlueprints(fs, refs(), h)
		var df strings.Builder
		if err := authoring.Assemble(bps, h, &authoring.Files{Dockerfile: &df}); err != nil {
			t.Fatalf("assemble: %v", err)
		}
		for _, want := range []string{
			"FROM " + authoring.DefaultBaseImage,
			"RUN useradd -m " + authoring.DefaultUser,
			"WORKDIR " + authoring.DefaultHome,
		} {
			if !strings.Contains(df.String(), want) {
				t.Errorf("Dockerfile missing default %q\n---\n%s", want, df.String())
			}
		}
	})
}

// failWriter is an io.Writer that fails every write, to exercise Assemble's
// error propagation.
var _ io.Writer = failWriter{}

type failWriter struct{}

func (failWriter) Write(p []byte) (int, error) { return 0, errors.New("boom") }
