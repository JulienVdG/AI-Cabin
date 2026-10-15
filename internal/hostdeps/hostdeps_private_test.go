package hostdeps

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeELF builds a minimal little-endian ELF header selecting arch, with
// payload as file content.
func fakeELF(arch, payload string) []byte {
	machine := map[string]uint16{"amd64": 0x3E, "arm64": 0xB7}[arch]
	b := []byte{0x7f, 'E', 'L', 'F', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint16(b[18:20], machine)
	return append(b, payload...)
}

// serveTarGz serves a tarball holding one greywall member and returns its
// URL.
func serveTarGz(t *testing.T, bin []byte) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: greywallName, Mode: 0o755, Size: int64(len(bin)), Typeflag: tar.TypeReg}))
	_, err := tw.Write(bin)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(buf.Bytes())
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/greywall.tar.gz"
}

// isolateHost empties the host binary lookup paths so ensureSlot falls
// through to the download.
func isolateHost(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
}

func TestGreyproxyCAPath(t *testing.T) {
	t.Run("DarwinUsesApplicationSupport", func(t *testing.T) {
		require.Equal(t,
			filepath.Join("home", "Library", "Application Support", "greyproxy", "ca-cert.pem"),
			greyproxyCAPath("darwin", "home"))
	})

	t.Run("LinuxUsesXdgData", func(t *testing.T) {
		require.Equal(t,
			filepath.Join("home", ".local", "share", "greyproxy", "ca-cert.pem"),
			greyproxyCAPath("linux", "home"))
	})
}

func TestEnsureSlot(t *testing.T) {
	t.Run("DownloadStagesSlot", func(t *testing.T) {
		isolateHost(t)
		url := serveTarGz(t, fakeELF("amd64", "from-release"))
		s := stager{dir: t.TempDir(), releaseURL: func(string) string { return url }}

		art, err := s.ensureSlot("amd64")
		require.NoError(t, err)

		require.True(t, art.Staged)
		require.Equal(t, "amd64", art.Arch)
		require.Equal(t, url, art.Source)
		got, err := os.ReadFile(filepath.Join(s.dir, "amd64", greywallName))
		require.NoError(t, err)
		require.Equal(t, fakeELF("amd64", "from-release"), got)
	})

	t.Run("DownloadArchMismatchErrors", func(t *testing.T) {
		isolateHost(t)
		url := serveTarGz(t, fakeELF("arm64", "from-release"))
		s := stager{dir: t.TempDir(), releaseURL: func(string) string { return url }}

		_, err := s.ensureSlot("amd64")
		require.Error(t, err)
		require.Contains(t, err.Error(), "carries a arm64 binary")
	})

	t.Run("DownloadFailureErrors", func(t *testing.T) {
		isolateHost(t)
		srv := httptest.NewServer(http.NotFoundHandler())
		t.Cleanup(srv.Close)
		s := stager{dir: t.TempDir(), releaseURL: func(string) string { return srv.URL }}

		_, err := s.ensureSlot("amd64")
		require.Error(t, err)
		require.Contains(t, err.Error(), "download greywall")
		require.Contains(t, err.Error(), "place a linux binary at")
	})
}

func TestDefaultReleaseURL(t *testing.T) {
	t.Run("PinsTheRelease", func(t *testing.T) {
		require.Equal(t,
			"https://github.com/GreyhavenHQ/greywall/releases/download/v0.3.7/greywall_0.3.7_Linux_x86_64.tar.gz",
			defaultReleaseURL("amd64"))
		require.Equal(t,
			"https://github.com/GreyhavenHQ/greywall/releases/download/v0.3.7/greywall_0.3.7_Linux_arm64.tar.gz",
			defaultReleaseURL("arm64"))
	})

	t.Run("UsesTheHostArch", func(t *testing.T) {
		require.Contains(t,
			defaultReleaseURL(runtime.GOARCH),
			fmt.Sprintf("greywall_0.3.7_Linux_%s.tar.gz", releaseArch(runtime.GOARCH)))
	})
}

func TestDefaultArches(t *testing.T) {
	t.Run("DarwinArm64StagesBoth", func(t *testing.T) {
		require.Equal(t, []string{"arm64", "amd64"}, defaultArches("darwin", "arm64"))
	})

	t.Run("DarwinAmd64StagesHostAlone", func(t *testing.T) {
		require.Equal(t, []string{"amd64"}, defaultArches("darwin", "amd64"))
	})

	t.Run("LinuxStagesHostAlone", func(t *testing.T) {
		require.Equal(t, []string{"amd64"}, defaultArches("linux", "amd64"))
		require.Equal(t, []string{"arm64"}, defaultArches("linux", "arm64"))
	})
}

func TestParseArches(t *testing.T) {
	t.Run("EmptyUsesDefaultArches", func(t *testing.T) {
		arches, err := parseArches("")
		require.NoError(t, err)
		require.Equal(t, defaultArches(runtime.GOOS, runtime.GOARCH), arches)
	})

	t.Run("TrimsAndDedups", func(t *testing.T) {
		arches, err := parseArches(" amd64,arm64 , amd64")
		require.NoError(t, err)
		require.Equal(t, []string{"amd64", "arm64"}, arches)
	})

	t.Run("UnsupportedArchErrors", func(t *testing.T) {
		_, err := parseArches("amd64,riscv64")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported arch")
	})
}
