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

// filePerm is the mode used to create destination files: a plain 0666 so the
// umask reduces it to 0644 (writable by the owner, idempotent on re-run). The
// source mode is not preserved (embed.FS exposes every embedded file as 0444,
// an artifact of the Go embed package; the executable bit is the Dockerfile's
// authority via RUN chmod +x, blueprint facet).
const filePerm os.FileMode = 0o666

// dirPerm is the mode used to create destination directories: a plain 0777 so
// the umask reduces it to 0755. Symmetric with filePerm (0666/umask for files,
// 0777/umask for dirs).
const dirPerm os.FileMode = 0o777

// BuildLayers constructs the fallback chain as a union fs.FS, ordered highest
// priority first (first-wins like $PATH): AI_CABIN_FRAGMENTS_DIRS entries
// (conf), then the cabin-local dir (dev), then the embedded base layer.
//
// dirsVar is a comma-separated list; each entry is ~-expanded via
// cabin.ExpandHome. A missing dir entry is a strict error (no silent skip of a
// typo'd path — a misconfigured override layer should fail loudly, not
// silently drop). Returns an error if no layer is configured at all.
//
// When the var is unset, it defaults to the XDG path alone — but that default
// is applied by the caller (which resolves AI_CABIN_FRAGMENTS_DIRS like any
// profile var), not here. BuildLayers receives the resolved value.
func BuildLayers(dirsVar, cabinDir string, embedFS fs.FS) (fs.FS, error) {
	var layers []fs.FS

	for _, dir := range splitAndTrim(dirsVar) {
		dir = cabin.ExpandHome(dir)
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

// splitAndTrim splits a comma-separated list and trims whitespace, dropping
// empty entries. Returns nil for an empty string so the caller's range is a
// no-op.
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// manifest is the on-disk deps.yaml/setup.yaml file. It has two mutually
// exclusive declaration modes:
//   - mirror: copy a whole subtree rooted at <mirrorDir> into destBase/,
//     preserving the internal structure (with .tmpl stripped from filenames).
//   - entries: an explicit src→dst list, for bundles that need templated dst
//     (port-forward multi-instance) or a non-mirrored layout.
type manifest struct {
	Mirror  string          `yaml:"mirror"`
	Entries []manifestEntry `yaml:"entries"`
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
// destBase, before name templating). The destination mode is always filePerm
// (umask-applied): the source mode is not preserved (embed.FS exposes every
// file as 0444, an artifact; the executable bit is the Dockerfile's
// authority, blueprint facet).
type resolvedEntry struct {
	src string
	dst string
}

// validate checks the manifest has exactly one of mirror/entries.
func (m *manifest) validate() error {
	hasMirror := m.Mirror != ""
	hasEntries := len(m.Entries) > 0
	switch {
	case hasMirror && hasEntries:
		return errors.New("manifest has both mirror and entries (use only one)")
	case !hasMirror && !hasEntries:
		return errors.New("manifest has neither mirror nor entries")
	}
	return nil
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

// Materialize reads <bundle>/<manifestName> from merged and writes each declared
// fragment to destBase/<dst>. The facet (deps vs setup) is carried by
// manifestName ("deps.yaml" or "setup.yaml"); destBase is the destination root
// (<cabin>/.deps for deps, $AI_CABIN_HOME for setup — resolved by the caller).
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
func Materialize(merged fs.FS, bundle, manifestName, destBase string, vars map[string]string, attrs map[string]any) ([]string, error) {
	wfs, ok := merged.(walkFS)
	if !ok {
		return nil, errors.New("merged fs does not implement ReadDirFS+StatFS (required by WalkDir)")
	}

	// Bundle must exist in at least one layer.
	if _, err := fs.Stat(wfs, bundle); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("bundle %q not found in any fragment layer: %w", bundle, err)
		}
		return nil, fmt.Errorf("stat bundle %q: %w", bundle, err)
	}

	// Manifest: absent = no-op (bundle has no such facet).
	manifestPath := path.Join(bundle, manifestName)
	data, err := fs.ReadFile(wfs, manifestPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read manifest %q: %w", manifestPath, err)
	}

	var m manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %q: %w", manifestPath, err)
	}
	if err := m.validate(); err != nil {
		return nil, fmt.Errorf("manifest %q: %w", manifestPath, err)
	}

	// expand resolves modes + collects drift/per-entry issues (no fail-fast):
	// valid entries are returned alongside the aggregated error.
	entries, expandErr := m.expand(wfs, bundle, manifestName)
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
			rendered, rerr := render.RenderString(dstRel, vars, attrs)
			if rerr != nil {
				errs = append(errs, fmt.Errorf("bundle %q dst %q: %w", bundle, e.dst, rerr))
				continue
			}
			dstRel = rendered
		}

		dstPath := filepath.Join(destBase, filepath.FromSlash(dstRel))
		if err := os.MkdirAll(filepath.Dir(dstPath), dirPerm); err != nil {
			errs = append(errs, fmt.Errorf("mkdir %q: %w", filepath.Dir(dstPath), err))
			continue
		}

		// .tmpl renders (Parse streams from merged, Execute streams to the file);
		// plain files copy as-is. Either way a partial file is left on error.
		var ferr error
		if strings.HasSuffix(e.src, tmplSuffix) {
			ferr = renderFragment(wfs, srcPath, dstPath, vars, attrs)
		} else {
			ferr = copyFragment(wfs, srcPath, dstPath)
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
// dst via internal/render (Parse reads the source, Execute streams to the file
// writer — no intermediate buffer; the partial-output contract lives in
// Execute's noValueScanner). The destination is opened with filePerm so the
// umask reduces it to a writable, idempotent mode; on any error a partial file
// is left in place (by design, so the user can locate a missing var). Returns
// an error naming both src and dst.
func renderFragment(wfs walkFS, srcPath, dstPath string, vars map[string]string, attrs map[string]any) error {
	tmpl, err := render.Parse(wfs, srcPath)
	if err != nil {
		return fmt.Errorf("render %q -> %q: %w", srcPath, dstPath, err)
	}
	file, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePerm)
	if err != nil {
		return fmt.Errorf("render %q -> %q: %w", srcPath, dstPath, err)
	}
	defer file.Close()
	if err := render.Execute(tmpl, vars, attrs, file); err != nil {
		return fmt.Errorf("render %q -> %q: %w", srcPath, dstPath, err)
	}
	return nil
}

// copyFragment streams a plain (non-template) fragment from the merged FS to
// dst without buffering the whole content in memory. io.Copy uses ReaderFrom
// / WriterTo when available (sendfile for os.File-to-os.File copies via
// os.DirFS layers); for fstest.MapFS / embed.FS sources it falls back to a
// small internal buffer. The destination is opened with filePerm so the umask
// reduces it to a writable, idempotent mode; both files are closed via defer;
// a mid-copy error leaves a partial destination in place (consistent with
// renderFragment — partial files are expected and reported via the returned
// error).
func copyFragment(srcFS walkFS, srcPath, dstPath string) error {
	src, err := srcFS.Open(srcPath)
	if err != nil {
		return fmt.Errorf("copy %q -> %q: %w", srcPath, dstPath, err)
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePerm)
	if err != nil {
		return fmt.Errorf("copy %q -> %q: %w", srcPath, dstPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy %q -> %q: %w", srcPath, dstPath, err)
	}
	return nil
}
