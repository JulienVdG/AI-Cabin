package config_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/mock"

	"github.com/JulienVdG/AI-Cabin/internal/config"
	mock_config "github.com/JulienVdG/AI-Cabin/internal/mocks/config"
	mock_fs "github.com/JulienVdG/AI-Cabin/internal/mocks/io/fs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestConfig creates a temporary directory for testing and sets XDG_CONFIG_HOME.
// It returns the temp directory path and a cleanup function.
func setupTestConfig(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
}

// newTestService creates a ConfigService backed by a temp config dir (os.DirFS).
// It sets XDG_CONFIG_HOME to an empty temp dir so the service reads/writes there.
// Tests create profile/config files on real disk; the service reads them via fs.FS.
func newTestService(t *testing.T) *config.ConfigService {
	t.Helper()
	setupTestConfig(t)
	configDir, err := config.GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error = %v", err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	return config.NewConfigService(nil, nil, os.DirFS(configDir), config.AtomicFileWriter{})
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

func TestLoadProfile(t *testing.T) {
	t.Run("loads existing profile", func(t *testing.T) {
		svc := newTestService(t)

		// Create a test profile
		profilesDir, _ := config.GetProfilesDir()
		profileContent := `name: perso
vars:
  TEST_VAR: test-value
`
		profilePath := filepath.Join(profilesDir, "perso.yaml")
		if err := os.MkdirAll(profilesDir, 0755); err != nil {
			t.Fatalf("failed to create profiles dir: %v", err)
		}
		if err := os.WriteFile(profilePath, []byte(profileContent), 0644); err != nil {
			t.Fatalf("failed to write profile: %v", err)
		}

		// Load the profile
		profile, err := svc.LoadProfile("perso")
		if err != nil {
			t.Fatalf("LoadProfile() error = %v", err)
		}
		if profile.Name != "perso" {
			t.Errorf("LoadProfile() Name = %q, want %q", profile.Name, "perso")
		}
		if profile.Vars["TEST_VAR"] != "test-value" {
			t.Errorf("LoadProfile() TEST_VAR = %q, want %q", profile.Vars["TEST_VAR"], "test-value")
		}
	})

	t.Run("returns error for non-existing profile", func(t *testing.T) {
		svc := newTestService(t)

		_, err := svc.LoadProfile("nonexistent")
		if err == nil {
			t.Error("LoadProfile() expected error for non-existing profile, got nil")
		}
	})
}

func TestListProfiles(t *testing.T) {
	t.Run("lists existing profiles", func(t *testing.T) {
		svc := newTestService(t)

		// Create test profiles
		profilesDir, _ := config.GetProfilesDir()
		if err := os.MkdirAll(profilesDir, 0755); err != nil {
			t.Fatalf("failed to create profiles dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(profilesDir, "perso.yaml"), []byte("name: perso"), 0644); err != nil {
			t.Fatalf("failed to write profile: %v", err)
		}
		if err := os.WriteFile(filepath.Join(profilesDir, "work.yaml"), []byte("name: work"), 0644); err != nil {
			t.Fatalf("failed to write profile: %v", err)
		}

		// List profiles
		profiles, err := svc.ListProfiles()
		if err != nil {
			t.Fatalf("ListProfiles() error = %v", err)
		}
		if len(profiles) != 2 {
			t.Errorf("ListProfiles() count = %d, want %d", len(profiles), 2)
		}
	})

	t.Run("returns empty list when no profiles exist", func(t *testing.T) {
		svc := newTestService(t)

		profiles, err := svc.ListProfiles()
		if err != nil {
			t.Fatalf("ListProfiles() error = %v", err)
		}
		if len(profiles) != 0 {
			t.Errorf("ListProfiles() count = %d, want %d", len(profiles), 0)
		}
	})
}

func TestSetCurrentProfile(t *testing.T) {
	svc := newTestService(t)

	// Create a profile first
	profilesDir, _ := config.GetProfilesDir()
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatalf("failed to create profiles dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profilesDir, "perso.yaml"), []byte("name: perso"), 0644); err != nil {
		t.Fatalf("failed to write profile: %v", err)
	}

	// Set current profile
	err := svc.SetCurrentProfile("perso")
	if err != nil {
		t.Fatalf("SetCurrentProfile() error = %v", err)
	}

	// Verify it was set
	current, err := svc.GetCurrentProfile()
	if err != nil {
		t.Fatalf("GetCurrentProfile() error = %v", err)
	}
	if current != "perso" {
		t.Errorf("GetCurrentProfile() = %q, want %q", current, "perso")
	}
}

func TestProfileExists(t *testing.T) {
	svc := newTestService(t)

	// Create a profile
	profilesDir, _ := config.GetProfilesDir()
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatalf("failed to create profiles dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profilesDir, "perso.yaml"), []byte("name: perso"), 0644); err != nil {
		t.Fatalf("failed to write profile: %v", err)
	}

	// Test existing profile
	exists, err := svc.ProfileExists("perso")
	if err != nil {
		t.Fatalf("ProfileExists() error = %v", err)
	}
	if !exists {
		t.Error("ProfileExists(perso) = false, want true")
	}

	// Test non-existing profile
	exists, err = svc.ProfileExists("nonexistent")
	if err != nil {
		t.Fatalf("ProfileExists() error = %v", err)
	}
	if exists {
		t.Error("ProfileExists(nonexistent) = true, want false")
	}
}

func TestGetActiveProfile(t *testing.T) {
	t.Run("no current profile set, empty name returns error", func(t *testing.T) {
		svc := newTestService(t)

		_, err := svc.GetActiveProfile("")
		if err == nil {
			t.Error("GetActiveProfile() expected error when no current profile set, got nil")
		}
	})

	t.Run("explicit non-existing profile returns error", func(t *testing.T) {
		svc := newTestService(t)

		_, err := svc.GetActiveProfile("nonexistent")
		if err == nil {
			t.Error("GetActiveProfile() expected error for non-existing profile, got nil")
		}
	})

	t.Run("uses current profile when name is empty", func(t *testing.T) {
		svc := newTestService(t)

		configDir, _ := config.GetConfigDir()
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("failed to create config dir: %v", err)
		}
		// Create the profile first
		profilesDir, _ := config.GetProfilesDir()
		if err := os.MkdirAll(profilesDir, 0755); err != nil {
			t.Fatalf("failed to create profiles dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(profilesDir, "perso.yaml"), []byte("name: perso"), 0644); err != nil {
			t.Fatalf("failed to write profile: %v", err)
		}
		configContent := `currentProfile: perso`
		configPath := filepath.Join(configDir, config.ConfigFileName)
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		profile, err := svc.GetActiveProfile("")
		if err != nil {
			t.Fatalf("GetActiveProfile() error = %v", err)
		}
		if profile.Name != "perso" {
			t.Errorf("GetActiveProfile() Name = %q, want %q", profile.Name, "perso")
		}
	})

	t.Run("uses explicit profile name when provided", func(t *testing.T) {
		svc := newTestService(t)

		// Create the profile first
		profilesDir, _ := config.GetProfilesDir()
		if err := os.MkdirAll(profilesDir, 0755); err != nil {
			t.Fatalf("failed to create profiles dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(profilesDir, "perso.yaml"), []byte("name: perso"), 0644); err != nil {
			t.Fatalf("failed to write profile: %v", err)
		}

		profile, err := svc.GetActiveProfile("perso")
		if err != nil {
			t.Fatalf("GetActiveProfile() error = %v", err)
		}
		if profile.Name != "perso" {
			t.Errorf("GetActiveProfile() Name = %q, want %q", profile.Name, "perso")
		}
	})
}

func TestConfigService_BuildDefaultProfile(t *testing.T) {
	t.Run("nominal case with git config", func(t *testing.T) {
		setupTestConfig(t)

		// Create mock Git provider
		mockGit := &mock_config.GitConfigProvider{}
		mockGit.On("GetUserName").Return("Test User", nil)
		mockGit.On("GetUserEmail").Return("test@example.com", nil)

		// Create mock HomeDir provider
		mockHomeDir := &mock_config.HomeDirProvider{}
		mockHomeDir.On("GetHomeDir").Return("/tmp/test-home", nil)

		// Create service with mocks
		svc := config.NewConfigService(mockGit, mockHomeDir, nil, nil)

		// Build profile (logic only, no I/O)
		profile, err := svc.BuildDefaultProfile("test")
		if err != nil {
			t.Fatalf("BuildDefaultProfile() error = %v", err)
		}

		// Verify profile name
		if profile.Name != "test" {
			t.Errorf("BuildDefaultProfile() Name = %q, want %q", profile.Name, "test")
		}

		// Verify variables
		expectedVars := map[string]string{
			"AI_CABIN_HOME":    "/tmp/test-home",
			"AI_CABIN_DESK":    "/tmp/test-home/Documents/desk",
			"AI_CABIN_WORKDIR": "/tmp/test-home/projects",
			"GIT_AGENT_NAME":   "AI Agent + Test User",
			"GIT_AGENT_EMAIL":  "test@example.com",
		}

		for key, expected := range expectedVars {
			value, ok := profile.Vars[key]
			if !ok {
				t.Errorf("BuildDefaultProfile() missing variable %q", key)
				continue
			}
			if value != expected {
				t.Errorf("BuildDefaultProfile() %s = %q, want %q", key, value, expected)
			}
		}

		// Verify mock expectations
		mockGit.AssertExpectations(t)
		mockHomeDir.AssertExpectations(t)
	})

	t.Run("uses defaults when git fails", func(t *testing.T) {
		setupTestConfig(t)

		// Create mock Git provider that returns errors
		mockGit := &mock_config.GitConfigProvider{}
		mockGit.On("GetUserName").Return("", fmt.Errorf("git not configured"))
		mockGit.On("GetUserEmail").Return("", fmt.Errorf("git not configured"))

		// Create mock HomeDir provider
		mockHomeDir := &mock_config.HomeDirProvider{}
		mockHomeDir.On("GetHomeDir").Return("/tmp/test-home", nil)

		// Create service with mocks
		svc := config.NewConfigService(mockGit, mockHomeDir, nil, nil)

		// Build profile (should succeed with defaults)
		profile, err := svc.BuildDefaultProfile("test")
		if err != nil {
			t.Fatalf("BuildDefaultProfile() error = %v", err)
		}

		// Verify default values are used when git fails
		if profile.Vars["GIT_AGENT_NAME"] != "AI Agent" {
			t.Errorf("BuildDefaultProfile() GIT_AGENT_NAME = %q, want %q", profile.Vars["GIT_AGENT_NAME"], "AI Agent")
		}
		if profile.Vars["GIT_AGENT_EMAIL"] != "ai-agent@localhost" {
			t.Errorf("BuildDefaultProfile() GIT_AGENT_EMAIL = %q, want %q", profile.Vars["GIT_AGENT_EMAIL"], "ai-agent@localhost")
		}

		mockGit.AssertExpectations(t)
		mockHomeDir.AssertExpectations(t)
	})

	t.Run("returns error when homeDir fails", func(t *testing.T) {
		setupTestConfig(t)

		// Create mock HomeDir provider that returns error
		mockHomeDir := &mock_config.HomeDirProvider{}
		mockHomeDir.On("GetHomeDir").Return("", fmt.Errorf("home dir not found"))

		// Create service with mock
		svc := config.NewConfigService(&mock_config.GitConfigProvider{}, mockHomeDir, nil, nil)

		// Build profile should fail
		_, err := svc.BuildDefaultProfile("test")
		if err == nil {
			t.Fatal("BuildDefaultProfile() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "home dir not found") {
			t.Errorf("BuildDefaultProfile() error = %q, should contain 'home dir not found'", err.Error())
		}

		mockHomeDir.AssertExpectations(t)
	})
}

func TestConfigService_SaveProfile(t *testing.T) {
	setupTestConfig(t)

	// SaveProfile doesn't use gitProvider or homeDir, so nil is safe.
	svc := config.NewConfigService(nil, nil, nil, config.AtomicFileWriter{})

	// Build a profile
	profile := &config.Profile{
		Name: "test-save",
		Vars: map[string]string{
			"TEST_VAR": "test-value",
		},
	}

	// Save the profile
	err := svc.SaveProfile(profile)
	if err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}

	// Verify file was created
	if profile.Path() == "" {
		t.Error("SaveProfile() Path() is empty after save")
	}
	if _, err := os.Stat(profile.Path()); os.IsNotExist(err) {
		t.Errorf("SaveProfile() profile file not created at %q", profile.Path())
	}

	// Verify file content
	data, err := os.ReadFile(profile.Path())
	if err != nil {
		t.Fatalf("failed to read profile file: %v", err)
	}
	if !strings.Contains(string(data), "test-save") {
		t.Errorf("SaveProfile() file content doesn't contain profile name: %s", string(data))
	}
	if !strings.Contains(string(data), "test-value") {
		t.Errorf("SaveProfile() file content doesn't contain test var: %s", string(data))
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

func TestGetCurrentProfile(t *testing.T) {
	t.Run("returns_empty_when_config_file_does_not_exist", func(t *testing.T) {
		mapFS := fstest.MapFS{}
		svc := config.NewConfigService(nil, nil, mapFS, nil)

		current, err := svc.GetCurrentProfile()
		if err != nil {
			t.Fatalf("GetCurrentProfile() error = %v", err)
		}
		if current != "" {
			t.Errorf("GetCurrentProfile() = %q, want empty string", current)
		}
	})

	t.Run("returns_profile_name_when_config_exists", func(t *testing.T) {
		mapFS := fstest.MapFS{
			config.ConfigFileName: &fstest.MapFile{
				Data: []byte("currentProfile: perso"),
			},
		}
		svc := config.NewConfigService(nil, nil, mapFS, nil)

		current, err := svc.GetCurrentProfile()
		if err != nil {
			t.Fatalf("GetCurrentProfile() error = %v", err)
		}
		if current != "perso" {
			t.Errorf("GetCurrentProfile() = %q, want %q", current, "perso")
		}
	})

	t.Run("returns error when yaml is invalid", func(t *testing.T) {
		mapFS := fstest.MapFS{
			config.ConfigFileName: &fstest.MapFile{
				Data: []byte("::: not valid yaml :::"),
			},
		}
		svc := config.NewConfigService(nil, nil, mapFS, nil)

		_, err := svc.GetCurrentProfile()
		if err == nil {
			t.Fatal("GetCurrentProfile() expected error for invalid yaml, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse config file") {
			t.Errorf("GetCurrentProfile() error = %q, should contain 'failed to parse config file'", err.Error())
		}
	})
}

func TestConfigService_LoadProfile(t *testing.T) {
	t.Run("loads profile with fstest.MapFS", func(t *testing.T) {
		mapFS := fstest.MapFS{
			"profiles/perso.yaml": &fstest.MapFile{
				Data: []byte("name: perso\nvars:\n  TEST: value"),
			},
		}

		svc := config.NewConfigService(nil, nil, mapFS, nil)

		profile, err := svc.LoadProfile("perso")
		if err != nil {
			t.Fatalf("LoadProfile() error = %v", err)
		}
		if profile.Name != "perso" {
			t.Errorf("LoadProfile() Name = %q, want %q", profile.Name, "perso")
		}
		if profile.Vars["TEST"] != "value" {
			t.Errorf("LoadProfile() TEST = %q, want %q", profile.Vars["TEST"], "value")
		}
	})

	t.Run("returns error when profile not found", func(t *testing.T) {
		mapFS := fstest.MapFS{}
		svc := config.NewConfigService(nil, nil, mapFS, nil)

		_, err := svc.LoadProfile("nonexistent")
		if err == nil {
			t.Fatal("LoadProfile() expected error, got nil")
		}
	})

	t.Run("returns error when filesystem fails", func(t *testing.T) {
		mockFS := &mock_fs.FS{}
		mockFS.On("Open", "profiles/test.yaml").Return(nil, fmt.Errorf("disk error"))

		svc := config.NewConfigService(nil, nil, mockFS, nil)

		_, err := svc.LoadProfile("test")
		if err == nil {
			t.Fatal("LoadProfile() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "disk error") {
			t.Errorf("LoadProfile() error = %q, should contain 'disk error'", err.Error())
		}

		mockFS.AssertExpectations(t)
	})

	t.Run("returns error when yaml is invalid", func(t *testing.T) {
		mapFS := fstest.MapFS{
			"profiles/bad.yaml": &fstest.MapFile{
				Data: []byte("::: not valid yaml :::"),
			},
		}
		svc := config.NewConfigService(nil, nil, mapFS, nil)

		_, err := svc.LoadProfile("bad")
		if err == nil {
			t.Fatal("LoadProfile() expected error for invalid yaml, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse profile") {
			t.Errorf("LoadProfile() error = %q, should contain 'failed to parse profile'", err.Error())
		}
	})
}

func TestConfigService_ListProfiles(t *testing.T) {
	t.Run("lists profiles with fstest.MapFS", func(t *testing.T) {
		mapFS := fstest.MapFS{
			"profiles/perso.yaml": &fstest.MapFile{Data: []byte("name: perso")},
			"profiles/work.yaml":  &fstest.MapFile{Data: []byte("name: work")},
		}
		svc := config.NewConfigService(nil, nil, mapFS, nil)

		profiles, err := svc.ListProfiles()
		if err != nil {
			t.Fatalf("ListProfiles() error = %v", err)
		}
		if len(profiles) != 2 {
			t.Errorf("ListProfiles() count = %d, want %d", len(profiles), 2)
		}
	})

	t.Run("returns empty list when profiles dir does not exist", func(t *testing.T) {
		mapFS := fstest.MapFS{}
		svc := config.NewConfigService(nil, nil, mapFS, nil)

		profiles, err := svc.ListProfiles()
		if err != nil {
			t.Fatalf("ListProfiles() error = %v", err)
		}
		if len(profiles) != 0 {
			t.Errorf("ListProfiles() count = %d, want %d", len(profiles), 0)
		}
	})

	t.Run("returns error when filesystem fails", func(t *testing.T) {
		mockFS := &mock_fs.ReadDirFS{}
		mockFS.On("ReadDir", config.ProfilesDirName).Return([]fs.DirEntry(nil), fmt.Errorf("disk error"))

		svc := config.NewConfigService(nil, nil, mockFS, nil)

		_, err := svc.ListProfiles()
		if err == nil {
			t.Fatal("ListProfiles() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "disk error") {
			t.Errorf("ListProfiles() error = %q, should contain 'disk error'", err.Error())
		}

		mockFS.AssertExpectations(t)
	})

	t.Run("returns error when fs does not implement ReadDirFS", func(t *testing.T) {
		// mock_fs.FS only implements Open, not ReadDir.
		mockFS := &mock_fs.FS{}
		svc := config.NewConfigService(nil, nil, mockFS, nil)

		_, err := svc.ListProfiles()
		if err == nil {
			t.Fatal("ListProfiles() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "does not implement fs.ReadDirFS") {
			t.Errorf("ListProfiles() error = %q, should contain 'does not implement fs.ReadDirFS'", err.Error())
		}
	})

	t.Run("skips directories and non-yaml files", func(t *testing.T) {
		mapFS := fstest.MapFS{
			"profiles/perso.yaml":   &fstest.MapFile{Data: []byte("name: perso")},
			"profiles/subdir":       &fstest.MapFile{Mode: fs.ModeDir},
			"profiles/README.md":    &fstest.MapFile{Data: []byte("not a profile")},
			"profiles/template.txt": &fstest.MapFile{Data: []byte("ignored")},
		}
		svc := config.NewConfigService(nil, nil, mapFS, nil)

		profiles, err := svc.ListProfiles()
		if err != nil {
			t.Fatalf("ListProfiles() error = %v", err)
		}
		if len(profiles) != 1 {
			t.Errorf("ListProfiles() count = %d, want %d (only .yaml files, dirs ignored)", len(profiles), 1)
		}
		if len(profiles) > 0 && profiles[0] != "perso" {
			t.Errorf("ListProfiles() first = %q, want %q", profiles[0], "perso")
		}
	})
}

func TestConfigService_ProfileExists(t *testing.T) {
	t.Run("returns true when profile exists in fstest.MapFS", func(t *testing.T) {
		mapFS := fstest.MapFS{
			"profiles/perso.yaml": &fstest.MapFile{Data: []byte("name: perso")},
		}
		svc := config.NewConfigService(nil, nil, mapFS, nil)

		exists, err := svc.ProfileExists("perso")
		if err != nil {
			t.Fatalf("ProfileExists() error = %v", err)
		}
		if !exists {
			t.Error("ProfileExists(perso) = false, want true")
		}
	})

	t.Run("returns false when profile not found", func(t *testing.T) {
		mapFS := fstest.MapFS{}
		svc := config.NewConfigService(nil, nil, mapFS, nil)

		exists, err := svc.ProfileExists("nonexistent")
		if err != nil {
			t.Fatalf("ProfileExists() error = %v", err)
		}
		if exists {
			t.Error("ProfileExists(nonexistent) = true, want false")
		}
	})

	t.Run("returns error when filesystem fails", func(t *testing.T) {
		mockFS := &mock_fs.StatFS{}
		mockFS.On("Stat", "profiles/test.yaml").Return(nil, fmt.Errorf("disk error"))

		svc := config.NewConfigService(nil, nil, mockFS, nil)

		_, err := svc.ProfileExists("test")
		if err == nil {
			t.Fatal("ProfileExists() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "disk error") {
			t.Errorf("ProfileExists() error = %q, should contain 'disk error'", err.Error())
		}

		mockFS.AssertExpectations(t)
	})
}

func TestConfigService_SetCurrentProfile_WriteError(t *testing.T) {
	setupTestConfig(t)

	mockWriter := &mock_config.FileWriter{}
	mockWriter.On("WriteFile",
		mock.Anything, // path
		mock.Anything, // data
		mock.Anything, // perm
	).Return(fmt.Errorf("disk write error"))

	svc := config.NewConfigService(nil, nil, nil, mockWriter)

	err := svc.SetCurrentProfile("perso")
	if err == nil {
		t.Fatal("SetCurrentProfile() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "disk write error") {
		t.Errorf("SetCurrentProfile() error = %q, should contain 'disk write error'", err.Error())
	}
	if !strings.Contains(err.Error(), "failed to write config file") {
		t.Errorf("SetCurrentProfile() error = %q, should be wrapped by 'failed to write config file'", err.Error())
	}

	mockWriter.AssertExpectations(t)
}

func TestConfigService_SaveProfile_WriteError(t *testing.T) {
	setupTestConfig(t)

	mockWriter := &mock_config.FileWriter{}
	mockWriter.On("WriteFile",
		mock.Anything, // path
		mock.Anything, // data
		mock.Anything, // perm
	).Return(fmt.Errorf("disk write error"))

	svc := config.NewConfigService(nil, nil, nil, mockWriter)

	profile := &config.Profile{
		Name: "test-save",
		Vars: map[string]string{"TEST_VAR": "test-value"},
	}

	err := svc.SaveProfile(profile)
	if err == nil {
		t.Fatal("SaveProfile() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "disk write error") {
		t.Errorf("SaveProfile() error = %q, should contain 'disk write error'", err.Error())
	}
	if !strings.Contains(err.Error(), "failed to write profile file") {
		t.Errorf("SaveProfile() error = %q, should be wrapped by 'failed to write profile file'", err.Error())
	}

	mockWriter.AssertExpectations(t)
}

func TestConfigService_GetCabin(t *testing.T) {
	svc := newTestService(t)
	if err := svc.AddCabin("blog", "/blog/path"); err != nil {
		t.Fatalf("AddCabin() error = %v", err)
	}

	t.Run("returns cabin added via ConfigService", func(t *testing.T) {
		c, err := svc.GetCabin("blog")
		if err != nil {
			t.Fatalf("GetCabin(blog) error = %v", err)
		}
		if c.Name != "blog" || c.Path != "/blog/path" {
			t.Errorf("GetCabin(blog) = {%q, %q}, want {blog, /blog/path}", c.Name, c.Path)
		}
	})

	t.Run("missing name returns ErrCabinNotFound", func(t *testing.T) {
		_, err := svc.GetCabin("ghost")
		if !errors.Is(err, config.ErrCabinNotFound) {
			t.Errorf("GetCabin(ghost) error = %v, want ErrCabinNotFound", err)
		}
	})
}

// unsetEnv unsets the given env vars for the duration of the test, restoring
// them (set or unset, with their original value) on cleanup. Unlike
// t.Setenv(k, "") which sets a var to empty (still an override), this truly
// unsets — needed when a test asserts the "existing" or "default" value wins
// and the host env carries the var (e.g. OPENCODE_SERVER_PASSWORD in the dev
// container).
func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	saved := make(map[string]string, len(keys))
	wasSet := make(map[string]bool, len(keys))
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			saved[k] = v
			wasSet[k] = true
		}
		_ = os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for _, k := range keys {
			if wasSet[k] {
				_ = os.Setenv(k, saved[k])
			} else {
				_ = os.Unsetenv(k)
			}
		}
	})
}

// newInitService builds a ConfigService backed by a temp config dir with a
// fixed home (/tmp/test-home) and git identity, so the defaults are stable
// across sub-cases. The AI_CABIN_*/GIT_AGENT_* env vars are unset (via the
// shared unsetCabinEnv helper) so the defaults from the mock providers apply;
// sub-cases that test env override set them explicitly via t.Setenv.
func newInitService(t *testing.T) *config.ConfigService {
	t.Helper()
	setupTestConfig(t)
	unsetCabinEnv(t)
	configDir, err := config.GetConfigDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	mockGit := &mock_config.GitConfigProvider{}
	mockGit.On("GetUserName").Return("Test User", nil)
	mockGit.On("GetUserEmail").Return("test@example.com", nil)
	mockHomeDir := &mock_config.HomeDirProvider{}
	mockHomeDir.On("GetHomeDir").Return("/tmp/test-home", nil)
	return config.NewConfigService(mockGit, mockHomeDir, os.DirFS(configDir), config.AtomicFileWriter{})
}

// initDefaultVars is the set BuildDefaultProfile produces (the bounded base).
// Used to assert the persisted set stays bounded — never the whole env.
var initDefaultVars = map[string]string{
	config.HomeVar:    "/tmp/test-home",
	config.DeskVar:    "/tmp/test-home/Documents/desk",
	config.WorkdirVar: "/tmp/test-home/projects",
	"GIT_AGENT_NAME":  "AI Agent + Test User",
	"GIT_AGENT_EMAIL": "test@example.com",
}

func TestConfigService_InitProfile(t *testing.T) {
	t.Run("NewProfilePersistsDefaultsOnly", func(t *testing.T) {
		// A new init with no --var persists exactly the default set: defaults
		// from BuildDefaultProfile, no env stray, no typed-var normalization
		// (CONTAINER_WORKDIR/CREDENTIAL_* are runtime concerns, not persisted).
		svc := newInitService(t)

		profile, err := svc.InitProfile("dev", nil, false)
		require.NoError(t, err)
		assert.Equal(t, "dev", profile.Name)
		assert.Equal(t, initDefaultVars, profile.Vars, "persisted set must be exactly the defaults")
	})

	t.Run("VarEnlargesTheSet", func(t *testing.T) {
		// --var adds a key not in defaults (the initial `set` of the CRUD):
		// a custom desk path, or a credential var, is persisted as-is.
		svc := newInitService(t)
		cliVars := []string{"AI_CABIN_DESK=/custom/desk", "OPENCODE_SERVER_PASSWORD=s3cret"}

		profile, err := svc.InitProfile("dev", cliVars, false)
		require.NoError(t, err)

		// --var AI_CABIN_DESK overrides the default value.
		assert.Equal(t, "/custom/desk", profile.Vars[config.DeskVar])
		// OPENCODE_SERVER_PASSWORD was added (enlarged set).
		assert.Equal(t, "s3cret", profile.Vars["OPENCODE_SERVER_PASSWORD"])
		// The rest of the defaults are still present.
		assert.Equal(t, initDefaultVars[config.HomeVar], profile.Vars[config.HomeVar])
	})

	t.Run("EnvOverridesDefaultValueButDoesNotEnlarge", func(t *testing.T) {
		// An env var matching a default key overrides its value (env > defaults),
		// but an env var outside the bounded set is dropped (no PATH stray).
		svc := newInitService(t) // unsets AI_CABIN_* host env first.
		t.Setenv(config.DeskVar, "/env/desk")
		t.Setenv("PATH", "/usr/bin:/bin")
		t.Setenv("SCW_PROJECT_ID", "should-not-persist")

		profile, err := svc.InitProfile("dev", nil, false)
		require.NoError(t, err)

		assert.Equal(t, "/env/desk", profile.Vars[config.DeskVar], "env overrides a default value")
		_, hasPath := profile.Vars["PATH"]
		assert.False(t, hasPath, "PATH must not be persisted (outside the bounded set)")
		_, hasSCW := profile.Vars["SCW_PROJECT_ID"]
		assert.False(t, hasSCW, "SCW_PROJECT_ID must not be persisted (not a default, not a --var)")
	})

	t.Run("VarWinsOverEnv", func(t *testing.T) {
		// --var is the highest precedence: when both --var and env set a key,
		// --var wins.
		svc := newInitService(t) // unsets AI_CABIN_* host env first.
		t.Setenv(config.DeskVar, "/env/desk")

		profile, err := svc.InitProfile("dev", []string{"AI_CABIN_DESK=/var/desk"}, false)
		require.NoError(t, err)
		assert.Equal(t, "/var/desk", profile.Vars[config.DeskVar], "--var wins over env")
	})

	t.Run("ExistingNoForceIsNoOp", func(t *testing.T) {
		// On an existing profile without --force: no-op, returns the existing
		// profile unchanged (warn + exit 0 is the CLI's job). Mirrors cabin add.
		svc := newInitService(t)
		// Create the profile first with a custom var.
		orig, err := svc.InitProfile("dev", []string{"CUSTOM=first"}, false)
		require.NoError(t, err)

		// Second init without --force: no-op, --var ignored.
		profile, err := svc.InitProfile("dev", []string{"CUSTOM=second"}, false)
		require.NoError(t, err)
		assert.Equal(t, "first", profile.Vars["CUSTOM"], "existing profile must not be modified")
		assert.Equal(t, orig.Vars, profile.Vars, "returns the existing profile vars unchanged")
	})

	t.Run("ForceOverwritesAndKeepsExistingKeys", func(t *testing.T) {
		// --force overwrites: persisted = defaults ∪ --var ∪ existing. Keys
		// already in the existing profile (not in defaults, not in --var) are
		// kept; values are resolved (--var > env > existing > defaults).
		// Unset OPENCODE_SERVER_PASSWORD from the host env so the existing value
		// is the resolved one — env > existing would otherwise override it with
		// the host value (correct behaviour, but it masks the "existing key
		// kept" assertion under test).
		unsetEnv(t, "OPENCODE_SERVER_PASSWORD")
		svc := newInitService(t)
		// First init: create with a custom var the user added via the CRUD.
		_, err := svc.InitProfile("dev", []string{"OPENCODE_SERVER_PASSWORD=old-pass"}, false)
		require.NoError(t, err)
		// Set an env override on a default key to exercise the env > existing
		// precedence (existing was the default /tmp/test-home/...).
		t.Setenv(config.WorkdirVar, "/env/projects")

		profile, err := svc.InitProfile("dev", []string{"AI_CABIN_DESK=/new/desk"}, true)
		require.NoError(t, err)

		// --var override applied.
		assert.Equal(t, "/new/desk", profile.Vars[config.DeskVar])
		// Existing key kept (not in defaults, not in --var) — the CRUD value
		// survives a --force re-init.
		assert.Equal(t, "old-pass", profile.Vars["OPENCODE_SERVER_PASSWORD"])
		// Env override on a default key applied (env > existing).
		assert.Equal(t, "/env/projects", profile.Vars[config.WorkdirVar])
		// Defaults still present.
		assert.Equal(t, initDefaultVars[config.HomeVar], profile.Vars[config.HomeVar])
	})

	t.Run("ForceOnNonExistingActsAsNewInit", func(t *testing.T) {
		// --force on a non-existing profile behaves like a new init (no
		// existing keys to keep). The force flag is a no-op in that case.
		svc := newInitService(t)

		profile, err := svc.InitProfile("dev", nil, true)
		require.NoError(t, err)
		assert.Equal(t, initDefaultVars, profile.Vars, "force on a new profile = defaults only")
	})

	t.Run("BuildDefaultProfileError", func(t *testing.T) {
		// When BuildDefaultProfile fails (home dir lookup error), InitProfile
		// propagates the error and writes nothing.
		setupTestConfig(t)
		unsetCabinEnv(t)
		configDir, err := config.GetConfigDir()
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(configDir, 0o755))
		mockHomeDir := &mock_config.HomeDirProvider{}
		mockHomeDir.On("GetHomeDir").Return("", fmt.Errorf("home dir not found"))
		svc := config.NewConfigService(&mock_config.GitConfigProvider{}, mockHomeDir, os.DirFS(configDir), config.AtomicFileWriter{})

		_, err = svc.InitProfile("dev", nil, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "home dir not found")
		// Nothing was written.
		profilesDir, err := config.GetProfilesDir()
		require.NoError(t, err)
		_, statErr := os.Stat(filepath.Join(profilesDir, "dev.yaml"))
		assert.True(t, os.IsNotExist(statErr), "no profile file written on build error")
	})

	t.Run("InvalidVar", func(t *testing.T) {
		// A --var without `=` is rejected with a clear error (same shape as
		// ResolveVars).
		svc := newInitService(t)
		_, err := svc.InitProfile("dev", []string{"NO_EQUALS"}, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected KEY=VAL")
	})

	t.Run("EmptyNameDefaultsToDefault", func(t *testing.T) {
		// An empty name defaults to "default" (matches the CLI's default).
		svc := newInitService(t)
		profile, err := svc.InitProfile("", nil, false)
		require.NoError(t, err)
		assert.Equal(t, "default", profile.Name)
		// The profile file exists at the expected path.
		profilesDir, err := config.GetProfilesDir()
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(profilesDir, "default.yaml"))
		require.NoError(t, err)
	})
}
