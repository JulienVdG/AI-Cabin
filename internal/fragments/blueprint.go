// Package fragments blueprint facet: a feature bundle declares the authoring
// content an author writes when creating a cabin. Unlike the deps/setup facets
// (which materialize fragments to a destination tree), the blueprint facet is
// authoring-only — it is consumed by `cabin authoring`, never at runtime.
//
// The representation follows the target format: apt is a YAML list of package
// names; args and dockerfile are verbatim line blocks (Dockerfile is a text
// format, so the author's lines — continuations and comments included — are
// kept as written); compose and taskfile are YAML-native subtrees (kept as
// yaml.Node) so the engine can deep-merge them and re-serialize with comments
// preserved. Nothing is rendered at authoring time: `${...}` compose syntax and
// `{{...}}` task-time vars stay literal and resolve at their runtime.
package fragments

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/JulienVdG/AI-Cabin/internal/cabin"
)

// blueprintManifest is the manifest name for the blueprint facet, read by
// ResolveBlueprints. It lives at the bundle root (like deps.yaml/setup.yaml).
const blueprintManifest = "blueprint.yaml"

// BundleBlueprint is the resolved blueprint of a single active bundle. Compose
// and Taskfile are YAML mapping subtrees (kept as yaml.Node, merged by the
// authoring engine); Apt/Args/Dockerfile are verbatim lists. Err carries a
// read/parse error for the bundle without dropping the rest.
type BundleBlueprint struct {
	Name       string
	Apt        []string
	Args       []string
	Dockerfile []string
	Compose    *yaml.Node
	Taskfile   *yaml.Node
	Help       string
	Err        error
}

// ResolveBlueprints reads the blueprint.yaml of each active bundle from the
// merged fallback chain and returns the resolved blueprints in bundle order.
// A bundle with no blueprint.yaml contributes nothing and is skipped (e.g.
// port-forward has only deps/setup facets). Every bundle is attempted (no
// fail-fast) so one broken bundle (a read or parse error) does not hide the
// rest.
func ResolveBlueprints(merged fs.FS, bundles []cabin.FeatureRef) []BundleBlueprint {
	out := make([]BundleBlueprint, 0, len(bundles))
	for _, b := range bundles {
		bp := resolveBundleBlueprint(merged, b)
		if bp != nil {
			out = append(out, *bp)
		}
	}
	return out
}

// resolveBundleBlueprint reads and resolves a single bundle's blueprint.yaml.
// A missing manifest is a no-op (nil): the bundle has no blueprint facet. A
// malformed manifest (bad YAML or a non-mapping document) is reported via
// BundleBlueprint.Err so the caller can surface it without aborting.
func resolveBundleBlueprint(merged fs.FS, b cabin.FeatureRef) *BundleBlueprint {
	manifestPath := path.Join(b.Name, blueprintManifest)
	data, err := fs.ReadFile(merged, manifestPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // no blueprint facet for this bundle
		}
		return &BundleBlueprint{Name: b.Name, Err: fmt.Errorf("read manifest %q: %w", manifestPath, err)}
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return &BundleBlueprint{Name: b.Name, Err: fmt.Errorf("parse manifest %q: %w", manifestPath, err)}
	}
	root := &doc
	if len(doc.Content) > 0 {
		root = doc.Content[0] // unwrap the document node to the root mapping
	}
	if root.Kind != yaml.MappingNode {
		return &BundleBlueprint{Name: b.Name, Err: fmt.Errorf("manifest %q: root is not a mapping", manifestPath)}
	}

	return &BundleBlueprint{
		Name:       b.Name,
		Apt:        stringSeq(root, "apt"),
		Args:       verbatimList(root, "args"),
		Dockerfile: verbatimList(root, "dockerfile"),
		Compose:    nodeAt(root, "compose"),
		Taskfile:   nodeAt(root, "taskfile"),
		Help:       scalarAt(root, "help"),
	}
}

// findKey returns the Content index of the mapping key with the given value,
// or -1 when absent. Mapping Content alternates key/value.
func findKey(root *yaml.Node, key string) int {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// stringSeq reads a key whose value is a sequence of scalars (a YAML list).
func stringSeq(root *yaml.Node, key string) []string {
	i := findKey(root, key)
	if i < 0 || root.Content[i+1].Kind != yaml.SequenceNode {
		return nil
	}
	seq := root.Content[i+1]
	out := make([]string, 0, len(seq.Content))
	for _, n := range seq.Content {
		out = append(out, n.Value)
	}
	return out
}

// verbatimList reads a key whose value is either a verbatim scalar block
// (dockerfile:/args:) or a sequence, and returns the line list with leading and
// trailing blank lines stripped but internal blanks kept.
func verbatimList(root *yaml.Node, key string) []string {
	i := findKey(root, key)
	if i < 0 {
		return nil
	}
	v := root.Content[i+1]
	switch v.Kind {
	case yaml.ScalarNode:
		return splitLines(v.Value)
	case yaml.SequenceNode:
		out := make([]string, 0, len(v.Content))
		for _, n := range v.Content {
			out = append(out, n.Value)
		}
		return out
	}
	return nil
}

// splitLines splits a verbatim block into lines, dropping leading and trailing
// blank lines but keeping interior blanks.
func splitLines(s string) []string {
	lines := strings.Split(s, "\n")
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}

// nodeAt returns the mapping value node at key when it is a mapping (compose /
// taskfile subtrees), else nil.
func nodeAt(root *yaml.Node, key string) *yaml.Node {
	i := findKey(root, key)
	if i < 0 {
		return nil
	}
	if root.Content[i+1].Kind != yaml.MappingNode {
		return nil
	}
	return root.Content[i+1]
}

// scalarAt returns the scalar value at key (e.g. help), empty when absent.
func scalarAt(root *yaml.Node, key string) string {
	i := findKey(root, key)
	if i < 0 {
		return ""
	}
	return root.Content[i+1].Value
}
