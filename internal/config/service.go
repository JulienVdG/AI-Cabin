package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigService provides configuration management with injectable dependencies.
type ConfigService struct {
	gitProvider GitConfigProvider
	homeDir     HomeDirProvider
	fs          fs.FS
	writer      FileWriter
}

// NewConfigService creates a new ConfigService with all dependencies explicit.
// This is the canonical DI constructor (see skill:go-test-patterns):
//   - gitProvider: Git user identity source (nil if unused by the tested method).
//   - homeDir: user home directory source (nil if unused).
//   - filesystem: fs.FS rooted at the config dir, used for reads (nil for write-only tests).
//   - writer: FileWriter used for atomic writes (nil if unused; tests use mock_config.FileWriter).
//
// Production code should use newGlobalConfigService, which wires real implementations.
func NewConfigService(
	gitProvider GitConfigProvider,
	homeDir HomeDirProvider,
	filesystem fs.FS,
	writer FileWriter,
) *ConfigService {
	return &ConfigService{
		gitProvider: gitProvider,
		homeDir:     homeDir,
		fs:          filesystem,
		writer:      writer,
	}
}

// configService is the global instance used by package-level functions.
// It is initialized with the real config dir as an os.DirFS so reads go to disk.
// Tests should create their own ConfigService (via NewConfigService) instead of relying on this global.
var configService = newGlobalConfigService()

// newGlobalConfigService initializes the global ConfigService with real dependencies
// and an os.DirFS rooted at the config dir. Paths passed to fs.FS are relative to it.
func newGlobalConfigService() *ConfigService {
	configDir, err := GetConfigDir()
	if err != nil {
		panic(fmt.Sprintf("failed to init global config service: %v", err))
	}
	return NewConfigService(&RealGitConfig{}, &RealHomeDir{}, os.DirFS(configDir), AtomicFileWriter{})
}

// LoadProfile loads a profile by name using the configured filesystem.
// Paths are relative to the config dir (e.g. "profiles/perso.yaml").
// The fs field must be set via NewConfigService; a nil fs will panic.
func (s *ConfigService) LoadProfile(name string) (*Profile, error) {
	pathStr := path.Join(ProfilesDirName, name+".yaml")
	data, err := fs.ReadFile(s.fs, pathStr)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile %q: %w", name, err)
	}

	var profile Profile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile %q: %w", name, err)
	}
	profile.path = pathStr

	return &profile, nil
}

// ListProfiles returns all available profile names using the configured filesystem.
// It reads the profiles directory relative to the config dir.
// The fs field must be set via NewConfigService; a nil fs will panic.
func (s *ConfigService) ListProfiles() ([]string, error) {
	readDirFS, ok := s.fs.(fs.ReadDirFS)
	if !ok {
		return nil, fmt.Errorf("configured filesystem does not implement fs.ReadDirFS")
	}
	entries, err := readDirFS.ReadDir(ProfilesDirName)
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

// GetCurrentProfile returns the currently selected profile from config.yaml.
// Reads config.yaml via the configured fs.FS (path relative to config dir).
// Returns empty string when config.yaml does not exist (no profile selected yet).
func (s *ConfigService) GetCurrentProfile() (string, error) {
	data, err := fs.ReadFile(s.fs, ConfigFileName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
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
// It writes config.yaml atomically via the injected FileWriter (fs.FS is read-only).
// Respects XDG_CONFIG_HOME.
func (s *ConfigService) SetCurrentProfile(name string) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	config := Config{CurrentProfile: name}
	data, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := s.writer.WriteFile(path, data, filePerm); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ProfileExists checks if a profile exists using the configured filesystem.
// It stats the profile path relative to the config dir.
// The fs field must be set via NewConfigService; a nil fs will panic.
func (s *ConfigService) ProfileExists(name string) (bool, error) {
	relPath := path.Join(ProfilesDirName, name+".yaml")
	statFS, ok := s.fs.(fs.StatFS)
	if !ok {
		return false, fmt.Errorf("configured filesystem does not implement fs.StatFS")
	}
	_, err := statFS.Stat(relPath)
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
func (s *ConfigService) GetActiveProfile(name string) (*Profile, error) {
	if name == "" {
		var err error
		name, err = s.GetCurrentProfile()
		if err != nil {
			return nil, fmt.Errorf("failed to get current profile: %w", err)
		}
		if name == "" {
			return nil, fmt.Errorf("no current profile selected\nRun 'cabin profile init' to create a default profile, or 'cabin profile use <name>' to select one")
		}
	}

	exists, err := s.ProfileExists(name)
	if err != nil {
		return nil, fmt.Errorf("failed to check profile existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("profile %q does not exist\nRun 'cabin profile init %s' to create it", name, name)
	}

	profile, err := s.LoadProfile(name)
	if err != nil {
		return nil, fmt.Errorf("failed to load profile %q: %w", name, err)
	}

	return profile, nil
}

// BuildDefaultProfile creates a Profile object without writing it to disk.
// This allows testing the profile construction logic separately from I/O.
func (s *ConfigService) BuildDefaultProfile(name string) (*Profile, error) {
	if name == "" {
		name = "default"
	}

	home, err := s.homeDir.GetHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Get Git user name from host config (mimics bootstrap-cabin.sh)
	gitAgentName := "AI Agent"
	gitAgentEmail := "ai-agent@localhost"

	// Try to get git config (best effort, may fail if git not configured)
	if name, err := s.gitProvider.GetUserName(); err == nil {
		if name != "" {
			gitAgentName = "AI Agent + " + name
		}
	}
	if email, err := s.gitProvider.GetUserEmail(); err == nil {
		if email != "" {
			gitAgentEmail = email
		}
	}

	profile := &Profile{
		Name: name,
		Vars: map[string]string{
			HomeVar:           home,
			DeskVar:           filepath.Join(home, "Documents", "desk"),
			WorkdirVar:        filepath.Join(home, "projects"),
			"GIT_AGENT_NAME":  gitAgentName,
			"GIT_AGENT_EMAIL": gitAgentEmail,
		},
	}

	return profile, nil
}

// InitProfile creates or overwrites a profile with a bounded set of resolved
// vars and returns the persisted profile (so the caller, `cabin profile
// init`, can read AI_CABIN_DESK to copy the desk skeleton).
//
// The persisted key set is bounded — never the whole env:
//   - defaults (BuildDefaultProfile: AI_CABIN_HOME/DESK/WORKDIR + GIT_AGENT_*)
//   - the --var keys (cliVars), which act as the initial `set` of the profile
//     CRUD and DO enlarge the set (e.g. --var AI_CABIN_DESK=/custom)
//   - on --force with an existing profile, the existing profile's keys
//
// Values are resolved with ResolveVars precedence (--var > env > existing >
// defaults): an env override on a bounded key (e.g. AI_CABIN_DESK in env) is
// picked up for the value, but env vars outside the bounded set are dropped
// (no PATH, no stray SCW_PROJECT_ID unless passed as --var).
//
// On a new profile (does not exist, force ignored): persisted = defaults ∪
// --var. On --force with an existing profile: persisted = defaults ∪ --var ∪
// existing. On an existing profile without --force: no-op, returns the existing
// profile (the CLI warns + exit 0, mirroring `cabin add`).
//
// The merge precedence mirrors ResolveVars (--var > env > existing > defaults)
// but is not delegated to it: ResolveVars loads the *selected* profile (axis A)
// and includes the whole env in its view, whereas InitProfile loads the named
// profile under creation/update and persists only a bounded key set (env
// overrides values but does not enlarge the set). The two share the same
// precedence rule by design; a future refactor could extract the shared merge
// if the bounded-set concern is factored out, but inlining keeps InitProfile
// readable and decoupled from the profile-selection axis.
func (s *ConfigService) InitProfile(name string, cliVars []string, force bool) (*Profile, error) {
	if name == "" {
		name = "default"
	}

	exists, err := s.ProfileExists(name)
	if err != nil {
		return nil, fmt.Errorf("check profile existence: %w", err)
	}
	if exists && !force {
		// No-op: return the existing profile so the CLI can skip the skeleton
		// copy too. The caller owns the warn + exit 0 UX (pattern cabin add).
		return s.LoadProfile(name)
	}

	// Defaults (the bounded base set).
	defaults, err := s.BuildDefaultProfile(name)
	if err != nil {
		return nil, err
	}

	// Build the persisted var map with ResolveVars precedence:
	// --var > env > existing (force) > defaults.
	persisted := make(Vars, len(defaults.Vars))
	setIfAbsent(persisted, defaults.Vars) // defaults first (lowest precedence)

	if force && exists {
		existing, err := s.LoadProfile(name)
		if err != nil {
			return nil, fmt.Errorf("load existing profile: %w", err)
		}
		setIfAbsent(persisted, existing.Vars) // existing wins over defaults
	}

	// Env: only the keys already in `persisted` (the bounded set) are updated
	// from env — env overrides default/existing values but does NOT enlarge the
	// set (no PATH stray). Precedence so far: env > existing > defaults.
	for k := range persisted {
		if v, ok := os.LookupEnv(k); ok {
			persisted[k] = v
		}
	}

	// --var (cliVars): highest precedence and DOES enlarge the set (a --var key
	// not in defaults/existing is added — the initial `set` of the CRUD).
	for _, kv := range cliVars {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --var %q: expected KEY=VAL", kv)
		}
		persisted[k] = v
	}

	profile := &Profile{Name: name, Vars: persisted}
	if err := s.SaveProfile(profile); err != nil {
		return nil, err
	}
	return profile, nil
}

// SaveProfile writes a profile to disk atomically via the injected FileWriter.
func (s *ConfigService) SaveProfile(profile *Profile) error {
	profilesDir, err := GetProfilesDir()
	if err != nil {
		return err
	}

	path := filepath.Join(profilesDir, profile.Name+".yaml")
	profile.path = path

	data, err := yaml.Marshal(profile)
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	if err := s.writer.WriteFile(path, data, filePerm); err != nil {
		return fmt.Errorf("failed to write profile file: %w", err)
	}

	return nil
}
