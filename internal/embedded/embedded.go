// Package embedded ships the default AI-Cabin assets built into the binary.
// The assets live under root/ and are exposed via a single embed.FS with
// fs.Sub views per sub-tree, so the source tree stays discoverable in one
// place (rather than embed directives scattered across packages).
//
// internal/fragments consumes Fragments() as the base layer of the fallback
// chain (AI_CABIN_FRAGMENTS_DIRS > cabin-local > embedded). It never references
// "root/fragments" by string, so a future rename of the sub-tree does not
// ripple.
package embedded

import (
	"embed"
	"io/fs"
)

// rootFS holds every embedded asset under root/. The "all:" prefix is
// required because Go silently skips _-prefixed dirs without it (protection
// against future .gitkeep/_internal entries).
//
//go:embed all:root
var rootFS embed.FS

// Fragments returns the embedded default fragments as a rooted fs.FS: paths
// are "base/deps/..." without the "root/fragments/" prefix. This is the base
// layer consumed by internal/fragments.BuildLayers.
func Fragments() (fs.FS, error) {
	return fs.Sub(rootFS, "root/fragments")
}

// State returns the embedded state artifacts as a rooted fs.FS (paths are
// "Taskfile.lifecycle.yml", ... without the "root/state/" prefix). These are
// cross-cabin files the CLI materializes to XDG state (config.GetStateDir)
// before `task` parses a cabin Taskfile. The tree is flat: an artifact here is
// cross-cabin and references cabin-owned targets, so it belongs to no bundle.
// Consumed by internal/state.EnsureArtifact.
func State() (fs.FS, error) {
	return fs.Sub(rootFS, "root/state")
}

// Skeletons returns the embedded Class 1 scaffolding trees as a rooted fs.FS
// (paths are "desks/minimal/..." without the "root/skeletons/" prefix). Typed
// by concern: `desks/` holds desk skeletons copied to the profile's
// AI_CABIN_DESK by `cabin setup` / `cabin profile init`. The embedded tree
// ships the `minimal` desk skeleton (the zero-config default of `cabin setup`);
// richer skeletons live in the repo under skeletons/desks/ and are resolved
// via --skeleton <path>. Consumed by internal/skeletons.Apply.
func Skeletons() (fs.FS, error) {
	return fs.Sub(rootFS, "root/skeletons")
}
