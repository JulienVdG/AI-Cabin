// Package skeletons copies a Class 1 skeleton (a flat directory tree) to a
// destination, rendering .tmpl files via internal/render. It shares the
// writestrategy.FileCreator policy with internal/fragments so the overwrite
// behaviour (no-overwrite default, --force, future interactive) is selected at
// the call site, not baked into the engine.
//
// Unlike fragments, a skeleton has no manifest (no deps.yaml/setup.yaml): it
// is a plain tree to copy, with .tmpl files rendered and the suffix stripped
// from the destination name. A desk skeleton (copied to AI_CABIN_DESK) and a
// project skeleton (copied to ~/projects/<name>) both use this engine; the
// concern (desk vs project) drives the destination, not the copy mechanism.
package skeletons

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/JulienVdG/AI-Cabin/internal/render"
	"github.com/JulienVdG/AI-Cabin/internal/writestrategy"
)

// walkFS is the contract Apply needs from the source tree: fs.FS for Open
// and fs.ReadDirFS for ReadDir. fs.WalkDir only requires these two in
// practice (it derives DirEntry info from ReadDir; the fs.StatFS method is
// only consulted on a ReadDir error path that an embed.FS does not hit), so
// a plain fs.Sub(embed.FS, ...) satisfies the contract — unlike
// internal/fragments, which declares fs.StatFS too because it always wraps its
// FS in internal/unionfs (a layer that implements all three). Making the
// contract explicit avoids a hidden runtime assertion: a caller passing a
// plain fs.FS gets a clear error instead of a WalkDir panic.
type walkFS interface {
	fs.FS
	fs.ReadDirFS
}

// tmplSuffix marks a file whose content is rendered via internal/render. The
// suffix is stripped from the destination filename (Ansible .j2 convention,
// same as internal/fragments).
const tmplSuffix = ".tmpl"

// Apply copies the source tree rooted at srcRoot in srcFS to dest, rendering
// .tmpl files via internal/render (vars namespaced as {{.Vars.X}}, no attrs —
// a Class 1 skeleton has no per-instance attributes) and copying the rest
// as-is. The destination name of a .tmpl file has the suffix stripped.
//
// The write policy (no-overwrite default, --force overwrite, future
// interactive) is carried by creator: SkipCreator skips an existing
// destination (returns writestrategy.ErrSkip, non-fatal — the file is excluded
// from the written list and the walk continues); TruncateCreator overwrites.
// Other errors (read, render, I/O) are collected via errors.Join (no fail-fast,
// mirroring internal/fragments) so the user sees every issue in one run;
// partial files are left on disk for discoverability.
//
// Returns the list of successfully written relpaths (relative to dest) and an
// optional aggregated error. It is business-logic-only: the CLI formats
// progress and the error for the user.
func Apply(srcFS fs.FS, srcRoot, dest string, vars map[string]string, creator writestrategy.FileCreator) ([]string, error) {
	wfs, ok := srcFS.(walkFS)
	if !ok {
		return nil, errors.New("source fs does not implement ReadDirFS+StatFS (required by WalkDir)")
	}

	var written []string
	var errs []error
	walkErr := fs.WalkDir(wfs, srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// rel is the destination path relative to srcRoot, forward-slash.
		rel := pathRel(srcRoot, p)
		dstRel := stripTmplSuffix(rel)
		dstPath := filepath.Join(dest, filepath.FromSlash(dstRel))

		if err := os.MkdirAll(filepath.Dir(dstPath), writestrategy.DirPerm); err != nil {
			errs = append(errs, fmt.Errorf("mkdir %q: %w", filepath.Dir(dstPath), err))
			return nil
		}

		if ferr := copyOrRender(wfs, p, dstPath, vars, creator); ferr != nil {
			if errors.Is(ferr, writestrategy.ErrSkip) {
				return nil // Skipped: non-fatal, not added to written.
			}
			errs = append(errs, ferr)
			return nil
		}
		written = append(written, dstRel)
		return nil
	})
	if walkErr != nil {
		errs = append(errs, fmt.Errorf("walk %q: %w", srcRoot, walkErr))
	}
	if len(errs) > 0 {
		return written, errors.Join(errs...)
	}
	return written, nil
}

// copyOrRender renders a .tmpl file from the source FS to dst, or copies a
// plain file as-is. The write policy is carried by creator. On
// writestrategy.ErrSkip the caller treats the file as not written (non-fatal).
func copyOrRender(wfs walkFS, srcPath, dstPath string, vars map[string]string, creator writestrategy.FileCreator) error {
	if strings.HasSuffix(srcPath, tmplSuffix) {
		return renderFile(wfs, srcPath, dstPath, vars, creator)
	}
	return copyFile(wfs, srcPath, dstPath, creator)
}

// renderFile parses a .tmpl from the source FS and renders it to dst via
// internal/render (vars namespaced as {{.Vars.X}}, no attrs). The destination
// is opened via creator. On any error a partial file is left in place.
func renderFile(wfs walkFS, srcPath, dstPath string, vars map[string]string, creator writestrategy.FileCreator) error {
	tmpl, err := render.Parse(wfs, srcPath)
	if err != nil {
		return fmt.Errorf("render %q -> %q: %w", srcPath, dstPath, err)
	}
	w, err := creator.Create(dstPath)
	if err != nil {
		return fmt.Errorf("render %q -> %q: %w", srcPath, dstPath, err)
	}
	defer w.Close()
	if err := render.Execute(tmpl, vars, nil, w); err != nil {
		return fmt.Errorf("render %q -> %q: %w", srcPath, dstPath, err)
	}
	return nil
}

// copyFile streams a plain (non-template) file from the source FS to dst via
// io.Copy. The destination is opened via creator.
func copyFile(wfs walkFS, srcPath, dstPath string, creator writestrategy.FileCreator) error {
	src, err := wfs.Open(srcPath)
	if err != nil {
		return fmt.Errorf("copy %q -> %q: %w", srcPath, dstPath, err)
	}
	defer src.Close()
	dst, err := creator.Create(dstPath)
	if err != nil {
		return fmt.Errorf("copy %q -> %q: %w", srcPath, dstPath, err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy %q -> %q: %w", srcPath, dstPath, err)
	}
	return nil
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
