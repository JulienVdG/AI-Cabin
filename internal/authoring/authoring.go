// Package authoring assembles a near-complete cabin (Dockerfile + agent-service
// compose + Taskfile) from the active bundles' resolved blueprints. The merge
// is a single format-agnostic operation (append lists, deep-merge YAML maps,
// override scalars) over all blueprints; only the writers differ per target
// format: Dockerfile text (verbatim concatenation), the apt-get RUN, and the
// YAML compose/Taskfile (deep-merged yaml.Node subtrees re-serialized with
// comments preserved).
//
// internal/fragments resolves the per-bundle blueprint content through the
// fallback chain; cmd/cabin wires resolution and write/display. No UX here.
package authoring

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/JulienVdG/AI-Cabin/internal/fragments"
)

// DefaultBaseImage is the FROM the assembled Dockerfile ships: the Debian Go
// image the reference cabins use (greywall's CA step is Debian-specific, hence
// the Debian base assumption). The author adapts it to their project image.
const DefaultBaseImage = "golang:1.26-trixie"

// CabinDockerfile is the filename the assembled Dockerfile is written to by
// `cabin authoring new`. It is distinct from a project's own Dockerfile so a
// non-destructive write never collides with existing project files.
const CabinDockerfile = "ai-cabin.Dockerfile"

// Selection is the feature selection of the cabin being authored: its name
// (used for the compose image/hostname) and the agents/features.
type Selection struct {
	Name     string
	Agents   []string
	Features []string
}

// Files is the write target for Assemble: the destination writer for each
// cabin artifact, chosen by the caller (e.g. a writestrategy.FileCreator for a
// non-destructive write).
type Files struct {
	Dockerfile io.Writer
	Compose    io.Writer
	Taskfile   io.Writer
}

// MergedBlueprint is the single merged structure Merge produces: lists
// appended, YAML subtrees deep-merged, in bundle order (base first). Writers
// read only this, so the merge never special-cases a format.
type MergedBlueprint struct {
	Apt        []string
	Args       []string
	Dockerfile []string
	Compose    *yaml.Node
	Taskfile   *yaml.Node
}

// Assemble renders the three cabin files from the resolved blueprints into the
// provided writers; a nil writer field means that artifact is not requested. base
// (the minimal default structure) must be merged first by the caller-provided
// order of blueprints. It returns the first write error (Dockerfile text or YAML
// encode).
func Assemble(bps []fragments.BundleBlueprint, sel Selection, w *Files) error {
	m := Merge(bps)
	var errs []error
	if w.Dockerfile != nil {
		if e := m.writeDockerfile(w.Dockerfile); e != nil {
			errs = append(errs, e)
		}
	}
	if w.Compose != nil {
		if e := m.writeCompose(w.Compose, sel.Name); e != nil {
			errs = append(errs, e)
		}
	}
	if w.Taskfile != nil {
		if e := m.writeTaskfile(w.Taskfile, sel); e != nil {
			errs = append(errs, e)
		}
	}
	return errors.Join(errs...)
}

// Merge folds the blueprints into a single merged structure. It is format
// agnostic: list fields are appended, the compose and taskfile YAML mapping
// subtrees are deep-merged (lists appended, nested maps merged, scalars
// overridden) in bundle order. No deduplication happens here (apt is deduped
// by its writer).
func Merge(bps []fragments.BundleBlueprint) MergedBlueprint {
	m := MergedBlueprint{}
	for _, bp := range bps {
		m.Apt = append(m.Apt, bp.Apt...)
		m.Args = append(m.Args, bp.Args...)
		m.Dockerfile = append(m.Dockerfile, bp.Dockerfile...)
		m.Compose = mergeMapping(m.Compose, bp.Compose)
		m.Taskfile = mergeMapping(m.Taskfile, bp.Taskfile)
	}
	return m
}

// mergeMapping deep-merges src into dst (a mapping node). Both-mapping values
// recurse; both-sequence values append; anything else is overridden by src.
// dst is mutated and returned; src is never mutated (its nodes are cloned on
// insertion). A nil dst starts as a clone of src.
func mergeMapping(dst, src *yaml.Node) *yaml.Node {
	if src == nil {
		return dst
	}
	if dst == nil || dst.Kind != yaml.MappingNode || src.Kind != yaml.MappingNode {
		return cloneNode(src)
	}
	for i := 0; i+1 < len(src.Content); i += 2 {
		sk, sv := src.Content[i], src.Content[i+1]
		if j := findKeyNode(dst, sk.Value); j >= 0 {
			dv := dst.Content[j+1]
			switch {
			case dv.Kind == yaml.MappingNode && sv.Kind == yaml.MappingNode:
				mergeMapping(dv, sv)
			case dv.Kind == yaml.SequenceNode && sv.Kind == yaml.SequenceNode:
				dv.Content = append(dv.Content, cloneSeq(sv.Content)...)
			default:
				dst.Content[j+1] = cloneNode(sv)
			}
		} else {
			dst.Content = append(dst.Content, cloneNode(sk), cloneNode(sv))
		}
	}
	return dst
}

// findKeyNode returns the Content index of a mapping key with the given value.
func findKeyNode(m *yaml.Node, key string) int {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// cloneSeq clones a sequence of nodes.
func cloneSeq(nodes []*yaml.Node) []*yaml.Node {
	out := make([]*yaml.Node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, cloneNode(n))
	}
	return out
}

// cloneNode deep-copies a yaml.Node, preserving style and comments so the
// merged output keeps the author's formatting.
func cloneNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	c := &yaml.Node{
		Kind:        n.Kind,
		Style:       n.Style,
		Tag:         n.Tag,
		Value:       n.Value,
		Anchor:      n.Anchor,
		Alias:       n.Alias,
		HeadComment: n.HeadComment,
		LineComment: n.LineComment,
		FootComment: n.FootComment,
	}
	for _, child := range n.Content {
		c.Content = append(c.Content, cloneNode(child))
	}
	return c
}

// --- Writers ---------------------------------------------------------------

// writeDockerfile lays out FROM + args + merged apt RUN + the verbatim
// dockerfile body. A default CMD is appended unless a body line already
// declares one (opencode's CMD wins over the interactive sleep default). The
// body is emitted as many small writes, so it is buffered through an errWriter
// (the first error is returned once at the end); the caller just provides an
// io.Writer like the YAML writers.
func (m MergedBlueprint) writeDockerfile(ww io.Writer) error {
	w := &errWriter{w: ww}
	w.printf("# Generated by cabin authoring. Adapt FROM to your project base image.\n")
	w.printf("FROM %s\n\n", DefaultBaseImage)
	for _, a := range m.Args {
		w.printf("%s\n", a)
	}
	if len(m.Args) > 0 {
		w.printf("\n")
	}
	m.writeApt(w)
	hasCMD := false
	for _, l := range m.Dockerfile {
		if strings.HasPrefix(l, "CMD ") {
			hasCMD = true
		}
		w.printf("%s\n", l)
	}
	if len(m.Dockerfile) > 0 {
		w.printf("\n")
	}
	if !hasCMD {
		w.printf("# Default command: sleep infinity for manual testing\n")
		w.printf("CMD [\"sleep\", \"infinity\"]\n")
	}
	return w.err
}

// writeApt merges the apt lists (deduplicated here — a writer concern, the
// merge stays generic) into the classic Debian apt-get RUN with the usual
// lists cleanup. Emits nothing when no bundle contributes packages.
func (m MergedBlueprint) writeApt(w *errWriter) {
	if len(m.Apt) == 0 {
		return
	}
	var pkgs []string
	seen := make(map[string]bool)
	for _, p := range m.Apt {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		pkgs = append(pkgs, p)
	}
	w.printf("# Install all required packages in one step\n")
	w.printf("RUN apt-get update && apt-get install -y \\\n")
	for _, p := range pkgs {
		w.printf("    %s \\\n", p)
	}
	w.printf("    && rm -rf /var/lib/apt/lists/*\n\n")
}

// errWriter records the first write error and drops subsequent writes, so a
// writer that emits many parts can check its error once at the end instead of
// after every call.
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) Write(p []byte) (int, error) {
	if e.err != nil {
		return len(p), nil
	}
	n, err := e.w.Write(p)
	if err != nil {
		e.err = err
	}
	return n, err
}

func (e *errWriter) printf(format string, a ...any) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e, format, a...)
}

// writeCompose emits the agent service: the injected scaffold (build, image,
// hostname) followed by the merged compose content. The explicit image tag
// keeps the build shared across profile instances; container_name is omitted
// (daemon-global, would collide across projects). Comment nodes inside the
// merged content are preserved on marshal.
func (m MergedBlueprint) writeCompose(w io.Writer, name string) error {
	service := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	build := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	build.Content = append(build.Content,
		strNode("context"), strNode("."),
		strNode("dockerfile"), strNode(CabinDockerfile),
	)
	service.Content = append(service.Content,
		strNode("build"), build,
		strNode("image"), strNode(name),
		strNode("hostname"), strNode(name),
	)
	if m.Compose != nil {
		service.Content = append(service.Content, m.Compose.Content...)
	}

	agent := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	agent.Content = append(agent.Content, strNode("agent"), service)
	services := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	services.Content = append(services.Content, strNode("services"), agent)
	return encodeYAML(w, services)
}

// writeTaskfile emits the thin Taskfile: the injected ai-cabin: header built
// from the selection, followed by the merged taskfile content (base provides
// the default tasks/env/vars structure).
func (m MergedBlueprint) writeTaskfile(w io.Writer, sel Selection) error {
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	header := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	header.Content = append(header.Content,
		strNode("agents"), seqNode(sel.Agents),
	)
	if len(sel.Features) > 0 {
		header.Content = append(header.Content,
			strNode("features"), seqNode(sel.Features),
		)
	}
	root.Content = append(root.Content, strNode("ai-cabin"), header)
	if m.Taskfile != nil {
		root.Content = append(root.Content, m.Taskfile.Content...)
	}
	return encodeYAML(w, root)
}

// encodeYAML serializes a node tree with two-space indentation (the compose /
// Taskfile convention) directly to w, preserving comments.
func encodeYAML(w io.Writer, n *yaml.Node) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return err
	}
	return enc.Close()
}

// strNode builds a scalar string node.
func strNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

// seqNode builds a sequence node from scalars.
func seqNode(items []string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, it := range items {
		n.Content = append(n.Content, strNode(it))
	}
	return n
}
