package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/cabin"
)

// TestParseDockerPS covers the NDJSON decoding of `docker ps --format '{{json .}}'`
// output: multi-line, empty lines, and a malformed line (hard error).
func TestParseDockerPS(t *testing.T) {
	t.Run("multiple containers", func(t *testing.T) {
		data := []byte(`{"Names":"blog-agent-1","State":"running","Labels":{"com.docker.compose.service":"agent","com.docker.compose.config_files":"/home/u/blog/docker-compose.yml"}}
{"Names":"other-1","State":"exited","Labels":{"com.docker.compose.service":"db","com.docker.compose.config_files":"/home/u/blog/docker-compose.yml"}}
`)
		got, err := parseDockerPS(data)
		if err != nil {
			t.Fatalf("parseDockerPS error = %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[0].Names != "blog-agent-1" || got[0].State != "running" {
			t.Errorf("got[0] = %+v", got[0])
		}
		if got[1].State != "exited" {
			t.Errorf("got[1].State = %q, want exited", got[1].State)
		}
	})

	t.Run("labels as CSV string (Docker {{json .}} template form)", func(t *testing.T) {
		// Docker's `{{json .}}` template renders Labels as a CSV string
		// ("k=v,k2=v2"), not a JSON object — even on recent Docker versions.
		// This is the real shape observed from `docker ps --format '{{json .}}'`.
		// Uses the Compose v2 label name (com.docker.compose.project.config_files).
		data := []byte(`{"Names":"pi_go_agent","State":"running","Labels":"com.docker.compose.service=agent,com.docker.compose.project.config_files=/home/u/blog/docker-compose.yml"}
`)
		got, err := parseDockerPS(data)
		if err != nil {
			t.Fatalf("parseDockerPS error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		if got[0].Labels[labelComposeService] != "agent" {
			t.Errorf("service = %q, want agent", got[0].Labels[labelComposeService])
		}
		if got[0].Labels[labelComposeConfigFilesV2] != "/home/u/blog/docker-compose.yml" {
			t.Errorf("config_files = %q", got[0].Labels[labelComposeConfigFilesV2])
		}
	})

	t.Run("labels CSV with value containing equals sign", func(t *testing.T) {
		// A value may contain '=' (e.g. a URL); only the first '=' splits a pair.
		data := []byte(`{"Names":"a","State":"running","Labels":"foo=bar=baz,x=y"}
`)
		got, err := parseDockerPS(data)
		if err != nil {
			t.Fatalf("parseDockerPS error = %v", err)
		}
		if got[0].Labels["foo"] != "bar=baz" {
			t.Errorf("foo = %q, want bar=baz", got[0].Labels["foo"])
		}
		if got[0].Labels["x"] != "y" {
			t.Errorf("x = %q, want y", got[0].Labels["x"])
		}
	})

	t.Run("labels null", func(t *testing.T) {
		data := []byte(`{"Names":"a","State":"running","Labels":null}
`)
		got, err := parseDockerPS(data)
		if err != nil {
			t.Fatalf("parseDockerPS error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		if got[0].Labels != nil {
			t.Errorf("Labels = %v, want nil", got[0].Labels)
		}
	})

	t.Run("trailing newline and blank lines skipped", func(t *testing.T) {
		data := []byte("\n{\"Names\":\"a\",\"State\":\"running\"}\n\n\n")
		got, err := parseDockerPS(data)
		if err != nil {
			t.Fatalf("parseDockerPS error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got, err := parseDockerPS(nil)
		if err != nil {
			t.Fatalf("parseDockerPS error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})

	t.Run("malformed line is a hard error", func(t *testing.T) {
		data := []byte("{\"Names\":\"a\",\"State\":\"running\"}\n{not json}\n")
		_, err := parseDockerPS(data)
		if err == nil {
			t.Fatal("parseDockerPS error = nil, want error for malformed line")
		}
	})
}

// TestComposeLabelsOf covers the extraction of the Docker Compose labels:
// missing labels, missing config_files, and the happy path.
func TestComposeLabelsOf(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		ct := dockerContainer{Labels: map[string]string{
			labelComposeService:       "agent",
			labelComposeConfigFilesV2: "/home/u/blog/docker-compose.yml",
		}}
		got, ok := composeLabelsOf(ct)
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if got.service != "agent" {
			t.Errorf("service = %q, want agent", got.service)
		}
		if got.configFiles != "/home/u/blog/docker-compose.yml" {
			t.Errorf("configFiles = %q", got.configFiles)
		}
	})

	t.Run("nil labels", func(t *testing.T) {
		_, ok := composeLabelsOf(dockerContainer{})
		if ok {
			t.Error("ok = true, want false for nil labels")
		}
	})

	t.Run("missing config_files label", func(t *testing.T) {
		ct := dockerContainer{Labels: map[string]string{
			labelComposeService: "agent",
		}}
		_, ok := composeLabelsOf(ct)
		if ok {
			t.Error("ok = true, want false when config_files is absent")
		}
	})

	t.Run("empty config_files label", func(t *testing.T) {
		ct := dockerContainer{Labels: map[string]string{
			labelComposeConfigFilesV2: "",
		}}
		_, ok := composeLabelsOf(ct)
		if ok {
			t.Error("ok = true, want false for empty config_files")
		}
	})

	t.Run("legacy v1 config_files label", func(t *testing.T) {
		// Compose v1 (legacy) used com.docker.compose.config_files (no
		// ".project." prefix). Both names are supported so `cabin ps` works
		// across Compose versions; v2 takes precedence.
		ct := dockerContainer{Labels: map[string]string{
			labelComposeService:       "agent",
			labelComposeConfigFilesV1: "/home/u/blog/docker-compose.yml",
		}}
		got, ok := composeLabelsOf(ct)
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if got.configFiles != "/home/u/blog/docker-compose.yml" {
			t.Errorf("configFiles = %q", got.configFiles)
		}
	})
}

// TestCabinDirFromConfigFiles covers deriving the cabin dir from the
// com.docker.compose.config_files label: single file and multiple files.
func TestCabinDirFromConfigFiles(t *testing.T) {
	t.Run("single compose file", func(t *testing.T) {
		got := cabinDirFromConfigFiles("/home/u/blog/docker-compose.yml")
		if got != "/home/u/blog" {
			t.Errorf("got %q, want /home/u/blog", got)
		}
	})

	t.Run("multiple comma-separated files", func(t *testing.T) {
		// Docker Compose may list overrides; the first is the primary file.
		got := cabinDirFromConfigFiles("/home/u/blog/docker-compose.yml,/home/u/blog/docker-compose.override.yml")
		if got != "/home/u/blog" {
			t.Errorf("got %q, want /home/u/blog", got)
		}
	})
}

// TestMapContainersToAgents covers the filtering and resolution logic with a
// real temp dir + Taskfile (registered cabin) and a non-existent dir
// (skipped). The running-state filter runs first; non-compose containers and
// non-agent services are skipped; registered cabins use the registry name.
func TestMapContainersToAgents(t *testing.T) {
	// Registered cabin: a real temp dir with a valid ai-cabin header and an
	// AGENT_SERVICE var so agentServiceForCabin reads it.
	blogDir := t.TempDir()
	writeTaskfileMain(t, blogDir, "ai-cabin: {}\nvars:\n  AGENT_SERVICE: agent\n")
	// The registry stores the canonical path (EvalSymlinks, like ValidateCabin).
	blogCanonical, err := filepath.EvalSymlinks(blogDir)
	if err != nil {
		t.Fatalf("EvalSymlinks blogDir: %v", err)
	}
	registry := map[string]string{blogCanonical: "blog"}
	blogCompose := filepath.Join(blogDir, "docker-compose.yml")

	containers := []dockerContainer{
		{
			Names: "blog-agent-1", State: "running",
			Labels: map[string]string{
				labelComposeService:       "agent",
				labelComposeProject:       "default_blog",
				labelComposeConfigFilesV2: blogCompose,
			},
		},
		{
			Names: "blog-db-1", State: "running",
			Labels: map[string]string{
				labelComposeService:       "db",
				labelComposeProject:       "default_blog",
				labelComposeConfigFilesV2: blogCompose,
			},
		},
		{
			Names: "blog-agent-1", State: "exited",
			Labels: map[string]string{
				labelComposeService:       "agent",
				labelComposeProject:       "blog",
				labelComposeConfigFilesV2: blogCompose,
			},
		},
		{
			Names: "ghost-1", State: "running",
			Labels: map[string]string{
				labelComposeService:       "agent",
				labelComposeProject:       "custom",
				labelComposeConfigFilesV2: "/nonexistent/cabin/docker-compose.yml",
			},
		},
		{
			Names: "raw-1", State: "running",
			Labels: map[string]string{},
		},
	}

	t.Run("default: running agent only", func(t *testing.T) {
		// Only the running agent of the registered cabin matches. The db service,
		// the exited agent, the ghost (dir does not exist), and the raw container
		// (no compose labels) are all skipped. The profile is derived from the
		// project label: "default_blog" -> "default".
		got := mapContainersToAgents(containers, registry, false)
		want := []agentRow{
			{name: "blog", profile: "default", container: "blog-agent-1", state: "running"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("--all: include stopped agent", func(t *testing.T) {
		// The exited agent carries the canonical-only project "blog" (no profile
		// selected on that standalone run), so its profile is empty.
		got := mapContainersToAgents(containers, registry, true)
		want := []agentRow{
			{name: "blog", profile: "default", container: "blog-agent-1", state: "running"},
			{name: "blog", profile: "", container: "blog-agent-1", state: "exited"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
}

// writeTaskfileMain writes a Taskfile into dir (test helper for the main
// package, mirroring internal/cabin's writeTaskfile).
func writeTaskfileMain(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, cabin.TaskfileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write Taskfile: %v", err)
	}
}
