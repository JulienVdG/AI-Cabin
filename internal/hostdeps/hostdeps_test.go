package hostdeps_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/JulienVdG/AI-Cabin/internal/config"
	"github.com/JulienVdG/AI-Cabin/internal/hostdeps"
)

// newStageEnv isolates the environment Stage reads: a temp HOME (candidate
// paths), a temp XDG_STATE_HOME (stage destination) and an empty PATH dir so
// no host greywall resolves unless the case installs one.
func newStageEnv(t *testing.T) (home, xdg string) {
	t.Helper()
	home = t.TempDir()
	xdg = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", xdg)
	t.Setenv("PATH", t.TempDir())
	return home, xdg
}

// greywallPath returns the staged greywall slot path for the isolated env.
func greywallPath(xdg, arch string) string {
	return filepath.Join(xdg, "ai-cabin", "host-deps", arch, "greywall")
}

// certPath returns the staged certificate path for the isolated env.
func certPath(xdg string) string {
	return filepath.Join(xdg, "ai-cabin", "host-deps", "greyproxy-ca.crt")
}

// fakeELF builds a fake linux binary carrying the ELF machine type of arch.
func fakeELF(arch, payload string) []byte {
	machine := map[string]uint16{"amd64": 0x3E, "arm64": 0xB7}[arch]
	b := make([]byte, 20+len(payload))
	copy(b, []byte{0x7f, 'E', 'L', 'F'})
	binary.LittleEndian.PutUint16(b[18:20], machine)
	copy(b[20:], payload)
	return b
}

// writeFile creates path with content, making parent dirs. The mode matters
// for PATH candidates: exec.LookPath requires the executable bit.
func writeFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, content, mode))
}

func TestStage(t *testing.T) {
	nonElfBin := []byte("mach-o-greywall-host-binary")

	t.Run("HostBinaryOnPath", func(t *testing.T) {
		_, xdg := newStageEnv(t)
		binDir := t.TempDir()
		t.Setenv("PATH", binDir)
		onPath := filepath.Join(binDir, "greywall")
		writeFile(t, onPath, fakeELF("amd64", "host"), 0o755)

		arts, err := hostdeps.Stage(config.Vars{
			hostdeps.GreywallArchesVar: "amd64",
		})
		require.NoError(t, err)

		require.True(t, arts[0].Staged)
		require.Equal(t, "amd64", arts[0].Arch)
		require.Equal(t, onPath, arts[0].Source)
		got, err := os.ReadFile(greywallPath(xdg, "amd64"))
		require.NoError(t, err)
		require.Equal(t, fakeELF("amd64", "host"), got)
	})

	t.Run("HostBinaryInLocalBin", func(t *testing.T) {
		home, _ := newStageEnv(t)
		local := filepath.Join(home, ".local", "bin", "greywall")
		writeFile(t, local, fakeELF("arm64", "host"), 0o755)

		arts, err := hostdeps.Stage(config.Vars{
			hostdeps.GreywallArchesVar: "arm64",
		})
		require.NoError(t, err)

		require.True(t, arts[0].Staged)
		require.Equal(t, "arm64", arts[0].Arch)
		require.Equal(t, local, arts[0].Source)
	})

	t.Run("ArchesVarEnsuresBothSlots", func(t *testing.T) {
		home, xdg := newStageEnv(t)
		binDir := t.TempDir()
		t.Setenv("PATH", binDir)
		onPath := filepath.Join(binDir, "greywall")
		local := filepath.Join(home, ".local", "bin", "greywall")
		writeFile(t, onPath, fakeELF("amd64", "from-path"), 0o755)
		writeFile(t, local, fakeELF("arm64", "from-local"), 0o755)

		arts, err := hostdeps.Stage(config.Vars{
			hostdeps.GreywallArchesVar: "amd64,arm64",
		})
		require.NoError(t, err)

		require.Len(t, arts, 3)
		require.Equal(t, "amd64", arts[0].Arch)
		require.Equal(t, onPath, arts[0].Source)
		require.Equal(t, "arm64", arts[1].Arch)
		require.Equal(t, local, arts[1].Source)
		for arch, want := range map[string][]byte{
			"amd64": fakeELF("amd64", "from-path"),
			"arm64": fakeELF("arm64", "from-local"),
		} {
			got, err := os.ReadFile(greywallPath(xdg, arch))
			require.NoError(t, err)
			require.Equal(t, want, got)
		}
	})

	t.Run("WrongArchCandidateFallsThrough", func(t *testing.T) {
		home, xdg := newStageEnv(t)
		binDir := t.TempDir()
		t.Setenv("PATH", binDir)
		writeFile(t, filepath.Join(binDir, "greywall"), fakeELF("arm64", "wrong-slot"), 0o755)
		local := filepath.Join(home, ".local", "bin", "greywall")
		writeFile(t, local, fakeELF("amd64", "right-slot"), 0o755)

		arts, err := hostdeps.Stage(config.Vars{
			hostdeps.GreywallArchesVar: "amd64",
		})
		require.NoError(t, err)

		require.Equal(t, local, arts[0].Source)
		got, err := os.ReadFile(greywallPath(xdg, "amd64"))
		require.NoError(t, err)
		require.Equal(t, fakeELF("amd64", "right-slot"), got)
	})

	t.Run("NonElfCandidateFallsThrough", func(t *testing.T) {
		home, _ := newStageEnv(t)
		binDir := t.TempDir()
		t.Setenv("PATH", binDir)
		writeFile(t, filepath.Join(binDir, "greywall"), nonElfBin, 0o755)
		local := filepath.Join(home, ".local", "bin", "greywall")
		writeFile(t, local, fakeELF("amd64", "right-slot"), 0o755)

		arts, err := hostdeps.Stage(config.Vars{
			hostdeps.GreywallArchesVar: "amd64",
		})
		require.NoError(t, err)

		require.Equal(t, local, arts[0].Source)
	})

	t.Run("ArchesVarInvalidArchErrors", func(t *testing.T) {
		_, _ = newStageEnv(t)

		_, err := hostdeps.Stage(config.Vars{
			hostdeps.GreywallArchesVar: "riscv64",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported arch")
	})

	t.Run("CertFromDefaultPath", func(t *testing.T) {
		home, xdg := newStageEnv(t)
		def := filepath.Join(home, ".local", "share", "greyproxy", "ca-cert.pem")
		writeFile(t, def, []byte("CA-PEM"), 0o644)
		binDir := t.TempDir()
		t.Setenv("PATH", binDir)
		writeFile(t, filepath.Join(binDir, "greywall"), fakeELF("amd64", "bin"), 0o755)

		arts, err := hostdeps.Stage(config.Vars{
			hostdeps.GreywallArchesVar: "amd64",
		})
		require.NoError(t, err)

		require.True(t, arts[1].Staged)
		require.Equal(t, def, arts[1].Source)
		got, err := os.ReadFile(certPath(xdg))
		require.NoError(t, err)
		require.Equal(t, "CA-PEM", string(got))
	})

	t.Run("CertAbsentWarns", func(t *testing.T) {
		_, xdg := newStageEnv(t)
		binDir := t.TempDir()
		t.Setenv("PATH", binDir)
		writeFile(t, filepath.Join(binDir, "greywall"), fakeELF("amd64", "bin"), 0o755)

		arts, err := hostdeps.Stage(config.Vars{
			hostdeps.GreywallArchesVar: "amd64",
		})
		require.NoError(t, err)

		require.False(t, arts[1].Staged)
		require.Contains(t, arts[1].Warning, hostdeps.GreyproxyCAVar)
		_, serr := os.Stat(certPath(xdg))
		require.True(t, os.IsNotExist(serr), "no cert file should be staged")
	})

	t.Run("CertOverrideMissingFileErrors", func(t *testing.T) {
		_, _ = newStageEnv(t)
		binDir := t.TempDir()
		t.Setenv("PATH", binDir)
		writeFile(t, filepath.Join(binDir, "greywall"), fakeELF("amd64", "bin"), 0o755)

		_, err := hostdeps.Stage(config.Vars{
			hostdeps.GreywallArchesVar: "amd64",
			hostdeps.GreyproxyCAVar:    filepath.Join(t.TempDir(), "missing-ca.pem"),
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "greyproxy CA")
	})
}
