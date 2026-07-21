package config

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

	mock_config "github.com/JulienVdG/AI-Cabin/internal/mocks/config"
	mock_fs "github.com/JulienVdG/AI-Cabin/internal/mocks/io/fs"
)

// newCabinStore builds a cabinFileStore for tests, using a real fs.FS (mapFS or
// os.DirFS) and an optional FileWriter (nil for read-only tests).
func newCabinStore(t *testing.T, filesystem fs.FS, writer FileWriter) *cabinFileStore {
	t.Helper()
	absPath, err := getCabinsPath()
	if err != nil {
		t.Fatalf("getCabinsPath() error = %v", err)
	}
	return newCabinFileStore(absPath, filesystem, writer)
}

// setupCabinTest sets XDG_CONFIG_HOME to a temp dir for the whitebox cabin tests
// (config_test.go's setupTestConfig lives in package config_test and is not
// accessible from this package config file).
func setupCabinTest(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
}

func TestCabinFileStore_Load(t *testing.T) {
	t.Run("returns empty registry when file does not exist", func(t *testing.T) {
		setupCabinTest(t)
		mapFS := fstest.MapFS{}
		store := newCabinStore(t, mapFS, nil)

		reg, err := store.load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if reg == nil {
			t.Fatal("Load() registry is nil, want non-nil empty registry")
		}
		if len(reg.Cabins) != 0 {
			t.Errorf("Load() cabins count = %d, want 0", len(reg.Cabins))
		}
	})

	t.Run("loads populated registry", func(t *testing.T) {
		setupCabinTest(t)
		mapFS := fstest.MapFS{
			CabinsFileName: &fstest.MapFile{
				Data: []byte("cabins:\n  - name: pi-go\n    path: /home/jvdg/AI-Cabin/cabin/pi-go\n  - name: blog\n    path: /home/jvdg/projects/blog\n"),
			},
		}
		store := newCabinStore(t, mapFS, nil)

		reg, err := store.load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if len(reg.Cabins) != 2 {
			t.Fatalf("Load() cabins count = %d, want 2", len(reg.Cabins))
		}
		if reg.Cabins[0].Name != "pi-go" {
			t.Errorf("Load() Cabins[0].Name = %q, want %q", reg.Cabins[0].Name, "pi-go")
		}
		if reg.Cabins[0].Path != "/home/jvdg/AI-Cabin/cabin/pi-go" {
			t.Errorf("Load() Cabins[0].Path = %q, want %q", reg.Cabins[0].Path, "/home/jvdg/AI-Cabin/cabin/pi-go")
		}
		if reg.Cabins[1].Name != "blog" {
			t.Errorf("Load() Cabins[1].Name = %q, want %q", reg.Cabins[1].Name, "blog")
		}
	})

	t.Run("returns error when yaml is invalid", func(t *testing.T) {
		setupCabinTest(t)
		mapFS := fstest.MapFS{
			CabinsFileName: &fstest.MapFile{
				Data: []byte("::: not valid yaml :::"),
			},
		}
		store := newCabinStore(t, mapFS, nil)

		_, err := store.load()
		if err == nil {
			t.Fatal("Load() expected error for invalid yaml, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse cabins file") {
			t.Errorf("Load() error = %q, should contain 'failed to parse cabins file'", err.Error())
		}
	})

	t.Run("returns error when filesystem fails", func(t *testing.T) {
		setupCabinTest(t)
		mockFS := &mock_fs.FS{}
		mockFS.On("Open", CabinsFileName).Return(nil, fmt.Errorf("disk error"))
		store := newCabinStore(t, mockFS, nil)

		_, err := store.load()
		if err == nil {
			t.Fatal("Load() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "disk error") {
			t.Errorf("Load() error = %q, should contain 'disk error'", err.Error())
		}
		mockFS.AssertExpectations(t)
	})
}

func TestCabinFileStore_List(t *testing.T) {
	t.Run("returns cabins from populated registry", func(t *testing.T) {
		setupCabinTest(t)
		mapFS := fstest.MapFS{
			CabinsFileName: &fstest.MapFile{
				Data: []byte("cabins:\n  - name: pi-go\n    path: /a/b\n"),
			},
		}
		store := newCabinStore(t, mapFS, nil)

		cabins, err := store.list()
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(cabins) != 1 {
			t.Errorf("List() count = %d, want 1", len(cabins))
		}
		if cabins[0].Name != "pi-go" {
			t.Errorf("List() cabins[0].Name = %q, want %q", cabins[0].Name, "pi-go")
		}
	})

	t.Run("returns empty slice when file does not exist", func(t *testing.T) {
		setupCabinTest(t)
		mapFS := fstest.MapFS{}
		store := newCabinStore(t, mapFS, nil)

		cabins, err := store.list()
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if cabins == nil {
			t.Error("List() returned nil slice, want non-nil empty")
		}
		if len(cabins) != 0 {
			t.Errorf("List() count = %d, want 0", len(cabins))
		}
	})
}

func TestCabinFileStore_Add(t *testing.T) {
	t.Run("adds a new cabin via atomic writer", func(t *testing.T) {
		setupCabinTest(t)
		// Real writer on real disk; fs is os.DirFS so reads see the write.
		configDir, _ := GetConfigDir()
		store := newCabinStore(t, os.DirFS(configDir), AtomicFileWriter{})

		if err := store.add("pi-go", "/home/jvdg/AI-Cabin/cabin/pi-go"); err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		// Verify file was written and readable.
		cabins, err := store.list()
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(cabins) != 1 {
			t.Fatalf("List() count after Add = %d, want 1", len(cabins))
		}
		if cabins[0].Name != "pi-go" || cabins[0].Path != "/home/jvdg/AI-Cabin/cabin/pi-go" {
			t.Errorf("Add() entry = {%q, %q}, want {%q, %q}",
				cabins[0].Name, cabins[0].Path, "pi-go", "/home/jvdg/AI-Cabin/cabin/pi-go")
		}
	})

	t.Run("upserts existing cabin by name", func(t *testing.T) {
		setupCabinTest(t)
		configDir, _ := GetConfigDir()
		// Seed with an existing entry (configDir must exist first).
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() config dir error = %v", err)
		}
		seed := "cabins:\n  - name: pi-go\n    path: /old/path\n  - name: blog\n    path: /blog/path\n"
		if err := os.WriteFile(filepath.Join(configDir, CabinsFileName), []byte(seed), 0o644); err != nil {
			t.Fatalf("WriteFile() seed error = %v", err)
		}
		store := newCabinStore(t, os.DirFS(configDir), AtomicFileWriter{})

		if err := store.add("pi-go", "/new/path"); err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		cabins, _ := store.list()
		if len(cabins) != 2 {
			t.Fatalf("List() count = %d, want 2 (upsert preserves other entries)", len(cabins))
		}
		// Find pi-go and verify path updated; blog must be untouched.
		var piGo, blog *Cabin
		for i := range cabins {
			if cabins[i].Name == "pi-go" {
				piGo = &cabins[i]
			}
			if cabins[i].Name == "blog" {
				blog = &cabins[i]
			}
		}
		if piGo == nil {
			t.Fatal("pi-go not found after upsert")
		}
		if piGo.Path != "/new/path" {
			t.Errorf("pi-go path = %q, want %q (updated)", piGo.Path, "/new/path")
		}
		if blog == nil {
			t.Fatal("blog not found after upsert (other entries must be preserved)")
		}
		if blog.Path != "/blog/path" {
			t.Errorf("blog path = %q, want %q (untouched)", blog.Path, "/blog/path")
		}
	})

	t.Run("rejects empty name", func(t *testing.T) {
		setupCabinTest(t)
		mapFS := fstest.MapFS{}
		store := newCabinStore(t, mapFS, AtomicFileWriter{})

		err := store.add("", "/some/path")
		if err == nil {
			t.Fatal("Add() expected error for empty name, got nil")
		}
		if !strings.Contains(err.Error(), "must not be empty") {
			t.Errorf("Add() error = %q, should contain 'must not be empty'", err.Error())
		}
	})

	t.Run("returns error when writer fails", func(t *testing.T) {
		setupCabinTest(t)
		mockWriter := &mock_config.FileWriter{}
		mockWriter.On("WriteFile",
			mock.Anything, // path
			mock.Anything, // data
			mock.Anything, // perm
		).Return(fmt.Errorf("disk write error"))
		// Empty fs: load returns empty registry, add appends one entry, save fails.
		store := newCabinStore(t, fstest.MapFS{}, mockWriter)

		err := store.add("pi-go", "/some/path")
		if err == nil {
			t.Fatal("Add() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "disk write error") {
			t.Errorf("Add() error = %q, should contain 'disk write error'", err.Error())
		}
		if !strings.Contains(err.Error(), "failed to write cabins file") {
			t.Errorf("Add() error = %q, should be wrapped by 'failed to write cabins file'", err.Error())
		}
		mockWriter.AssertExpectations(t)
	})
}

func TestConfigService_AddCabin(t *testing.T) {
	setupCabinTest(t)
	configDir, _ := GetConfigDir()
	svc := NewConfigService(nil, nil, os.DirFS(configDir), AtomicFileWriter{})

	if err := svc.AddCabin("pi-go", "/home/jvdg/AI-Cabin/cabin/pi-go"); err != nil {
		t.Fatalf("AddCabin() error = %v", err)
	}

	cabins, err := svc.ListCabins()
	if err != nil {
		t.Fatalf("ListCabins() error = %v", err)
	}
	if len(cabins) != 1 {
		t.Fatalf("ListCabins() count = %d, want 1", len(cabins))
	}
	if cabins[0].Name != "pi-go" {
		t.Errorf("ListCabins() cabins[0].Name = %q, want %q", cabins[0].Name, "pi-go")
	}
}

func TestConfigService_ListCabins(t *testing.T) {
	t.Run("returns empty when no cabins registered", func(t *testing.T) {
		setupCabinTest(t)
		mapFS := fstest.MapFS{}
		svc := NewConfigService(nil, nil, mapFS, nil)

		cabins, err := svc.ListCabins()
		if err != nil {
			t.Fatalf("ListCabins() error = %v", err)
		}
		if len(cabins) != 0 {
			t.Errorf("ListCabins() count = %d, want 0", len(cabins))
		}
	})

	t.Run("returns cabins from registry", func(t *testing.T) {
		setupCabinTest(t)
		mapFS := fstest.MapFS{
			CabinsFileName: &fstest.MapFile{
				Data: []byte("cabins:\n  - name: blog\n    path: /home/jvdg/projects/blog\n"),
			},
		}
		svc := NewConfigService(nil, nil, mapFS, nil)

		cabins, err := svc.ListCabins()
		if err != nil {
			t.Fatalf("ListCabins() error = %v", err)
		}
		if len(cabins) != 1 {
			t.Fatalf("ListCabins() count = %d, want 1", len(cabins))
		}
		if cabins[0].Name != "blog" {
			t.Errorf("ListCabins() cabins[0].Name = %q, want %q", cabins[0].Name, "blog")
		}
	})
}

func TestCabinFileStore_Get(t *testing.T) {
	setupCabinTest(t)
	configDir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error = %v", err)
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() config dir error = %v", err)
	}
	store := newCabinStore(t, os.DirFS(configDir), AtomicFileWriter{})

	t.Run("empty registry returns ErrCabinNotFound not disk error", func(t *testing.T) {
		// No cabins.yaml at all (fresh install): get returns ErrCabinNotFound,
		// not a disk error. The store's load() treats a missing file as an
		// empty registry (see cabinFileStore.load), so absence is normal.
		_, err := store.get("anything")
		if !errors.Is(err, ErrCabinNotFound) {
			t.Errorf("get() on empty registry error = %v, want ErrCabinNotFound", err)
		}
	})

	seed := "cabins:\n  - name: pi-go\n    path: /pi-go/path\n  - name: blog\n    path: /blog/path\n"
	if err := os.WriteFile(filepath.Join(configDir, CabinsFileName), []byte(seed), 0o644); err != nil {
		t.Fatalf("WriteFile() seed error = %v", err)
	}

	t.Run("returns existing cabin", func(t *testing.T) {
		c, err := store.get("blog")
		if err != nil {
			t.Fatalf("get(blog) error = %v", err)
		}
		if c.Name != "blog" || c.Path != "/blog/path" {
			t.Errorf("get(blog) = {%q, %q}, want {blog, /blog/path}", c.Name, c.Path)
		}
	})

	t.Run("missing name returns ErrCabinNotFound", func(t *testing.T) {
		_, err := store.get("ghost")
		if !errors.Is(err, ErrCabinNotFound) {
			t.Errorf("get(ghost) error = %v, want ErrCabinNotFound", err)
		}
	})
}
