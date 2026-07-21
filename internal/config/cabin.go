package config

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// CabinsFileName is the cabins registry file in the config dir (name -> path).
// Orthogonal to profiles: a profile (env vars, git identity) does not reference
// cabins. This separation means adding/removing a cabin never touches a
// profile, and switching profiles never touches the cabin registry.
const CabinsFileName = "cabins.yaml"

// Cabin is a registered cabin: a name resolving to a path on disk.
type Cabin struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// CabinRegistry is the on-disk representation of a cabins.yaml file.
type CabinRegistry struct {
	Cabins []Cabin `yaml:"cabins"`
}

// getCabinsPath returns the absolute path to the common cabins.yaml file.
// Used by write paths; reads use fs.FS with the CabinsFileName relative path.
func getCabinsPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, CabinsFileName), nil
}

// cabinFileStore reads/writes a single cabins.yaml file.
// ConfigService constructs one per call (CLI is one-shot = 1 access per process).
//
// Reads go through fs (path relative to the config dir); writes go through the
// injected FileWriter (atomic). This keeps read/write symmetry with the rest of
// the package and allows mocking I/O errors in tests.
type cabinFileStore struct {
	relPath string     // CabinsFileName, relative to fs root
	absPath string     // absolute path, for writes via writer
	fs      fs.FS      // reads (nil-safe: callers set it)
	writer  FileWriter // atomic writes
}

// newCabinFileStore builds a store for the common cabins.yaml wired to the
// service's fs and writer.
func newCabinFileStore(absPath string, filesystem fs.FS, writer FileWriter) *cabinFileStore {
	return &cabinFileStore{
		relPath: CabinsFileName,
		absPath: absPath,
		fs:      filesystem,
		writer:  writer,
	}
}

// load reads the registry from fs. Returns an empty registry (not an error)
// when the file does not exist yet (no cabins registered).
func (s *cabinFileStore) load() (*CabinRegistry, error) {
	data, err := fs.ReadFile(s.fs, s.relPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &CabinRegistry{}, nil
		}
		return nil, fmt.Errorf("failed to read cabins file %q: %w", s.relPath, err)
	}

	var reg CabinRegistry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("failed to parse cabins file %q: %w", s.relPath, err)
	}
	return &reg, nil
}

// save writes the registry atomically via the injected FileWriter.
func (s *cabinFileStore) save(reg *CabinRegistry) error {
	data, err := yaml.Marshal(reg)
	if err != nil {
		return fmt.Errorf("failed to marshal cabins file: %w", err)
	}
	if err := s.writer.WriteFile(s.absPath, data, filePerm); err != nil {
		return fmt.Errorf("failed to write cabins file: %w", err)
	}
	return nil
}

// add upserts a cabin by name: load, replace if the name exists else append, save.
// Preserves other entries (unlike a blind overwrite).
func (s *cabinFileStore) add(name, cabinPath string) error {
	if name == "" {
		return fmt.Errorf("cabin name must not be empty")
	}

	reg, err := s.load()
	if err != nil {
		return err
	}

	for i, c := range reg.Cabins {
		if c.Name == name {
			reg.Cabins[i].Path = cabinPath
			return s.save(reg)
		}
	}
	reg.Cabins = append(reg.Cabins, Cabin{Name: name, Path: cabinPath})
	return s.save(reg)
}

// list returns the cabins in the registry (empty, non-nil slice if none).
func (s *cabinFileStore) list() ([]Cabin, error) {
	reg, err := s.load()
	if err != nil {
		return nil, err
	}
	if reg.Cabins == nil {
		return []Cabin{}, nil
	}
	return reg.Cabins, nil
}

// get returns the cabin registered under name, or ErrCabinNotFound if absent.
func (s *cabinFileStore) get(name string) (Cabin, error) {
	reg, err := s.load()
	if err != nil {
		return Cabin{}, err
	}
	for _, c := range reg.Cabins {
		if c.Name == name {
			return c, nil
		}
	}
	return Cabin{}, ErrCabinNotFound
}

// AddCabin adds or updates a cabin entry in the common registry.
// Writes go through the injected FileWriter (atomic temp + rename).
func (s *ConfigService) AddCabin(name, cabinPath string) error {
	absPath, err := getCabinsPath()
	if err != nil {
		return err
	}
	store := newCabinFileStore(absPath, s.fs, s.writer)
	return store.add(name, cabinPath)
}

// ListCabins returns the cabins registered in the common registry.
// Reads go through the configured fs.FS.
func (s *ConfigService) ListCabins() ([]Cabin, error) {
	absPath, err := getCabinsPath()
	if err != nil {
		return nil, err
	}
	store := newCabinFileStore(absPath, s.fs, s.writer)
	return store.list()
}

// GetCabin returns the cabin registered under name, or ErrCabinNotFound if
// the name is not in the registry. Reads go through the configured fs.FS.
func (s *ConfigService) GetCabin(name string) (Cabin, error) {
	absPath, err := getCabinsPath()
	if err != nil {
		return Cabin{}, err
	}
	store := newCabinFileStore(absPath, s.fs, s.writer)
	return store.get(name)
}

// AddCabin adds or updates a cabin entry in the common registry.
// It delegates to the global ConfigService.
func AddCabin(name, cabinPath string) error {
	return configService.AddCabin(name, cabinPath)
}

// GetCabin returns the cabin registered under name, or ErrCabinNotFound if
// the name is not in the registry.
func GetCabin(name string) (Cabin, error) {
	return configService.GetCabin(name)
}

// ListCabins returns the cabins registered in the common registry.
// It delegates to the global ConfigService.
func ListCabins() ([]Cabin, error) {
	return configService.ListCabins()
}
