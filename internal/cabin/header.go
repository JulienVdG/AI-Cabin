package cabin

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// AICabinHeader is the metadata read from a Taskfile's top-level "ai-cabin:"
// key. task ignores this key (it does not know it), but the cabin CLI reads it
// as the source of cabin identity: the registry name, the declared agents,
// and the feature bundles. `agents: [pi]` is a shorthand for
// `features: [agent-pi]`; see ActiveBundles for the resolution.
//
// All fields are optional: a header declaring only "ai-cabin: {}" (empty map)
// is valid, and the cabin name then falls back to the directory basename.
// The contract for a valid cabin is "the ai-cabin: block exists".
//
// Note on "empty": yaml.v3 decodes a bare "ai-cabin:" (YAML null) as a nil
// pointer, indistinguishable from an absent block. To declare an empty but
// present header, use "ai-cabin: {}" (empty map).
type AICabinHeader struct {
	Cabin    string       `yaml:"cabin"`
	Agents   []string     `yaml:"agents"`
	Features []FeatureRef `yaml:"features"`
}

// FeatureRef is a feature bundle selected in the header's `features:` list,
// carrying optional attrs used as top-level template vars ({{.port}}) by
// internal/render (profile vars are namespaced as {{.Vars.X}}). Two YAML forms
// are accepted under `features:`:
//   - a bare string:        `- git-agent`
//   - a single-key mapping: `- port-forward: {port: 3306, host: mariadb}`
//
// Attrs travel with the bundle so the CLI can pass them to
// fragments.MaterializeDeps/MaterializeSetup alongside the bundle name.
type FeatureRef struct {
	Name  string
	Attrs map[string]any
}

// UnmarshalYAML accepts both the bare-string and single-key-mapping forms for
// a features: entry. A bare string yields Name with no attrs. A mapping must
// have exactly one key (the feature name); its value is the attrs map (or null
// for no attrs, e.g. `- git-agent:`). Any other YAML kind is a strict error.
func (f *FeatureRef) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		f.Name = value.Value
		f.Attrs = nil
		return nil
	case yaml.MappingNode:
		if len(value.Content)%2 != 0 {
			return fmt.Errorf("feature item is a malformed mapping")
		}
		if n := len(value.Content) / 2; n != 1 {
			return fmt.Errorf("feature item must be a single key, got %d", n)
		}
		keyNode, valNode := value.Content[0], value.Content[1]
		f.Name = keyNode.Value
		if valNode.Kind == 0 || valNode.Tag == "!!null" {
			// null / empty value (e.g. "- git-agent:"): no attrs.
			f.Attrs = nil
			return nil
		}
		var attrs map[string]any
		if err := valNode.Decode(&attrs); err != nil {
			return fmt.Errorf("decode attrs for feature %q: %w", f.Name, err)
		}
		f.Attrs = attrs
		return nil
	default:
		return fmt.Errorf("feature item must be a string or single-key mapping, got kind %d", value.Kind)
	}
}

// taskfileHeader wraps a Taskfile so we can unmarshal only the "ai-cabin:" key.
// Unknown top-level keys (version, vars, tasks, ...) are ignored by yaml.Unmarshal.
//
// AICabin is a pointer so yaml distinguishes the three cases natively:
//   - "ai-cabin:" absent from the YAML     -> AICabin == nil
//   - "ai-cabin: {}" (empty map)          -> AICabin != nil, zero struct
//   - "ai-cabin:" with sub-fields          -> AICabin != nil, struct filled
//
// (A bare "ai-cabin:" is YAML null == absent in yaml.v3; use "{}" for empty.)
type taskfileHeader struct {
	AICabin *AICabinHeader `yaml:"ai-cabin"`
	Vars    map[string]any `yaml:"vars"`
}

// TaskfileName is the conventional name of the Taskfile in a cabin directory.
// task accepts Taskfile.yml / Taskfile.yaml / Taskfile.dist.yml; the cabin
// contract requires Taskfile.yml specifically (one canonical name).
const TaskfileName = "Taskfile.yml"

// DefaultAgentService is the v1 convention for the compose service running the
// agent in a cabin (vars.AGENT_SERVICE in the Taskfile). `cabin ps` matches
// the agent container via its com.docker.compose.service label against this
// name when AGENT_SERVICE is not declared in the Taskfile.
const DefaultAgentService = "agent"

// AgentService extracts the agent compose service name from a Taskfile's
// top-level vars: block (vars.AGENT_SERVICE). Returns DefaultAgentService when
// the var is absent, not a string, or empty — a cabin relying on the v1
// convention. Never errors: this is a lookup with a fallback, not a
// validation. Used by `cabin ps` to identify the agent container among a
// compose project's services.
func AgentService(data []byte) string {
	var tf taskfileHeader
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return DefaultAgentService
	}
	if v, ok := tf.Vars["AGENT_SERVICE"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return DefaultAgentService
}

// ParseHeader parses the ai-cabin metadata from raw Taskfile bytes. It returns
// a non-nil *AICabinHeader when the "ai-cabin:" block is present (even if
// empty), and nil when the block is absent. A nil pointer is NOT an error:
// callers decide whether a missing header is a problem (ValidateCabin treats
// it as "not a cabin"). An error is returned only for invalid YAML.
func ParseHeader(data []byte) (*AICabinHeader, error) {
	var tf taskfileHeader
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("parse Taskfile yaml: %w", err)
	}
	return tf.AICabin, nil
}

// BaseBundle is the always-active bundle (greywall + greyproxy). Every
// cabin is sandboxed by greywall, so base is materialized unconditionally
// (first in the active list) and is not selectable in the header.
const BaseBundle = "base"

// ActiveBundles returns the active feature bundles for a cabin, derived from
// its header and ordered: base (always first), then each `agents:` entry as
// `agent-<name>` (the shorthand: `agents:[pi]` == `features:[agent-pi]`), then
// each `features:` entry in declaration order. There is no deduplication: a
// bundle may legitimately appear more than once — most notably port-forward,
// which models one instance per forwarded service (two entries with different
// attrs are both kept, not collapsed). A genuine duplicate (e.g. the same
// agent declared via both `agents:` and `features:`) is a user mistake that
// surfaces as a benign double-write in Materialize (idempotent on .deps/).
// Returns nil if header is nil (an invalid cabin).
func ActiveBundles(header *AICabinHeader) []FeatureRef {
	if header == nil {
		return nil
	}
	out := []FeatureRef{{Name: BaseBundle}}
	for _, a := range header.Agents {
		out = append(out, FeatureRef{Name: "agent-" + a})
	}
	return append(out, header.Features...)
}
