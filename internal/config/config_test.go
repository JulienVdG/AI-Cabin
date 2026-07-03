package config

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestConfig creates a temporary config directory for tests.
func setupTestConfig(t *testing.T) (string, func()) {
	t.Helper()

	// Save original XDG_CONFIG_HOME
	originalXDG := os.Getenv("XDG_CONFIG_HOME")

	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "ai-cabin-config-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Set XDG_CONFIG_HOME to temp dir
	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Cleanup function
	cleanup := func() {
		os.Setenv("XDG_CONFIG_HOME", originalXDG)
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestGetConfigDir(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error = %v", err)
	}

	expected := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "ai-cabin")
	if dir != expected {
		t.Errorf("GetConfigDir() = %q, want %q", dir, expected)
	}
}

func TestGetProfilesDir(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	dir, err := GetProfilesDir()
	if err != nil {
		t.Fatalf("GetProfilesDir() error = %v", err)
	}

	configDir, _ := GetConfigDir()
	expected := filepath.Join(configDir, "profiles")
	if dir != expected {
		t.Errorf("GetProfilesDir() = %q, want %q", dir, expected)
	}
}

func TestLoadProfile(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	// Create profiles directory
	profilesDir, _ := GetProfilesDir()
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatalf("failed to create profiles dir: %v", err)
	}

	// Create a test profile
	profileContent := `name: test
vars:
  VAR1: value1
  VAR2: value2
`
	profilePath := filepath.Join(profilesDir, "test.yaml")
	if err := os.WriteFile(profilePath, []byte(profileContent), 0644); err != nil {
		t.Fatalf("failed to write test profile: %v", err)
	}

	// Load the profile
	profile, err := LoadProfile("test")
	if err != nil {
		t.Fatalf("LoadProfile() error = %v", err)
	}

	if profile.Name != "test" {
		t.Errorf("LoadProfile() Name = %q, want %q", profile.Name, "test")
	}

	if profile.Vars["VAR1"] != "value1" {
		t.Errorf("LoadProfile() VAR1 = %q, want %q", profile.Vars["VAR1"], "value1")
	}

	if profile.Vars["VAR2"] != "value2" {
		t.Errorf("LoadProfile() VAR2 = %q, want %q", profile.Vars["VAR2"], "value2")
	}
}

func TestLoadProfileNotFound(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	_, err := LoadProfile("nonexistent")
	if err == nil {
		t.Error("LoadProfile() expected error for nonexistent profile, got nil")
	}
}

func TestListProfiles(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	// Create profiles directory
	profilesDir, _ := GetProfilesDir()
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatalf("failed to create profiles dir: %v", err)
	}

	// Create test profiles
	profiles := []string{"perso", "pro", "test"}
	for _, name := range profiles {
		content := "name: " + name
		path := filepath.Join(profilesDir, name+".yaml")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write profile %q: %v", name, err)
		}
	}

	// List profiles
	list, err := ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}

	if len(list) != len(profiles) {
		t.Errorf("ListProfiles() count = %d, want %d", len(list), len(profiles))
	}

	// Check all profiles are present
	profileMap := make(map[string]bool)
	for _, p := range list {
		profileMap[p] = true
	}
	for _, p := range profiles {
		if !profileMap[p] {
			t.Errorf("ListProfiles() missing profile %q", p)
		}
	}
}

func TestListProfilesEmpty(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	list, err := ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}

	if len(list) != 0 {
		t.Errorf("ListProfiles() = %v, want empty list", list)
	}
}

func TestGetCurrentProfile(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	// No config file yet
	profile, err := GetCurrentProfile()
	if err != nil {
		t.Fatalf("GetCurrentProfile() error = %v", err)
	}
	if profile != "" {
		t.Errorf("GetCurrentProfile() = %q, want empty string", profile)
	}

	// Create config file
	configDir, _ := GetConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `currentProfile: perso
`
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	profile, err = GetCurrentProfile()
	if err != nil {
		t.Fatalf("GetCurrentProfile() error = %v", err)
	}
	if profile != "perso" {
		t.Errorf("GetCurrentProfile() = %q, want %q", profile, "perso")
	}
}

func TestSetCurrentProfile(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	err := SetCurrentProfile("perso")
	if err != nil {
		t.Fatalf("SetCurrentProfile() error = %v", err)
	}

	profile, err := GetCurrentProfile()
	if err != nil {
		t.Fatalf("GetCurrentProfile() error = %v", err)
	}
	if profile != "perso" {
		t.Errorf("GetCurrentProfile() after SetCurrentProfile() = %q, want %q", profile, "perso")
	}
}
