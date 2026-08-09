package config

import (
	"fmt"
	"os"
	"path/filepath"
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

// GetStateDir returns ~/.local/state/ai-cabin (respects XDG_STATE_HOME). It
// holds runtime artifacts the CLI materializes (e.g. the lifecycle Taskfile)
// so they pre-exist before `task` parses a cabin Taskfile. Redirecting
// XDG_STATE_HOME matters in dev: ~/.local/state is read-only under the
// greywall sandbox.
func GetStateDir() (string, error) {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "ai-cabin"), nil
}

// ProfilesDirName is the subdirectory of the config dir containing profile files.
const ProfilesDirName = "profiles"

// ConfigFileName is the main config file in the config dir (current profile, etc.).
const ConfigFileName = "config.yaml"

// GetProfilesDir returns ~/.config/ai-cabin/profiles.
func GetProfilesDir() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, ProfilesDirName), nil
}

// LoadProfile loads a profile by name (e.g., "perso" → profiles/perso.yaml).
// It delegates to the global ConfigService which uses an os.DirFS in production.
func LoadProfile(name string) (*Profile, error) {
	return configService.LoadProfile(name)
}

// ListProfiles returns all available profile names.
// It delegates to the global ConfigService which uses an os.DirFS in production.
func ListProfiles() ([]string, error) {
	return configService.ListProfiles()
}

// getConfigPath returns the absolute path to the config file.
// Used by write methods (SetCurrentProfile); reads use fs.FS with ConfigFileName instead.
func getConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, ConfigFileName), nil
}

// GetCurrentProfile returns the currently selected profile from config.yaml.
// It delegates to the global ConfigService.
func GetCurrentProfile() (string, error) {
	return configService.GetCurrentProfile()
}

// SetCurrentProfile updates config.yaml with the selected profile.
// It delegates to the global ConfigService.
func SetCurrentProfile(name string) error {
	return configService.SetCurrentProfile(name)
}

// ProfileExists checks if a profile exists by name.
// It delegates to the global ConfigService which uses an os.DirFS in production.
func ProfileExists(name string) (bool, error) {
	return configService.ProfileExists(name)
}

// GetActiveProfile resolves and loads a profile.
// If name is empty, uses the current profile from config.yaml.
// Returns a user-friendly error if the profile doesn't exist or can't be loaded.
// It delegates to the global ConfigService.
func GetActiveProfile(name string) (*Profile, error) {
	return configService.GetActiveProfile(name)
}

// ResolveVars returns the variable view (defaults + selected profile + env
// + --var overrides) the CLI sets on its task subprocess.
func ResolveVars(profileFlag string, cliVars []string) (Vars, error) {
	return configService.ResolveVars(profileFlag, cliVars)
}

// InitProfile creates or overwrites a profile with a bounded set of resolved
// vars (defaults ∪ --var ∪ existing-on-force) and returns the persisted profile.
// On an existing profile without force it is a no-op returning the existing
// profile. See ConfigService.InitProfile for the persistence rule.
func InitProfile(name string, cliVars []string, force bool) (*Profile, error) {
	return configService.InitProfile(name, cliVars, force)
}
