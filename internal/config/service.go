package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

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
	// Resolve the absolute file path (the read above goes through fs with a
	// relative path, but Path() must surface a real, operator-facing absolute
	// path so output like `profile show`/`set` points the user at the exact
	// YAML to inspect or fix by hand). GetProfilesDir is host-resolved.
	profilesDir, err := GetProfilesDir()
	if err != nil {
		return nil, fmt.Errorf("resolve profiles dir: %w", err)
	}
	profile.path = filepath.Join(profilesDir, name+".yaml")

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

// GetActiveProfile resolves and loads the active profile. The selector follows
// the same precedence as ResolveVars: an explicit name (--profile) wins, then
// AI_CABIN_PROFILE env (set by `cabin setenv`, so the standalone `task` path
// selects the right profile), then the current profile from config.yaml.
// Returns a user-friendly error if the profile doesn't exist or can't be loaded.
func (s *ConfigService) GetActiveProfile(name string) (*Profile, error) {
	if name == "" {
		name = os.Getenv(ProfileEnvVar)
	}
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
//   - the layer vars (LayerVars: a layer's layer.yaml vars: block, when the
//     resolved AI_CABIN_LAYER_DIRS is non-empty — see the body)
//   - the --var keys (cliVars), which act as the initial `set` of the profile
//     CRUD and DO enlarge the set (e.g. --var AI_CABIN_DESK=/custom)
//   - on --force with an existing profile, the existing profile's keys
//
// Values are resolved with ResolveVars precedence (--var > env > existing >
// layer vars > defaults): an env override on a bounded key (e.g. AI_CABIN_DESK
// in env) is picked up for the value, but env vars outside the bounded set are
// dropped (no PATH, no stray SCW_PROJECT_ID unless passed as --var).
//
// On a new profile (does not exist, force ignored): persisted = defaults ∪
// layer vars ∪ --var. On --force with an existing profile: persisted =
// defaults ∪ layer vars ∪ --var ∪ existing. On an existing profile without
// --force: no-op, returns the existing profile (the CLI warns + exit 0,
// mirroring `cabin add`).
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

	// On --force with an existing profile, its keys form a persistence tier.
	var existing *Profile
	if force && exists {
		existing, err = s.LoadProfile(name)
		if err != nil {
			return nil, fmt.Errorf("load existing profile: %w", err)
		}
	}

	cliVarsMap, err := parseCLIVars(cliVars)
	if err != nil {
		return nil, err
	}
	env := EnvironMap()

	// The active layer dirs gate the layer vars tier, resolved with the var
	// precedence (--var > env > existing > defaults).
	activeLayerVar := ""
	if existing != nil {
		if v, ok := existing.Vars[LayerDirsEnvVar]; ok {
			activeLayerVar = v
		}
	}
	if v, ok := env[LayerDirsEnvVar]; ok {
		activeLayerVar = v
	}
	if v, ok := cliVarsMap[LayerDirsEnvVar]; ok {
		activeLayerVar = v
	}
	layerVars, err := LayerVars(SplitPathList(activeLayerVar))
	if err != nil {
		return nil, err
	}

	// Env overrides only the persisted bounded set (defaults, layer and existing
	// keys), never stray env vars (no PATH, no SCW_PROJECT_ID unless passed as
	// --var). Build that set, then the env slice restricted to it. The layer
	// dirs var is added explicitly — it is not a default, but an env-exported
	// value must persist when it activates a layer.
	bounded := make(Vars, 1+len(defaults.Vars)+len(layerVars))
	bounded[LayerDirsEnvVar] = ""
	if existing != nil {
		setIfAbsent(bounded, existing.Vars)
	}
	setIfAbsent(bounded, layerVars)
	setIfAbsent(bounded, defaults.Vars)
	envRestricted := make(Vars)
	for k := range bounded {
		if v, ok := env[k]; ok {
			envRestricted[k] = v
		}
	}

	// Build the persisted map with the same first-wins helper, highest priority
	// first: --var > env > existing > layer vars > defaults. --var also enlarges
	// the set (the initial `set` of the CRUD).
	persisted := make(Vars, len(bounded)+len(cliVarsMap))
	setIfAbsent(persisted, cliVarsMap)
	setIfAbsent(persisted, envRestricted)
	if existing != nil {
		setIfAbsent(persisted, existing.Vars)
	}
	setIfAbsent(persisted, layerVars)
	setIfAbsent(persisted, defaults.Vars)

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

// SetProfileVar sets a single variable on a profile and persists it atomically.
// The profile is resolved like GetActiveProfile: an empty name selects the
// current profile. It is the runtime continuation of the `--var` CRUD (of which
// `profile init --var` is the initial set). It returns the updated profile.
func (s *ConfigService) SetProfileVar(name, key, value string) (*Profile, error) {
	profile, err := s.GetActiveProfile(name)
	if err != nil {
		return nil, err
	}
	if profile.Vars == nil {
		profile.Vars = map[string]string{}
	}
	profile.Vars[key] = value
	if err := s.SaveProfile(profile); err != nil {
		return nil, fmt.Errorf("failed to save profile %q: %w", profile.Name, err)
	}
	return profile, nil
}
