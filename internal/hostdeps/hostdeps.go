// Package hostdeps stages the host-side runtime artifacts a cabin image
// consumes (the greywall sandbox binary, the greyproxy CA certificate)
// into <XDG state>/ai-cabin/host-deps/, the stable location the lifecycle
// `deps` task copies from into .deps/. Staging happens only at
// `cabin setup`: the stage is a hand-editable cache — a patched greywall
// dropped in a slot survives builds, and the next setup that resolves the
// slot re-stages it (slots left out of the resolved arches persist).
package hostdeps

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/JulienVdG/AI-Cabin/internal/config"
)

// GreywallArchesVar lists the arch slots setup ensures (CSV, e.g.
// "arm64,amd64" on a dual-arch Linux host). Defaults to the host arch,
// plus amd64 on an arm64 macOS host.
const GreywallArchesVar = "AI_CABIN_GREYWALL_ARCHES"

// GreyproxyCAVar stages an explicit greyproxy CA certificate instead of
// the greyproxy state path.
const GreyproxyCAVar = "AI_CABIN_GREYPROXY_CA"

// greywallVersion pins the greywall release the download fallback fetches.
const greywallVersion = "v0.3.7"

// dirName is the state subdirectory holding the staged artifacts, mirrored
// by the HOST_DEPS var of the lifecycle Taskfile.
const dirName = "host-deps"

// greywallName and certName are the staged file names, matching the .deps/
// destinations the `deps` task copies to and install.d consumes. The
// greywall binary is staged per arch slot: host-deps/<arch>/greywall.
const (
	greywallName = "greywall"
	certName     = "greyproxy-ca.crt"
)

// elfMachines maps the ELF machine types of the supported architectures to
// their GOARCH slot names.
var elfMachines = map[uint16]string{
	0x3E: "amd64", // EM_X86_64
	0xB7: "arm64", // EM_AARCH64
}

// Artifact reports the staging outcome of one host artifact.
type Artifact struct {
	// Name is the staged file name (greywall, greyproxy-ca.crt).
	Name string
	// Arch is the arch slot a greywall artifact landed in; empty for the
	// arch-independent certificate.
	Arch string
	// Source describes the provenance of a staged artifact (a path or URL).
	Source string
	// Staged reports whether the artifact was written to the state dir.
	Staged bool
	// Warning explains an unstaged artifact and how to fix it.
	Warning string
}

// Stage resolves and stages the host artifacts under <state>/host-deps/:
// the greywall binary per arch slot (host-deps/<arch>/), the CA certificate
// at the top level. Every GreywallArchesVar arch (the host arch by default,
// plus amd64 on an arm64 macOS host) is ensured from a host binary with a
// matching machine type, else from the pinned release download — a macOS
// host greywall is a Mach-O the linux image cannot run, hence the download.
// The CA certificate comes from GreyproxyCAVar, else the greyproxy state
// path, and is never downloaded. Returns the staged artifacts in a stable
// order (greywall slots sorted by arch, then the certificate). An error
// means at least one requested slot could not be staged; its message
// carries the alternatives.
func Stage(vars config.Vars) ([]Artifact, error) {
	stateDir, err := config.GetStateDir()
	if err != nil {
		return nil, err
	}
	s := stager{vars: vars, dir: filepath.Join(stateDir, dirName), releaseURL: defaultReleaseURL}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", s.dir, err)
	}
	return s.stage()
}

// stager is the staging engine: the resolved vars plus the state host-deps
// dir its methods write to. releaseURL builds the greywall release tarball
// URL of an arch slot for the download fallback.
type stager struct {
	vars       config.Vars
	dir        string
	releaseURL func(arch string) string
}

// stage runs every staging step and returns the artifacts in a stable
// order: greywall slots sorted by arch, then the certificate.
func (s stager) stage() ([]Artifact, error) {
	arts, gwErr := s.greywall()
	cert, certErr := s.cert()

	slices.SortFunc(arts, func(a, b Artifact) int { return strings.Compare(a.Arch, b.Arch) })
	arts = append(arts, cert)

	if err := errors.Join(gwErr, certErr); err != nil {
		return arts, err
	}
	return arts, nil
}

// greywall stages one artifact per resolved arch slot.
func (s stager) greywall() ([]Artifact, error) {
	arches, err := parseArches(s.vars[GreywallArchesVar])
	if err != nil {
		return nil, err
	}
	var arts []Artifact
	var errs error
	for _, arch := range arches {
		art, err := s.ensureSlot(arch)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		arts = append(arts, art)
	}
	return arts, errs
}

// ensureSlot stages an arch slot from the host or the pinned release
// download.
func (s stager) ensureSlot(arch string) (Artifact, error) {
	if cand, ok := hostCandidate(arch); ok {
		if err := s.slot(arch).stageFrom(cand); err != nil {
			return Artifact{}, fmt.Errorf("stage greywall (%s) from %s: %w", arch, cand, err)
		}
		return Artifact{Name: greywallName, Arch: arch, Source: cand, Staged: true}, nil
	}
	url := s.releaseURL(arch)
	b, err := fetchBin(url)
	if err != nil {
		return Artifact{}, fmt.Errorf("download greywall (%s): %w; place a linux binary at %q and re-run", arch, err, s.slot(arch).path())
	}
	if b.arch != arch {
		return Artifact{}, fmt.Errorf("greywall release for %s carries a %s binary", arch, b.arch)
	}
	if err := s.slot(arch).write(b.data); err != nil {
		return Artifact{}, fmt.Errorf("stage greywall (%s): %w", arch, err)
	}
	return Artifact{Name: greywallName, Arch: arch, Source: url, Staged: true}, nil
}

// cert stages dir/greyproxy-ca.crt from the resolved var or the greyproxy
// state path. An absent source removes any previously staged certificate
// and is a warning, not an error: greyproxy may be installed later, and the
// `deps` precondition gates the build with the same guidance.
func (s stager) cert() (Artifact, error) {
	dst := filepath.Join(s.dir, certName)

	if src := s.vars[GreyproxyCAVar]; src != "" {
		if err := copyFile(src, dst, 0o644); err != nil {
			return Artifact{}, fmt.Errorf("stage greyproxy CA from %s: %w", src, err)
		}
		return Artifact{Name: certName, Source: src, Staged: true}, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Artifact{}, err
	}
	src := filepath.Join(home, ".local", "share", "greyproxy", "ca-cert.pem")
	if err := copyFile(src, dst, 0o644); err != nil {
		if os.IsNotExist(err) {
			_ = os.Remove(dst)
			return Artifact{Name: certName, Warning: fmt.Sprintf("greyproxy CA not found at %s: install greyproxy host-side or set %s, then re-run `cabin setup`", src, GreyproxyCAVar)}, nil
		}
		return Artifact{}, fmt.Errorf("stage greyproxy CA: %w", err)
	}
	return Artifact{Name: certName, Source: src, Staged: true}, nil
}

// slot returns the arch slot receiver of the greywall stage.
func (s stager) slot(arch string) slot {
	return slot{dir: s.dir, arch: arch}
}

// slot is one arch slot of the greywall stage: <dir>/<arch>/greywall.
type slot struct {
	dir  string
	arch string
}

// path returns the staged greywall path of the slot.
func (sl slot) path() string {
	return filepath.Join(sl.dir, sl.arch, greywallName)
}

// write stores greywall binary data in the slot, creating its directory.
func (sl slot) write(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(sl.path()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(sl.path(), data, 0o755)
}

// stageFrom copies the binary at src into the slot.
func (sl slot) stageFrom(src string) error {
	return copyFile(src, sl.path(), 0o755)
}

// bin is a greywall linux binary with the arch slot its ELF machine type
// selects.
type bin struct {
	arch string
	data []byte
}

// readBin reads a greywall binary from a file path.
func readBin(path string) (bin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return bin{}, err
	}
	arch, err := elfArch(data)
	if err != nil {
		return bin{}, err
	}
	return bin{arch: arch, data: data}, nil
}

// fetchBin downloads a greywall release tarball and returns the binary
// it holds.
func fetchBin(url string) (bin, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return bin{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return bin{}, fmt.Errorf("unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return bin{}, err
	}
	data, err := extractTarMember(body, greywallName)
	if err != nil {
		return bin{}, err
	}
	arch, err := elfArch(data)
	if err != nil {
		return bin{}, fmt.Errorf("extracted greywall: %w", err)
	}
	return bin{arch: arch, data: data}, nil
}

// elfArch reads the header of a linux binary and returns the GOARCH slot
// name its ELF machine type selects (amd64, arm64). Errors on a non-ELF
// or unsupported binary.
func elfArch(head []byte) (string, error) {
	if len(head) < 20 || !bytes.Equal(head[:4], []byte{0x7f, 'E', 'L', 'F'}) {
		return "", fmt.Errorf("not a linux binary (ELF magic missing)")
	}
	machine := binary.LittleEndian.Uint16(head[18:20])
	arch, ok := elfMachines[machine]
	if !ok {
		return "", fmt.Errorf("unsupported ELF machine type %d: want amd64 or arm64", machine)
	}
	return arch, nil
}

// extractTarMember returns the content of the regular member named name
// from gzipped tarball data.
func extractTarMember(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("tarball has no %q member", name)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == name && hdr.Typeflag == tar.TypeReg {
			return io.ReadAll(tr)
		}
	}
}

// hostCandidate returns the path of a host greywall whose ELF machine type
// matches arch.
func hostCandidate(arch string) (string, bool) {
	for _, cand := range greywallCandidates() {
		if b, err := readBin(cand); err == nil && b.arch == arch {
			return cand, true
		}
	}
	return "", false
}

// greywallCandidates returns the host locations probed for a linux greywall:
// $PATH first (distro and brew installs), then the greywall-documented
// ~/.local/bin.
func greywallCandidates() []string {
	var candidates []string
	if p, err := exec.LookPath("greywall"); err == nil {
		candidates = append(candidates, p)
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".local", "bin", "greywall"))
	}
	return candidates
}

// parseArches parses the GreywallArchesVar CSV (empty → the default arches),
// trimming, deduplicating and rejecting names no ELF machine maps to.
func parseArches(csv string) ([]string, error) {
	if csv == "" {
		arches := defaultArches(runtime.GOOS, runtime.GOARCH)
		for _, a := range arches {
			if !supportedArch(a) {
				return nil, fmt.Errorf("host arch %q has no supported greywall slot (amd64, arm64)", a)
			}
		}
		return arches, nil
	}
	var out []string
	seen := map[string]bool{}
	for _, a := range strings.Split(csv, ",") {
		a = strings.TrimSpace(a)
		if !supportedArch(a) {
			return nil, fmt.Errorf("unsupported arch %q in %s: want amd64 or arm64", a, GreywallArchesVar)
		}
		if !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	return out, nil
}

// defaultArches returns the arch slots ensured when GreywallArchesVar is
// unset: the host arch alone, plus amd64 on an arm64 macOS host — Rosetta
// runs amd64 cabins there, so both slots are the zero-config default.
func defaultArches(goos, goarch string) []string {
	arches := []string{goarch}
	if goos == "darwin" && goarch == "arm64" {
		arches = append(arches, "amd64")
	}
	return arches
}

// supportedArch reports whether an arch name maps to an ELF machine type.
func supportedArch(arch string) bool {
	for _, a := range elfMachines {
		if a == arch {
			return true
		}
	}
	return false
}

// defaultReleaseURL builds the pinned greywall release tarball URL for arch.
func defaultReleaseURL(arch string) string {
	ver := strings.TrimPrefix(greywallVersion, "v")
	return fmt.Sprintf("https://github.com/GreyhavenHQ/greywall/releases/download/%s/greywall_%s_Linux_%s.tar.gz", greywallVersion, ver, releaseArch(arch))
}

// releaseArch maps a GOARCH slot name to the goreleaser arch spelling of
// the greywall release assets (x86_64 for the amd64 slot).
func releaseArch(arch string) string {
	if arch == "amd64" {
		return "x86_64"
	}
	return arch
}

// copyFile copies src to dst with mode, creating dst parent dirs.
func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}
