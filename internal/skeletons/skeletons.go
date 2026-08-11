// Package skeletons is the Class 1 scaffolding facade over the fragments
// engine. A skeleton is a fragment with a mandatory skeleton.yaml manifest:
// desks mirror a content/ subtree (flat copy), project skeletons use entries:
// (templated destination names and per-instance attrs). The concern (desk vs
// project) drives the destination via the caller; the copy/template mechanism
// is the shared fragments Materializer, so skeleton and bundle authoring stay
// one engine.
//
// The manifest is mandatory (absent = error): a directory without skeleton.yaml
// is not a skeleton. Desks use mirror: content (the manifest itself lives
// outside content/, so it is not copied); project skeletons use entries: when
// they need templated destination names (cmd/{{.project}}/main.go) or
// per-instance attrs ({{.module}}).
package skeletons

import (
	"fmt"
	"io/fs"
	"path"

	"github.com/JulienVdG/AI-Cabin/internal/fragments"
	"github.com/JulienVdG/AI-Cabin/internal/writestrategy"
)

// ManifestName is the mandatory manifest file at the skeleton root. Reuses the
// fragments manifest syntax (mirror:/entries: + optional greywall_profiles:),
// read by fragments.Materializer. A skeleton without it is rejected.
const ManifestName = "skeleton.yaml"

// BuildLayers constructs the skeleton fallback chain as a union fs.FS, ordered
// highest priority first (first-wins like $PATH): the conf dirs, then the
// embedded base layer. It reuses fragments.BuildLayers (same unionfs assembly);
// skeletons have no cabin-local layer, so the cabin dir is empty. The conf dirs
// come pre-resolved from config.Vars.SkeletonDirs (which parses
// AI_CABIN_SKELETON_DIRS); a missing dir is a strict error.
func BuildLayers(dirs []string, embedFS fs.FS) (fs.FS, error) {
	return fragments.BuildLayers(dirs, "", embedFS)
}

// Apply copies a Class 1 skeleton to dest via the fragments engine. The
// skeleton.yaml manifest is mandatory (absent = error); its mirror:/entries:
// mode drives the copy, .tmpl contents are rendered via internal/render (attrs
// top-level as {{.attr}}, profile vars namespaced as {{.Vars.X}}), and a
// destination name containing {{ is rendered the same way.
//
// merged must implement ReadDirFS+StatFS (required by WalkDir); BuildLayers
// (unionfs.New) and os.DirFS on go1.21+ both satisfy it. creator carries the
// write policy (SkipCreator no-overwrite default, TruncateCreator for --force).
// attrs are the per-instance values (e.g. project, module) from the caller
// (CLI flags + positional); nil is valid for desks that take no attrs.
//
// Returns the list of successfully written relpaths (relative to dest) and an
// optional aggregated error. It is business-logic-only: the CLI formats progress
// and the error for the user.
func Apply(merged fs.FS, skeletonName, dest string, vars map[string]string, attrs map[string]any, creator writestrategy.FileCreator) ([]string, error) {
	// The manifest is mandatory: a directory without skeleton.yaml is not a
	// skeleton. Open (not Stat) so the check works on any fs.FS that can Open,
	// before delegating to NewMaterializer (which asserts ReadDirFS+StatFS).
	manifestPath := path.Join(skeletonName, ManifestName)
	f, err := merged.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("skeleton %q has no %s manifest (required): %w", skeletonName, ManifestName, err)
	}
	f.Close()

	mat, err := fragments.NewMaterializer(merged, ManifestName, dest, vars, creator)
	if err != nil {
		return nil, err
	}
	return mat.Materialize(skeletonName, attrs)
}
