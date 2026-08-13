package fragments

import (
	"slices"
	"testing"
	"testing/fstest"

	"gopkg.in/yaml.v3"

	"github.com/JulienVdG/AI-Cabin/internal/cabin"
)

// TestResolveBlueprints covers the blueprint facet resolution: a bundle with
// no blueprint.yaml is skipped, a malformed manifest surfaces a per-bundle
// error, verbatim args/dockerfile blocks are parsed into line lists, the
// compose/taskfile subtrees are kept as yaml.Node mappings, and bundle order
// is preserved.
func TestResolveBlueprints(t *testing.T) {
	merged := fstest.MapFS{
		"base/blueprint.yaml": {Data: []byte(`
apt: [git, curl]
args: |
  ARG PI_VERSION=v0.72.1
dockerfile: |
  COPY .deps/ /opt/ai-cabin-deps/
  RUN /bin/sh /opt/ai-cabin-deps/install.sh \
   && rm -rf /opt/ai-cabin-deps
compose:
  environment:
    - TERM=xterm-256color
taskfile:
  version: '3'
`)},
		"go/blueprint.yaml": {Data: []byte(`
compose:
  environment:
    - GOPATH=${CONTAINER_HOME}/go
  volumes:
    - x:y
taskfile:
  tasks:
    go:
      cmds:
        - mkdir -p {{.AI_CABIN_HOME}}/go
`)},
		"port-forward/deps.yaml": {Data: []byte("mirror: deps/\n")},
		"broken/blueprint.yaml":  {Data: []byte("not: [valid: yaml: [[")},
	}

	t.Run("skips bundles with no blueprint and preserves order", func(t *testing.T) {
		bundles := []cabin.FeatureRef{
			{Name: cabin.BaseBundle},
			{Name: "port-forward"}, // has deps.yaml only: no blueprint
		}
		got := ResolveBlueprints(merged, bundles)
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1 (base; port-forward skipped)", len(got))
		}
		if got[0].Name != "base" {
			t.Errorf("name = %q, want base", got[0].Name)
		}
	})

	t.Run("parses verbatim args/dockerfile blocks and yaml subtrees", func(t *testing.T) {
		got := ResolveBlueprints(merged, []cabin.FeatureRef{{Name: "base"}})
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1", len(got))
		}
		bp := got[0]
		if bp.Err != nil {
			t.Fatalf("unexpected error: %v", bp.Err)
		}
		wantArgs := []string{"ARG PI_VERSION=v0.72.1"}
		if len(bp.Args) != 1 || bp.Args[0] != wantArgs[0] {
			t.Errorf("args = %q, want %q", bp.Args, wantArgs)
		}
		if len(bp.Dockerfile) != 3 {
			t.Fatalf("dockerfile lines = %d, want 3 (continuation kept)", len(bp.Dockerfile))
		}
		if bp.Dockerfile[1] != "RUN /bin/sh /opt/ai-cabin-deps/install.sh \\" {
			t.Errorf("dockerfile[1] = %q, want the RUN continuation kept verbatim", bp.Dockerfile[1])
		}
		if bp.Compose == nil || bp.Compose.Kind != yaml.MappingNode {
			t.Errorf("compose should be a mapping node, got %v", bp.Compose)
		}
		if bp.Taskfile == nil || bp.Taskfile.Kind != yaml.MappingNode {
			t.Errorf("taskfile should be a mapping node, got %v", bp.Taskfile)
		}
	})

	t.Run("reports a malformed manifest per bundle without dropping the rest", func(t *testing.T) {
		bundles := []cabin.FeatureRef{{Name: "base"}, {Name: "broken"}}
		got := ResolveBlueprints(merged, bundles)
		if len(got) != 2 {
			t.Fatalf("len(got) = %d, want 2 (base resolved, broken reported)", len(got))
		}
		if got[1].Name != "broken" || got[1].Err == nil {
			t.Errorf("broken bundle: Name=%q Err=%v, want a parse error", got[1].Name, got[1].Err)
		}
		if got[0].Err != nil {
			t.Errorf("base bundle should resolve clean, got Err=%v", got[0].Err)
		}
	})
}

// TestVerbatimList covers the verbatim args/dockerfile line-list reader: a
// missing key returns nil, a scalar block goes through splitLines (outer blank
// lines stripped, inner blanks kept), a sequence node collects the values, and
// any other node kind returns nil.
func TestVerbatimList(t *testing.T) {
	t.Run("missing key returns nil", func(t *testing.T) {
		if got := verbatimList(parseRootNode(t, "other: 1\n"), "args"); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("scalar block strips outer blanks, keeps inner", func(t *testing.T) {
		got := verbatimList(parseRootNode(t, "args: |\n  A\n\n  B\n\n"), "args")
		want := []string{"A", "", "B"}
		if !slices.Equal(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("sequence node collects values", func(t *testing.T) {
		got := verbatimList(parseRootNode(t, "args:\n  - ARG A=1\n  - ARG B=2\n"), "args")
		want := []string{"ARG A=1", "ARG B=2"}
		if !slices.Equal(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("mapping value returns nil", func(t *testing.T) {
		if got := verbatimList(parseRootNode(t, "args:\n  x: 1\n"), "args"); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

// TestScalarAt covers the scalar reader (e.g. help): an absent key returns
// empty, a present scalar returns its value.
func TestScalarAt(t *testing.T) {
	if got := scalarAt(parseRootNode(t, "other: 1\n"), "help"); got != "" {
		t.Errorf("absent key: got %q, want empty", got)
	}
	if got := scalarAt(parseRootNode(t, "help: |\n  line one\n  line two\n"), "help"); got != "line one\nline two\n" {
		t.Errorf("happy path: got %q, want the verbatim content", got)
	}
}

// parseRootNode unmarshals a YAML document and returns its root mapping node
// (the shape blueprint readers take), failing the test on a parse error.
func parseRootNode(t *testing.T, s string) *yaml.Node {
	t.Helper()
	var n yaml.Node
	if err := yaml.Unmarshal([]byte(s), &n); err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return n.Content[0]
}
