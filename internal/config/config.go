package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Profile represents a user profile with environment variables.
type Profile struct {
	Name string            `yaml:"name"`
	Vars map[string]string `yaml:"vars"`
	path string            // runtime path, not serialized
}

// Path returns the full path to the profile file.
func (p *Profile) Path() string {
	return p.path
}

// Config represents the main config file (~/.config/ai-cabin/config.yaml).
type Config struct {
	CurrentProfile string `yaml:"currentProfile"`
}

// GetConfigDir returns ~/.config/ai-cabin (respects XDG_CONFIG_HOME).
func GetConfigDir() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "ai-cabin"), nil
}

// GetProfilesDir returns ~/.config/ai-cabin/profiles.
func GetProfilesDir() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "profiles"), nil
}

// LoadProfile loads a profile by name (e.g., "perso" → profiles/perso.yaml).
func LoadProfile(name string) (*Profile, error) {
	profilesDir, err := GetProfilesDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(profilesDir, name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile %q: %w", name, err)
	}

	var profile Profile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile %q: %w", name, err)
	}
	profile.path = path

	return &profile, nil
}

// ListProfiles returns all available profile names.
func ListProfiles() ([]string, error) {
	profilesDir, err := GetProfilesDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read profiles directory: %w", err)
	}

	var profiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".yaml" {
			continue
		}
		profiles = append(profiles, name[:len(name)-5]) // strip .yaml
	}

	return profiles, nil
}

// getConfigPath returns the path to config.yaml.
func getConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"), nil
}

// GetCurrentProfile returns the currently selected profile from config.yaml.
func GetCurrentProfile() (string, error) {
	path, err := getConfigPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // no config yet, no profile selected
		}
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("failed to parse config file: %w", err)
	}

	return config.CurrentProfile, nil
}

// SetCurrentProfile updates config.yaml with the selected profile.
func SetCurrentProfile(name string) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	path, err := getConfigPath()
	if err != nil {
		return err
	}

	config := Config{CurrentProfile: name}
	data, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ProfileExists checks if a profile exists by name.
func ProfileExists(name string) (bool, error) {
	profilesDir, err := GetProfilesDir()
	if err != nil {
		return false, err
	}

	path := filepath.Join(profilesDir, name+".yaml")
	_, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check profile existence: %w", err)
	}
	return true, nil
}

// GetActiveProfile resolves and loads a profile.
// If name is empty, uses the current profile from config.yaml.
// Returns a user-friendly error if the profile doesn't exist or can't be loaded.
func GetActiveProfile(name string) (*Profile, error) {
	if name == "" {
		var err error
		name, err = GetCurrentProfile()
		if err != nil {
			return nil, fmt.Errorf("failed to get current profile: %w", err)
		}
		if name == "" {
			return nil, fmt.Errorf("no current profile selected\nRun 'cabin profile init' to create a default profile, or 'cabin profile use <name>' to select one")
		}
	}

	exists, err := ProfileExists(name)
	if err != nil {
		return nil, fmt.Errorf("failed to check profile existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("profile %q does not exist\nRun 'cabin profile init %s' to create it", name, name)
	}

	profile, err := LoadProfile(name)
	if err != nil {
		return nil, fmt.Errorf("failed to load profile %q: %w", name, err)
	}

	return profile, nil
}

// CreateDefaultProfile creates a default profile with values derived from the environment.
// TODO: AI_CABIN_HOME/DESK/WORKDIR should be templated or passed as parameters to `cabin profile init`.
// For now, we use the user's home as a base, but this needs to be customized per bootstrap-cabin.sh pattern.
func CreateDefaultProfile(name string) (*Profile, error) {
	if name == "" {
		name = "default"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Get Git user name from host config (mimics bootstrap-cabin.sh)
	gitAgentName := "AI Agent"
	gitAgentEmail := "ai-agent@localhost"

	// Try to get git config (best effort, may fail if git not configured)
	if out, err := exec.Command("git", "config", "--global", "user.name").Output(); err == nil {
		name := strings.TrimSpace(string(out))
		if name != "" {
			gitAgentName = "AI Agent + " + name
		}
	}
	if out, err := exec.Command("git", "config", "--global", "user.email").Output(); err == nil {
		email := strings.TrimSpace(string(out))
		if email != "" {
			gitAgentEmail = email
		}
	}

	profile := &Profile{
		Name: name,
		Vars: map[string]string{
			"AI_CABIN_HOME":    home,
			"AI_CABIN_DESK":    filepath.Join(home, "Documents", "desk"),
			"AI_CABIN_WORKDIR": filepath.Join(home, "projects"),
			"GIT_AGENT_NAME":   gitAgentName,
			"GIT_AGENT_EMAIL":  gitAgentEmail,
		},
	}

	// Save the profile
	profilesDir, err := GetProfilesDir()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create profiles directory: %w", err)
	}

	path := filepath.Join(profilesDir, name+".yaml")
	profile.path = path

	data, err := yaml.Marshal(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal profile: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write profile file: %w", err)
	}

	return profile, nil
}
