package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// layerManifestName is the manifest at a layer root carrying the profile-var
// contribution. A layer without it contributes no vars (a fragments-only layer
// root is valid).
const layerManifestName = "layer.yaml"

// layerManifest is the on-disk shape of layer.yaml. Only the vars: block is
// read today; future layer metadata (version, compat) will extend this struct
// without changing the file format.
type layerManifest struct {
	Vars map[string]string `yaml:"vars"`
}

// LayerVars returns the profile-default vars contributed by the active layers.
// dirs must be the already-parsed AI_CABIN_LAYER_DIRS roots (SplitPathList).
// The first layer that has a layer.yaml contributes its whole vars: block;
// the layer.yaml of later layers are ignored (file-level first-wins, no
// intra-file merge — a user who wants a merged result writes a hand-merged
// layer.yaml and puts it first in the list). A layer with no layer.yaml is
// skipped; if none exists, LayerVars returns an empty Vars (no layer, no
// contribution — not an error). Read errors on a present layer.yaml are
// surfaced, not silently skipped.
func LayerVars(dirs []string) (Vars, error) {
	for _, dir := range dirs {
		manifestPath := filepath.Join(dir, layerManifestName)
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read layer manifest %q: %w", manifestPath, err)
		}
		var man layerManifest
		if err := yaml.Unmarshal(data, &man); err != nil {
			return nil, fmt.Errorf("parse layer manifest %q: %w", manifestPath, err)
		}
		return man.Vars, nil
	}
	return Vars{}, nil
}
