package config

import (
	"os/exec"
	"strings"
)

// GitConfigProvider defines the interface for accessing Git configuration.
// This interface allows mocking Git configuration in tests.
type GitConfigProvider interface {
	GetUserName() (string, error)
	GetUserEmail() (string, error)
}

// RealGitConfig implements GitConfigProvider by executing git config commands.
type RealGitConfig struct{}

// GetUserName returns the global Git user name.
func (g *RealGitConfig) GetUserName() (string, error) {
	out, err := exec.Command("git", "config", "--global", "user.name").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// GetUserEmail returns the global Git user email.
func (g *RealGitConfig) GetUserEmail() (string, error) {
	out, err := exec.Command("git", "config", "--global", "user.email").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
