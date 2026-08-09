// Package fragments is the business layer that builds the concrete fallback
// chain layers and materializes a bundle's facets into a destination tree. It
// wires the io/fs bricks (internal/unionfs for the layered union, internal/render
// for .tmpl rendering) and is the analogue of internal/task (which wires the
// go-task/task/v3 lib): a thin business layer with no embed directive and no UX
// (no printing, no --force — the CLI in cmd/cabin owns the UX).
//
// The fallback chain is a union fs.FS (first-wins like $PATH):
// AI_CABIN_FRAGMENTS_DIRS entries (conf) > cabin-local dir (dev) > embedded
// base layer. BuildLayers constructs it from os.DirFS layers + the embed FS.
//
// Materialize reads a bundle's manifest (deps.yaml or setup.yaml) and writes
// each declared fragment to a destination root (<cabin>/.deps for deps,
// $AI_CABIN_HOME for setup). The two facets share one function because they
// are symmetric: each is driven by a central manifest mapping src -> dst, and
// dst is templated the same way in both.
package fragments

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/JulienVdG/AI-Cabin/internal/cabin"
	"github.com/JulienVdG/AI-Cabin/internal/render"
	"github.com/JulienVdG/AI-Cabin/internal/unionfs"
	"github.com/JulienVdG/AI-Cabin/internal/writestrategy"
)

// walkFS is the contract Materialize needs from the merged fallback chain:
// fs.FS for Open, fs.ReadDirFS for ReadDir, fs.StatFS for Stat. fs.WalkDir
// requires all three. unionfs.New and fstest.MapFS both implement them; making
// the contract explicit avoids a hidden runtime type assertion (a caller that
// passes a plain fs.FS gets a clear error instead of a WalkDir panic).
type walkFS interface {
	fs.FS
	fs.ReadDirFS
	fs.StatFS
}

const (
	// tmplSuffix marks a fragment whose content is rendered via internal/render
	// (Ansible .j2 convention). The suffix is stripped from the destination
	// filename in mirror mode.
	tmplSuffix = ".tmpl"
	// tmplOpen is the text/template opening delimiter; a dst containing it is
	// rendered as a template (port-forward multi-instance naming).
	tmplOpen = "{{"
)

// BuildLayers constructs the fallback chain as a union fs.FS, ordered highest
// priority first (first-wins like $PATH): the conf dirs, then the cabin-local
// dir (dev), then the embedded base layer. Each conf dir is an os.DirFS layer;
// cabin-local is an os.DirFS on the cabin dir; embedFS is the base (typically
// embedded.Fragments()).
//
// The conf dirs come pre-resolved from config.Vars.FragmentsDirs (which parses
// AI_CABIN_FRAGMENTS_DIRS: comma-split + ~ expansion); BuildLayers does not
// re-parse the env var. A missing dir is a strict error (no silent skip of a
// typo'd path — a misconfigured override layer should fail loudly, not
// silently drop). Returns an error if no layer is configured at all.
func BuildLayers(dirs []string, cabinDir string, embedFS fs.FS) (fs.FS, error) {
	var layers []fs.FS

	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			return nil, fmt.Errorf("fragment dir %q: %w", dir, err)
		}
		layers = append(layers, os.DirFS(dir))
	}

	if cabinDir != "" {
		if _, err := os.Stat(cabinDir); err != nil {
			return nil, fmt.Errorf("cabin-local fragment dir %q: %w", cabinDir, err)
		}
		layers = append(layers, os.DirFS(cabinDir))
	}

	if embedFS != nil {
		layers = append(layers, embedFS)
	}

	if len(layers) == 0 {
		return nil, errors.New("no fragment layers configured (set AI_CABIN_FRAGMENTS_DIRS, provide a cabin dir, or an embed FS)")
	}

	return unionfs.New(layers...), nil
}

// manifest is the on-disk deps.yaml/setup.yaml file. It has two mutually
// exclusive declaration modes:
//   - mirror: copy a whole subtree rooted at <mirrorDir> into destBase/,
//     preserving the internal structure (with .tmpl stripped from filenames).
//   - entries: an explicit src→dst list, for bundles that need templated dst
//     (port-forward multi-instance) or a non-mirrored layout.
type manifest struct {
	Mirror           string          `yaml:"mirror"`
	Entries          []manifestEntry `yaml:"entries"`
	GreywallProfiles []string        `yaml:"greywall_profiles"`
}

// manifestEntry maps a fragment source to its destination. Src is relative to
// the bundle root in the merged FS; dst is relative to destBase and may contain
// template vars ({{.host}}) rendered at materialization.
type manifestEntry struct {
	Src string `yaml:"src"`
	Dst string `yaml:"dst"`
}

// resolvedEntry is a fragment ready to materialize: src is relative to the
// bundle root in the merged FS, dst is the raw destination path (relative to
// destBase, before name templating). The destination mode is always writestrategy.FilePerm
// (umask-applied): the source mode is not preserved (embed.FS exposes every
// file as 0444, an artifact; the executable bit is the Dockerfile's
// authority, blueprint facet).
type resolvedEntry struct {
	src string
	dst string
}

// validate checks the manifest declares a usable mode: mirror and entries
// are mutually exclusive (use only one); greywall_profiles is orthogonal and
// may combine with either (or stand alone, a no-op materialize for a bundle
// that only references built-in profiles). A manifest declaring none of the
// three is rejected as useless. Materialize relies on this: a greywall_profiles
// -only manifest must pass validate, then expand returns an empty entry list
// (no-op write).
func (m *manifest) validate() error {
	hasMirror := m.Mirror != ""
	hasEntries := len(m.Entries) > 0
	hasProfiles := len(m.GreywallProfiles) > 0
	switch {
	case hasMirror && hasEntries:
		return errors.New("manifest has both mirror and entries (use only one)")
	case !hasMirror && !hasEntries && !hasProfiles:
		return errors.New("manifest has neither mirror, entries, nor greywall_profiles")
	}
	return nil
}

// setupManifestName is the manifest name for the setup facet, read by
// ResolveGreywallProfiles to derive the greywall profile list from a bundle's
// shipped profiles and built-in references.
const setupManifestName = "setup.yaml"

// learnedDir is the destination subdir (relative to $AI_CABIN_HOME) where
// greywall learned profiles are seeded. A setup.yaml entry whose dst contains
// this marker contributes a shipped profile (the profile name is the stem of
// the rendered dst). The forward-slash form matches dst paths, which are
// forward-slash relative to destBase regardless of OS.
const learnedDir = "greywall/learned/"

// ResolveGreywallProfiles derives the greywall profile list for a cabin from
// its active bundles: shipped profiles (setup.yaml entries whose dst is under
// .config/greywall/learned/, name = stem of the rendered dst) plus built-in
// references (the greywall_profiles: manifest field). The list is ordered
// (bundle order from cabin.ActiveBundles, base first) and deduplicated by
// first occurrence — base ships learned/workspace.json so workspace leads
// naturally, no special-casing. A bundle with no setup.yaml (e.g. git-agent,
// or a port-forward deps-only variant) contributes nothing and is not an
// error. Errors are collected per-bundle (no fail-fast): a malformed manifest
// or an undefined var in a templated dst is reported alongside the profiles
// resolved so far.
//
// Only the entries: mode is scanned for shipped profiles (all current setup
// manifests use entries:). A bundle that mirrors: a subtree containing learned
// profiles would not be detected; such a bundle should declare them via
// greywall_profiles: or use entries: for the profiles.
func ResolveGreywallProfiles(merged fs.FS, bundles []cabin.FeatureRef, vars map[string]string) ([]string, error) {
	wfs, ok := merged.(walkFS)
	if !ok {
		return nil, errors.New("merged fs does not implement ReadDirFS+StatFS (required to read setup manifests)")
	}

	var profiles []string
	seen := make(map[string]bool)
	var errs []error
	for _, b := range bundles {
		names, err := bundleGreywallProfiles(wfs, b, vars)
		if err != nil {
			errs = append(errs, fmt.Errorf("bundle %q: %w", b.Name, err))
			continue
		}
		for _, n := range names {
			if n == "" || seen[n] {
				continue
			}
			seen[n] = true
			profiles = append(profiles, n)
		}
	}

	if len(errs) > 0 {
		return profiles, errors.Join(errs...)
	}
	return profiles, nil
}

// bundleGreywallProfiles reads a single bundle's setup.yaml and returns the
// greywall profile names it contributes: built-in references (greywall_profiles:)
// followed by shipped profiles (entries whose dst is under greywall/learned/,
// name = stem of the rendered dst). A missing setup.yaml is a no-op (the
// bundle has no setup facet), not an error. The manifest is not validated here
// — ResolveGreywallProfiles reads metadata, not the copy mode, and a manifest
// with greywall_profiles + entries is a valid combination.
func bundleGreywallProfiles(wfs walkFS, b cabin.FeatureRef, vars map[string]string) ([]string, error) {
	manifestPath := path.Join(b.Name, setupManifestName)
	data, err := fs.ReadFile(wfs, manifestPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil // no setup facet for this bundle
		}
		return nil, fmt.Errorf("read manifest %q: %w", manifestPath, err)
	}

	var man manifest
	if err := yaml.Unmarshal(data, &man); err != nil {
		return nil, fmt.Errorf("parse manifest %q: %w", manifestPath, err)
	}

	var profiles []string
	// Built-in references (e.g. go -> built-in greywall go profile).
	profiles = append(profiles, man.GreywallProfiles...)

	// Shipped profiles: entries whose dst is under greywall/learned/. The dst may
	// be templated (port-forward: forward-{{.host}}-{{.port}}.json) and is
	// rendered with the bundle attrs + profile vars before extracting the name.
	for _, e := range man.Entries {
		if !strings.Contains(e.Dst, learnedDir) {
			continue
		}
		dst := e.Dst
		if strings.Contains(dst, tmplOpen) {
			rendered, rerr := render.RenderString(dst, vars, b.Attrs)
			if rerr != nil {
				return nil, fmt.Errorf("render profile dst %q: %w", e.Dst, rerr)
			}
			dst = rendered
		}
		profiles = append(profiles, profileNameFromDst(dst))
	}
	return profiles, nil
}

// profileNameFromDst extracts the greywall profile name from a learned/ dst
// path: the stem of the basename (without the .json suffix). E.g.
// ".config/greywall/learned/forward-mariadb-3306.json" -> "forward-mariadb-3306".
func profileNameFromDst(dst string) string {
	return strings.TrimSuffix(path.Base(dst), ".json")
}

// expand turns the manifest into a flat list of resolved entries (with
// resolved source mode), collecting — not aborting on — per-entry problems so
// the user sees every manifest issue in one run (no fail-fast): an empty
// src/dst, or a src that does not exist in merged (drift), is skipped and its
// error joined into the returned error. The valid entries are still returned
// so the loop can materialize them. Mirror mode walks <bundle>/<mirrorDir> (a
// WalkDir error yields the partial entries collected so far + the error).
// This is the resolve+validate phase; the Materialize loop is the write phase
// and never stats again.
func (m *manifest) expand(wfs walkFS, bundle, manifestName string) ([]resolvedEntry, error) {
	if m.Mirror != "" {
		return expandMirror(wfs, bundle, m.Mirror)
	}
	out := make([]resolvedEntry, 0, len(m.Entries))
	var errs []error
	for _, e := range m.Entries {
		if e.Src == "" {
			errs = append(errs, fmt.Errorf("manifest %q: entry has empty src", manifestName))
			continue
		}
		if e.Dst == "" {
			errs = append(errs, fmt.Errorf("manifest %q: entry has empty dst", manifestName))
			continue
		}
		srcPath := path.Join(bundle, e.Src)
		if _, err := fs.Stat(wfs, srcPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, fmt.Errorf("bundle %q: manifest %q references %q which does not exist: %w",
					bundle, manifestName, e.Src, err))
				continue
			}
			errs = append(errs, fmt.Errorf("stat fragment %q: %w", srcPath, err))
			continue
		}
		out = append(out, resolvedEntry{src: e.Src, dst: e.Dst})
	}
	return out, errors.Join(errs...)
}

// expandMirror walks the mirror subtree and produces one resolvedEntry per
// file. A WalkDir error (e.g. unreadable subdir) yields the entries collected
// so far plus the error, so a partial mirror still materializes and the error
// is reported alongside the rest.
func expandMirror(wfs walkFS, bundle, mirrorDir string) ([]resolvedEntry, error) {
	root := path.Join(bundle, mirrorDir)
	out := make([]resolvedEntry, 0)
	err := fs.WalkDir(wfs, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := pathRel(root, p)
		out = append(out, resolvedEntry{
			src: path.Join(mirrorDir, rel),
			dst: stripTmplSuffix(rel),
		})
		return nil
	})
	return out, err
}

// pathRel returns the relative path of target under base, both forward-slash
// paths relative to an fs.FS root. WalkDir only produces paths under root, so
// TrimPrefix always strips the prefix — no unreachable defensive branch.
func pathRel(base, target string) string {
	return strings.TrimPrefix(target, base+"/")
}

// stripTmplSuffix removes a trailing .tmpl from the last path component. The
// .tmpl marker means "render the content"; the destination file does not keep
// it. TrimSuffix is a no-op when the suffix is absent.
func stripTmplSuffix(p string) string {
	dir, file := path.Split(p)
	return path.Join(dir, strings.TrimSuffix(file, tmplSuffix))
}

// Materializer carries the stable inputs to materializing a bundle facet: the
// merged fallback chain, the facet's manifest + destination + writer, and the
// resolved vars. Only the bundle name (and per-bundle attrs) vary per call, so
// the loop in cmd/cabin constructs one Materializer and calls Materialize per
// active bundle. The facet (deps vs setup) is carried by manifestName
// ("deps.yaml" or "setup.yaml") and opener (TruncateCreator for deps,
// BackupCreator for setup); destBase is the destination root (<cabin>/.deps for
// deps, $AI_CABIN_HOME for setup — resolved by the caller). Constructed via
// NewMaterializer, which fails fast if the merged FS does not implement
// ReadDirFS+StatFS (required by WalkDir).
type Materializer struct {
	fs           walkFS
	manifestName string
	destBase     string
	vars         map[string]string
	opener       writestrategy.FileCreator
}

// NewMaterializer builds a Materializer. Use TruncateCreator for the deps facet
// (throwaway .deps/) and BackupCreator for the setup facet (persistent
// $AI_CABIN_HOME). Returns an error if merged does not implement
// ReadDirFS+StatFS (required by WalkDir) — failing at construction rather than
// on the first Materialize call.
func NewMaterializer(merged fs.FS, manifestName, destBase string, vars map[string]string, opener writestrategy.FileCreator) (*Materializer, error) {
	wfs, ok := merged.(walkFS)
	if !ok {
		return nil, errors.New("merged fs does not implement ReadDirFS+StatFS (required by WalkDir)")
	}
	return &Materializer{fs: wfs, manifestName: manifestName, destBase: destBase, vars: vars, opener: opener}, nil
}

// Materialize reads <bundle>/<manifestName> from the merged FS and writes each
// declared fragment to destBase/<dst>.
//
// A src ending in .tmpl is rendered via internal/render; a dst containing {{
// is rendered with the same vars/attrs (port-forward multi-instance naming).
// A manifest absent for a bundle facet is a no-op (the bundle has no such
// facet — e.g. port-forward has deps.yaml only). Bundle-absent, manifest drift,
// undefined vars, and I/O errors are ALL collected (no fail-fast) so the user
// sees every problem in one run; partial files are expected and left in place
// (écriture-malgré-erreur is the base design for undefined vars, and removing
// partials would add error logic for little gain). The two abort cases are a
// malformed manifest (bad YAML or ambiguous mirror/entries): neither can
// produce a usable entry list. render.ErrUndefinedVar is the one sentinel —
// nothing else is, the remaining errors wrap fs.ErrNotExist or carry their
// context in the message.
//
// Returns the list of successfully written relpaths (relative to destBase)
// and an optional aggregated error. It is business-logic-only: the CLI formats
// progress and the error for the user.
func (m *Materializer) Materialize(bundle string, attrs map[string]any) ([]string, error) {
	// Bundle must exist in at least one layer.
	if _, err := fs.Stat(m.fs, bundle); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("bundle %q not found in any fragment layer: %w", bundle, err)
		}
		return nil, fmt.Errorf("stat bundle %q: %w", bundle, err)
	}

	// Manifest: absent = no-op (bundle has no such facet).
	manifestPath := path.Join(bundle, m.manifestName)
	data, err := fs.ReadFile(m.fs, manifestPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read manifest %q: %w", manifestPath, err)
	}

	var man manifest
	if err := yaml.Unmarshal(data, &man); err != nil {
		return nil, fmt.Errorf("parse manifest %q: %w", manifestPath, err)
	}
	if err := man.validate(); err != nil {
		return nil, fmt.Errorf("manifest %q: %w", manifestPath, err)
	}

	// expand resolves modes + collects drift/per-entry issues (no fail-fast):
	// valid entries are returned alongside the aggregated error.
	entries, expandErr := man.expand(m.fs, bundle, m.manifestName)
	var errs []error
	if expandErr != nil {
		errs = append(errs, fmt.Errorf("manifest %q: %w", manifestPath, expandErr))
	}

	var written []string
	for _, e := range entries {
		srcPath := path.Join(bundle, e.src)

		// Resolve the dst name first: if it contains {{, render it (port-forward
		// multi-instance naming). An undefined var in the name skips this file
		// (the path cannot be resolved) and is collected like any other error.
		dstRel := e.dst
		if strings.Contains(dstRel, tmplOpen) {
			rendered, rerr := render.RenderString(dstRel, m.vars, attrs)
			if rerr != nil {
				errs = append(errs, fmt.Errorf("bundle %q dst %q: %w", bundle, e.dst, rerr))
				continue
			}
			dstRel = rendered
		}

		dstPath := filepath.Join(m.destBase, filepath.FromSlash(dstRel))
		if err := os.MkdirAll(filepath.Dir(dstPath), writestrategy.DirPerm); err != nil {
			errs = append(errs, fmt.Errorf("mkdir %q: %w", filepath.Dir(dstPath), err))
			continue
		}

		// .tmpl renders (Parse streams from FS, Execute streams to the writer);
		// plain files copy as-is. Either way a partial file is left on error.
		var ferr error
		if strings.HasSuffix(e.src, tmplSuffix) {
			ferr = m.renderFragment(srcPath, dstPath, attrs)
		} else {
			ferr = m.copyFragment(srcPath, dstPath)
		}
		if ferr != nil {
			errs = append(errs, ferr)
			continue
		}
		written = append(written, dstRel)
	}

	if len(errs) > 0 {
		return written, errors.Join(errs...)
	}
	return written, nil
}

// renderFragment parses a .tmpl fragment from the merged FS and renders it to
// dst via internal/render. The destination is opened via the Materializer's
// opener. On any error a partial file is left in place. Returns an error
// naming both src and dst.
func (m *Materializer) renderFragment(srcPath, dstPath string, attrs map[string]any) error {
	tmpl, err := render.Parse(m.fs, srcPath)
	if err != nil {
		return fmt.Errorf("render %q -> %q: %w", srcPath, dstPath, err)
	}
	w, err := m.opener.Create(dstPath)
	if err != nil {
		return fmt.Errorf("render %q -> %q: %w", srcPath, dstPath, err)
	}
	defer w.Close()
	if err := render.Execute(tmpl, m.vars, attrs, w); err != nil {
		return fmt.Errorf("render %q -> %q: %w", srcPath, dstPath, err)
	}
	return nil
}

// copyFragment streams a plain (non-template) fragment from the merged FS to
// dst without buffering the whole content in memory. io.Copy uses ReaderFrom
// / WriterTo when available (sendfile for os.File-to-os.File copies via
// os.DirFS layers). The destination is opened via the Materializer's opener;
// both files are closed via defer; a mid-copy error leaves a partial
// destination in place (consistent with renderFragment).
func (m *Materializer) copyFragment(srcPath, dstPath string) error {
	src, err := m.fs.Open(srcPath)
	if err != nil {
		return fmt.Errorf("copy %q -> %q: %w", srcPath, dstPath, err)
	}
	defer src.Close()

	dst, err := m.opener.Create(dstPath)
	if err != nil {
		return fmt.Errorf("copy %q -> %q: %w", srcPath, dstPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy %q -> %q: %w", srcPath, dstPath, err)
	}
	return nil
}
