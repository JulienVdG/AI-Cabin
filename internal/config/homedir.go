package config

import "os"

// HomeDirProvider provides the user's home directory.
// This interface allows mocking in tests.
type HomeDirProvider interface {
	GetHomeDir() (string, error)
}

// RealHomeDir implements HomeDirProvider using os.UserHomeDir().
type RealHomeDir struct{}

// GetHomeDir returns the user's home directory.
func (h *RealHomeDir) GetHomeDir() (string, error) {
	return os.UserHomeDir()
}
