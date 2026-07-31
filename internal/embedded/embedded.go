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
