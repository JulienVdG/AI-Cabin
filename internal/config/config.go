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

// ProfileSource identifies which layer selected the active profile name.
type ProfileSource string

const (
	// ProfileSourceArg: selected by an explicit positional name (profile show <name>).
	ProfileSourceArg ProfileSource = "argument"
	// ProfileSourceFlag: selected by the root --profile flag.
	ProfileSourceFlag ProfileSource = "--profile"
	// ProfileSourceVar: selected by --var AI_CABIN_PROFILE=... (an alternate
	// spelling of the --profile selector, so it takes the same precedence slot).
	ProfileSourceVar ProfileSource = "--var AI_CABIN_PROFILE"
	// ProfileSourceEnv: selected by the AI_CABIN_PROFILE env var.
	ProfileSourceEnv ProfileSource = "AI_CABIN_PROFILE"
	// ProfileSourceConfig: the current profile from config.yaml.
	ProfileSourceConfig ProfileSource = "config"
)

// ProfileSelection describes the resolved active profile. Name is the active
// profile following the standard precedence (explicit positional name > root
// --profile flag / --var AI_CABIN_PROFILE > AI_CABIN_PROFILE env > current
// profile from config.yaml). Source records which layer selected it. Use is
// the profile persisted in config.yaml currentProfile (the one set with
// `cabin profile use`), for callers that warn when the effective profile
// diverges from the persisted selection.
type ProfileSelection struct {
	Name   string
	Source ProfileSource
	Use    string
}

// Overridden reports whether the effective profile (Name) diverges from the
// use-selected profile (Use) because it was chosen via --profile,
// --var AI_CABIN_PROFILE or AI_CABIN_PROFILE. A positional name
// (ProfileSourceArg) is never an override (the user asked for that profile
// explicitly). It drives both the origin annotation and the divergence warning.
func (sel ProfileSelection) Overridden() bool {
	switch sel.Source {
	case ProfileSourceFlag, ProfileSourceVar, ProfileSourceEnv:
		return sel.Name != sel.Use
	default:
		return false
	}
}

// OverrideWarning returns a stderr warning (or empty) when the effective
// profile differs from the profile persisted with `cabin profile use`. It is
// empty when there is no override (including an explicit positional name).
func (sel ProfileSelection) OverrideWarning() string {
	if !sel.Overridden() {
		return ""
	}
	if sel.Use == "" {
		return fmt.Sprintf("Warning: %s selects profile %q, but no profile is set with 'cabin profile use'. Use 'cabin profile use %s' to persist it.",
			string(sel.Source), sel.Name, sel.Name)
	}
	return fmt.Sprintf("Warning: %s selects profile %q, which differs from the profile set by 'cabin profile use' (%q). Use 'cabin profile use %s' to persist it.",
		string(sel.Source), sel.Name, sel.Use, sel.Name)
}

// ResolveProfile resolves the active profile name and its origin. It follows
// the standard precedence (explicit name > --profile > --var AI_CABIN_PROFILE >
// AI_CABIN_PROFILE env > current profile from config.yaml) and returns Use
// separately so display sites (profile list/show/group help) can warn when the
// effective profile differs from the one set with `cabin profile use`. It
// resolves through the same selector as ResolveVars, so the displayed active
// profile is guaranteed to match the one the runtime commands use. It does not
// check existence nor load the file; callers that load also use GetActiveProfile.
func ResolveProfile(name, profileFlag string, cliVars []string) (ProfileSelection, error) {
	return configService.ResolveProfile(name, profileFlag, cliVars)
}

// ResolveCabin returns the target cabin for cabin-scoped commands, resolving
// --cabin > AI_CABIN_CURRENT_CABIN env > active profile var. It delegates to the
// global ConfigService.
func ResolveCabin(cabinFlag, profileFlag string) (string, error) {
	return configService.ResolveCabin(cabinFlag, profileFlag)
}

// InitProfile creates or overwrites a profile with a bounded set of resolved
// vars (defaults ∪ --var ∪ existing-on-force) and returns the persisted profile.
// On an existing profile without force it is a no-op returning the existing
// profile. See ConfigService.InitProfile for the persistence rule.
func InitProfile(name string, cliVars []string, force bool) (*Profile, error) {
	return configService.InitProfile(name, cliVars, force)
}

// SetProfileVars sets several variables on a profile (resolved via
// GetActiveProfile if name is empty -> current profile) and persists them in one
// atomic write. It delegates to the global ConfigService.
func SetProfileVars(name string, vars Vars) (*Profile, error) {
	return configService.SetProfileVars(name, vars)
}

// SetProfileVar sets a single variable on a profile (resolved via
// GetActiveProfile if name is empty -> current profile) and persists it
// atomically. It delegates to the global ConfigService.
func SetProfileVar(name, key, value string) (*Profile, error) {
	return configService.SetProfileVar(name, key, value)
}
