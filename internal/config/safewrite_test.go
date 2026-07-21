package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/config"
)

func TestAtomicFileWriter_WriteFile(t *testing.T) {
	writer := config.AtomicFileWriter{}

	t.Run("creates file with content and perm", func(t *testing.T) {
		setupTestConfig(t)
		configDir, err := config.GetConfigDir()
		if err != nil {
			t.Fatalf("GetConfigDir() error = %v", err)
		}
		path := filepath.Join(configDir, "out.yaml")

		if err := writer.WriteFile(path, []byte("hello"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(data) != "hello" {
			t.Errorf("content = %q, want %q", string(data), "hello")
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		perm := info.Mode().Perm()
		if perm != 0o644 {
			t.Errorf("perm = %o, want %o", perm, 0o644)
		}
	})

	t.Run("creates parent dir when missing", func(t *testing.T) {
		setupTestConfig(t)
		configDir, err := config.GetConfigDir()
		if err != nil {
			t.Fatalf("GetConfigDir() error = %v", err)
		}
		// "profiles" subdir does not exist yet
		path := filepath.Join(configDir, "profiles", "perso.yaml")

		if err := writer.WriteFile(path, []byte("name: perso"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file at %q, got error: %v", path, err)
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		setupTestConfig(t)
		configDir, err := config.GetConfigDir()
		if err != nil {
			t.Fatalf("GetConfigDir() error = %v", err)
		}
		path := filepath.Join(configDir, "out.yaml")
		// Seed with old content
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		if err := writer.WriteFile(path, []byte("new"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		data, _ := os.ReadFile(path)
		if string(data) != "new" {
			t.Errorf("content = %q, want %q", string(data), "new")
		}
	})

	t.Run("leaves no temp file behind", func(t *testing.T) {
		setupTestConfig(t)
		configDir, err := config.GetConfigDir()
		if err != nil {
			t.Fatalf("GetConfigDir() error = %v", err)
		}
		path := filepath.Join(configDir, "out.yaml")

		if err := writer.WriteFile(path, []byte("hello"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		entries, err := os.ReadDir(configDir)
		if err != nil {
			t.Fatalf("ReadDir() error = %v", err)
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".out-tmp") {
				t.Errorf("temp file left behind: %s", e.Name())
			}
		}
	})
}
