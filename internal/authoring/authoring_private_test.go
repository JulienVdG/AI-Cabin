package authoring

import (
	"bytes"
	"errors"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestMergeMapping covers the mergeMapping merge cases: the default branch
// (different-kind values are replaced wholesale) and the nil-source guard.
func TestMergeMapping(t *testing.T) {
	// DefaultOverride: a key whose existing and incoming values are different
	// kinds (a scalar overridden by a sequence here) is replaced wholesale rather
	// than recursed or appended.
	t.Run("DefaultOverride", func(t *testing.T) {
		dst := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		dst.Content = append(dst.Content, strNode("flags"), strNode("x"))
		src := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		src.Content = append(src.Content, strNode("flags"), seqNode([]string{"a"}))

		got := mergeMapping(dst, src)
		if got.Content[1].Kind != yaml.SequenceNode {
			t.Fatalf("flags kind = %v, want seq (scalar overridden by sequence)", got.Content[1].Kind)
		}
		if got.Content[1].Content[0].Value != "a" {
			t.Errorf("flags[0] = %q, want a", got.Content[1].Content[0].Value)
		}
	})

	// NilSrc: merging a nil source subtree returns the destination unchanged.
	t.Run("NilSrc", func(t *testing.T) {
		dst := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		dst.Content = append(dst.Content, strNode("k"), strNode("v"))
		if got := mergeMapping(dst, nil); got != dst {
			t.Errorf("mergeMapping(dst, nil) changed the destination")
		}
	})

	// BothMaps: a key that is a mapping in both sides is merged recursively
	// (the two map subtrees are combined, not replaced).
	t.Run("BothMaps", func(t *testing.T) {
		dst := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		dst.Content = append(dst.Content, strNode("svc"), mapStr("a", "1"))
		src := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		src.Content = append(src.Content, strNode("svc"), mapStr("b", "2"))

		got := mergeMapping(dst, src)
		svc := got.Content[1]
		if len(svc.Content) != 4 {
			t.Fatalf("svc keys = %d, want 2 merged (a and b)", len(svc.Content)/2)
		}
	})

	// BothSeqs: a key that is a sequence in both sides is appended.
	t.Run("BothSeqs", func(t *testing.T) {
		dst := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		dst.Content = append(dst.Content, strNode("ports"), seqNode([]string{"p1"}))
		src := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		src.Content = append(src.Content, strNode("ports"), seqNode([]string{"p2"}))

		got := mergeMapping(dst, src)
		ports := got.Content[1]
		if len(ports.Content) != 2 {
			t.Fatalf("ports items = %d, want 2 appended", len(ports.Content))
		}
		if ports.Content[0].Value != "p1" || ports.Content[1].Value != "p2" {
			t.Errorf("ports = %q,%q want p1,p2", ports.Content[0].Value, ports.Content[1].Value)
		}
	})

	// NilDst: merging into a nil destination returns a clone of the source.
	t.Run("NilDst", func(t *testing.T) {
		src := mapStr("k", "v")
		got := mergeMapping(nil, src)
		if got.Content[0].Value != "k" || got.Content[1].Value != "v" {
			t.Errorf("got = %v, want a clone of src (k=v)", got.Content)
		}
	})
}

// TestCloneNode covers cloneNode: the nil guard and a real deep copy that
// preserves fields (comments, style) and does not alias the source children.
func TestCloneNode(t *testing.T) {
	t.Run("Nil", func(t *testing.T) {
		if got := cloneNode(nil); got != nil {
			t.Errorf("cloneNode(nil) = %v, want nil", got)
		}
	})

	t.Run("DeepCopy", func(t *testing.T) {
		src := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", HeadComment: "h", FootComment: "f"}
		child := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "v", LineComment: "lc", Style: yaml.DoubleQuotedStyle}
		src.Content = append(src.Content, child)

		got := cloneNode(src)
		if got == src {
			t.Fatal("cloneNode returned the source node, want a copy")
		}
		if got.HeadComment != "h" || got.FootComment != "f" {
			t.Errorf("node comments lost: head=%q foot=%q", got.HeadComment, got.FootComment)
		}
		if got.Content[0] == child {
			t.Error("child node aliased, want a deep copy")
		}
		c := got.Content[0]
		if c.Value != "v" || c.LineComment != "lc" || c.Style != yaml.DoubleQuotedStyle {
			t.Errorf("child fields not preserved: value=%q comment=%q style=%v", c.Value, c.LineComment, c.Style)
		}
		// The copy is independent: mutating it leaves the source untouched.
		c.Value = "changed"
		if child.Value != "v" {
			t.Error("mutating the clone changed the source child")
		}
	})
}

// mapStr builds a mapping node from alternating scalar key/value strings.
func mapStr(pairs ...string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for i := 0; i+1 < len(pairs); i += 2 {
		n.Content = append(n.Content, strNode(pairs[i]), strNode(pairs[i+1]))
	}
	return n
}

// failWriter is a writer that fails every write, to exercise errWriter's error
// path.
type failWriter struct{}

func (failWriter) Write(p []byte) (int, error) { return 0, errors.New("boom") }

// TestErrWriter covers the errWriter contract: a successful run buffers every
// part and stays error-free; a failing writer records the first error and
// drops every later write.
func TestErrWriter(t *testing.T) {
	t.Run("propagates first error and drops later writes", func(t *testing.T) {
		ew := &errWriter{w: failWriter{}}
		ew.Write([]byte("a"))
		ew.Write([]byte("b")) // dropped: the first error is cached
		ew.printf("c")        // dropped too
		if ew.err == nil {
			t.Fatal("expected the first write error to be recorded")
		}
	})

	t.Run("passes through successful writes", func(t *testing.T) {
		var buf bytes.Buffer
		ew := &errWriter{w: &buf}
		ew.printf("hello %s", "world")
		if ew.err != nil {
			t.Fatalf("unexpected error: %v", ew.err)
		}
		if buf.String() != "hello world" {
			t.Errorf("got %q, want hello world", buf.String())
		}
	})
}
