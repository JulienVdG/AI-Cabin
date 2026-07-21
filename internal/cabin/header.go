package cabin

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// AICabinHeader is the metadata read from a Taskfile's top-level "ai-cabin:"
// key. task ignores this key (it does not know it), but the cabin CLI reads it
// as the source of cabin identity: the registry name and the declared agents.
//
// All fields are optional: a header declaring only "ai-cabin: {}" (empty map)
// is valid, and the cabin name then falls back to the directory basename.
// The contract for a valid cabin is "the ai-cabin: block exists".
//
// Note on "empty": yaml.v3 decodes a bare "ai-cabin:" (YAML null) as a nil
// pointer, indistinguishable from an absent block. To declare an empty but
// present header, use "ai-cabin: {}" (empty map).
type AICabinHeader struct {
	Cabin  string   `yaml:"cabin"`
	Agents []string `yaml:"agents"`
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
}

// TaskfileName is the conventional name of the Taskfile in a cabin directory.
// task accepts Taskfile.yml / Taskfile.yaml / Taskfile.dist.yml; the cabin
// contract requires Taskfile.yml specifically (one canonical name).
const TaskfileName = "Taskfile.yml"

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
