package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/config"
)

// setupTestConfig creates a temporary directory for testing and sets XDG_CONFIG_HOME.
// It returns the temp directory path and a cleanup function.
func setupTestConfig(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
}

func TestGetConfigDir(t *testing.T) {
	t.Run("uses_default_XDG_CONFIG_HOME_when_not_set", func(t *testing.T) {
		setupTestConfig(t)
		// Clear XDG_CONFIG_HOME to test default behavior
		t.Setenv("XDG_CONFIG_HOME", "")

		home, _ := os.UserHomeDir()
		dir, err := config.GetConfigDir()
		if err != nil {
			t.Fatalf("GetConfigDir() error = %v", err)
		}
		expected := filepath.Join(home, ".config", "ai-cabin")
		if dir != expected {
			t.Errorf("GetConfigDir() = %q, want %q", dir, expected)
		}
	})

	t.Run("uses_custom_XDG_CONFIG_HOME_when_set", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tmpDir)

		dir, err := config.GetConfigDir()
		if err != nil {
			t.Fatalf("GetConfigDir() error = %v", err)
		}
		expected := filepath.Join(tmpDir, "ai-cabin")
		if dir != expected {
			t.Errorf("GetConfigDir() = %q, want %q", dir, expected)
		}
	})
}

func TestGetStateDir(t *testing.T) {
	t.Run("uses_default_XDG_STATE_HOME_when_not_set", func(t *testing.T) {
		setupTestConfig(t)
		t.Setenv("XDG_STATE_HOME", "")

		home, _ := os.UserHomeDir()
		dir, err := config.GetStateDir()
		if err != nil {
			t.Fatalf("GetStateDir() error = %v", err)
		}
		expected := filepath.Join(home, ".local", "state", "ai-cabin")
		if dir != expected {
			t.Errorf("GetStateDir() = %q, want %q", dir, expected)
		}
	})

	t.Run("uses_custom_XDG_STATE_HOME_when_set", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_STATE_HOME", tmpDir)

		dir, err := config.GetStateDir()
		if err != nil {
			t.Fatalf("GetStateDir() error = %v", err)
		}
		expected := filepath.Join(tmpDir, "ai-cabin")
		if dir != expected {
			t.Errorf("GetStateDir() = %q, want %q", dir, expected)
		}
	})
}

func TestGetProfilesDir(t *testing.T) {
	setupTestConfig(t)

	dir, err := config.GetProfilesDir()
	if err != nil {
		t.Fatalf("GetProfilesDir() error = %v", err)
	}
	if !strings.HasSuffix(dir, "ai-cabin/profiles") {
		t.Errorf("GetProfilesDir() = %q, should end with 'ai-cabin/profiles'", dir)
	}
}

func TestRealHomeDir_GetHomeDir(t *testing.T) {
	t.Run("returns_user_home_directory", func(t *testing.T) {
		h := &config.RealHomeDir{}

		home, err := h.GetHomeDir()
		if err != nil {
			t.Fatalf("RealHomeDir.GetHomeDir() error = %v", err)
		}
		if home == "" {
			t.Error("RealHomeDir.GetHomeDir() returned empty string")
		}

		// Verify it matches os.UserHomeDir()
		expected, _ := os.UserHomeDir()
		if home != expected {
			t.Errorf("RealHomeDir.GetHomeDir() = %q, want %q", home, expected)
		}
	})
}
