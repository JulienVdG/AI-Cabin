package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/JulienVdG/AI-Cabin/internal/cabin"
	"github.com/JulienVdG/AI-Cabin/internal/config"

	"github.com/spf13/cobra"
)

// Docker Compose labels set on every container it creates. `cabin ps` reads
// them to resolve a container to its cabin (config_files -> cabin dir) and to
// identify the agent service (service), without relying on a hardcoded service
// name or the registry alone. The project label carries the compose project
// name (<profile>_<cabin> or <cabin> alone) from which the active profile is
// derived.
//
// Compose v2 prefixes several labels with ".project."; the config_files label
// is "com.docker.compose.project.config_files" on v2 and "com.docker.compose.
// config_files" on legacy v1. Both are checked so `cabin ps` works across
// Compose versions. The service and project labels are the same on both.
const (
	labelComposeService       = "com.docker.compose.service"
	labelComposeProject       = "com.docker.compose.project"
	labelComposeConfigFilesV2 = "com.docker.compose.project.config_files"
	labelComposeConfigFilesV1 = "com.docker.compose.config_files"
)

// psCmd lists agent containers. Discovery is label-driven: it queries
// `docker ps` for all containers, keeps those created by Docker Compose (they
// carry com.docker.compose.* labels), and resolves each to a cabin via its
// compose config_files path. The cabin name comes from the registry when the
// dir is registered (fast path), or from Taskfile header validation otherwise
// (unregistered cabins are discovered too). The agent container is the one
// whose com.docker.compose.service matches the cabin's AGENT_SERVICE (default
// "agent"). Only running containers are listed unless --all is set.
var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List agent containers across cabins",
	Run: func(cmd *cobra.Command, args []string) {
		listRunningAgents(os.Stdout, os.Stderr, allFlag)
	},
}

// allFlag (-a, --all) shows stopped containers too (default: running only).
// Matches the `docker ps -a` convention: a stopped agent is useful when
// debugging why a cabin does not respond (exited/restarting vs absent).
var allFlag bool

func init() {
	psCmd.Flags().BoolVarP(&allFlag, "all", "a", false, "show all containers (default: running only)")
	rootCmd.AddCommand(psCmd)
}

// dockerContainer is the subset of `docker ps --format '{{json .}}'` output
// `cabin ps` consumes: the container name, the running state, and the labels
// (Docker Compose sets com.docker.compose.service / .config_files / .project).
//
// Labels shape varies across Docker versions: Docker 25+ (`--format json`)
// emits a JSON object, while older versions and the `{{json .}}` template emit
// a CSV string ("k=v,k2=v2"). dockerLabels unmarshals both forms into a map.
type dockerContainer struct {
	Names  string       `json:"Names"`
	State  string       `json:"State"`
	Labels dockerLabels `json:"Labels"`
}

// dockerLabels unmarshals the Labels field in either of Docker's two shapes:
//   - a JSON object (Docker 25+ `--format json`): {"key": "value", ...}
//   - a CSV string (older Docker, or the `{{json .}}` template):
//     "key=value,key2=value2"
//
// The CSV form is the one Docker Compose labels take: keys/values use '=' as
// the separator and ',' between pairs. Values may contain '=' (e.g. URLs);
// only the first '=' splits a pair, the rest is the value. Values do not
// contain ',' in practice for the compose labels `cabin ps` reads.
type dockerLabels map[string]string

// UnmarshalJSON implements the dual-shape parsing. An empty/absent label set
// yields a nil map (a container with no labels is a valid state).
func (l *dockerLabels) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	// Object form: decode into a map directly.
	if trimmed[0] == '{' {
		var m map[string]string
		if err := json.Unmarshal(trimmed, &m); err != nil {
			return err
		}
		*l = m
		return nil
	}
	// String form: parse the CSV "k=v,k2=v2". Unquote first (the value is a
	// JSON string); an empty string means no labels.
	var s string
	if err := json.Unmarshal(trimmed, &s); err != nil {
		return err
	}
	*l = parseLabelsCSV(s)
	return nil
}

// parseLabelsCSV parses the Docker CSV label form ("k=v,k2=v2") into a map.
// Only the first '=' splits a pair so values may contain '='. An empty input
// yields nil. Pairs with no '=' are skipped (malformed, unseen in practice).
func parseLabelsCSV(s string) map[string]string {
	if s == "" {
		return nil
	}
	m := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		// Only the first '=' splits: a value like "a=b=c" keeps "b=c" as the value.
		idx := strings.IndexByte(pair, '=')
		if idx < 0 {
			continue
		}
		m[pair[:idx]] = pair[idx+1:]
	}
	return m
}

// agentRow is one line of the `cabin ps` output: the cabin name, the active
// profile (derived from the compose project label; empty when the instance
// carries no profile), the container name, and the state.
type agentRow struct {
	name      string
	profile   string
	container string
	state     string
}

// listRunningAgents queries docker for all containers, filters to Docker
// Compose containers that resolve to an AI-Cabin cabin and match the agent
// service, and prints them as "cabin-name\tcontainer-name\tstate". Containers
// whose cabin dir is not a valid AI-Cabin cabin (no ai-cabin: header) are
// skipped silently. A docker failure is fatal; a registry load failure is
// reported but does not abort (label discovery still resolves unregistered
// cabins via Taskfile validation).
func listRunningAgents(stdout, stderr io.Writer, all bool) {
	// Load the registry as a path->name map (fast lookup for registered cabins).
	registry, err := registryByPath()
	if err != nil {
		fmt.Fprintf(stderr, "Warning: could not load cabin registry: %v\n", err)
		registry = map[string]string{}
	}

	output, err := runDockerPS()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	containers, err := parseDockerPS(output)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	rows := mapContainersToAgents(containers, registry, all)

	if len(rows) == 0 {
		if all {
			fmt.Fprintln(stdout, "No agent containers found.")
		} else {
			fmt.Fprintln(stdout, "No agent containers running.")
		}
		return
	}
	fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", "CABIN", "PROFILE", "CONTAINER", "STATE")
	for _, r := range rows {
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", r.name, r.profile, r.container, r.state)
	}
}

// mapContainersToAgents filters and resolves Docker Compose containers to
// agent rows. The running-state filter runs first (skip stopped containers
// unless all=true) so the cabin resolution (registry lookup + Taskfile read)
// is not done for containers that would be omitted anyway. Containers without
// the config_files label (not from Docker Compose) or not resolving to an
// AI-Cabin cabin are skipped silently.
func mapContainersToAgents(containers []dockerContainer, registry map[string]string, all bool) []agentRow {
	var rows []agentRow
	for _, ct := range containers {
		// Filter running first: avoids resolving cabin dirs for stopped
		// containers that would be omitted by default.
		if !all && ct.State != "running" {
			continue
		}
		labels, ok := composeLabelsOf(ct)
		if !ok {
			continue // not a Docker Compose container
		}
		dir := cabinDirFromConfigFiles(labels.configFiles)
		name, ok := resolveCabinName(dir, registry)
		if !ok {
			continue // not an AI-Cabin cabin (no ai-cabin: header)
		}
		agentSvc := agentServiceForCabin(dir)
		if labels.service != agentSvc {
			continue // not the agent service (e.g. an auxiliary service)
		}
		rows = append(rows, agentRow{
			name:      name,
			profile:   cabin.DeriveProfile(labels.project, name),
			container: ct.Names,
			state:     ct.State,
		})
	}
	return rows
}

// parseDockerPS decodes the NDJSON output of `docker ps --format '{{json .}}'`
// (one JSON object per line) into a slice of dockerContainer. Empty lines are
// skipped. A malformed line is a hard error: docker output is machine-generated,
// and a parse failure signals a real mismatch (e.g. a Docker version producing
// a different shape).
func parseDockerPS(data []byte) ([]dockerContainer, error) {
	var containers []dockerContainer
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ct dockerContainer
		if err := json.Unmarshal(line, &ct); err != nil {
			return nil, fmt.Errorf("parse docker ps output: %w", err)
		}
		containers = append(containers, ct)
	}
	return containers, nil
}

// composeLabelsOf extracts the Docker Compose labels `cabin ps` reads off a
// container. Returns ok=false when the container has no labels or no
// com.docker.compose.config_files label (i.e. not created by Docker Compose).
func composeLabelsOf(ct dockerContainer) (composeLabels, bool) {
	if ct.Labels == nil {
		return composeLabels{}, false
	}
	cfg := configFilesLabel(ct.Labels)
	if cfg == "" {
		return composeLabels{}, false
	}
	return composeLabels{
		service:     ct.Labels[labelComposeService],
		project:     ct.Labels[labelComposeProject],
		configFiles: cfg,
	}, true
}

// configFilesLabel reads the compose config_files label from a container's
// labels, trying the Compose v2 name first (com.docker.compose.project.
// config_files) then the legacy v1 name (com.docker.compose.config_files).
// Returns "" when neither is present (not a Docker Compose container).
func configFilesLabel(labels map[string]string) string {
	if v := labels[labelComposeConfigFilesV2]; v != "" {
		return v
	}
	return labels[labelComposeConfigFilesV1]
}

// composeLabels holds the Docker Compose labels `cabin ps` reads off each
// container to resolve it to a cabin and identify the agent service.
type composeLabels struct {
	service     string // com.docker.compose.service (e.g. "agent")
	project     string // com.docker.compose.project (<profile>_<cabin> or <cabin>)
	configFiles string // com.docker.compose.config_files (absolute path(s), comma-separated)
}

// cabinDirFromConfigFiles derives the cabin directory (where the Taskfile
// lives) from the com.docker.compose.config_files label. The label holds the
// absolute path to the compose file(s); the cabin dir is the parent of the
// first one. Docker Compose may list multiple comma-separated paths (extends /
// overrides); the first is the primary compose file, and its parent is where
// `cabin up` ran (the cabin dir).
func cabinDirFromConfigFiles(configFiles string) string {
	first := strings.SplitN(configFiles, ",", 2)[0]
	return filepath.Dir(first)
}

// registryByPath loads the cabin registry and indexes it by canonical path
// (the form ValidateCabin stores) for an O(1) lookup during discovery.
func registryByPath() (map[string]string, error) {
	cabins, err := config.ListCabins()
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(cabins))
	for _, c := range cabins {
		m[c.Path] = c.Name
	}
	return m, nil
}

// resolveCabinName resolves a cabin directory to its name: registry lookup by
// canonical path first (fast, no Taskfile read), then Taskfile header
// validation (derives the name and confirms it is an AI-Cabin cabin). Returns
// ok=false when the dir is not an AI-Cabin cabin (no ai-cabin: header) — the
// container is skipped. A path that cannot be normalized is also skipped
// rather than aborting the whole listing.
func resolveCabinName(dir string, registry map[string]string) (string, bool) {
	// Normalize the dir the same way ValidateCabin does (absolute + symlinks)
	// so it matches the registry's canonical paths.
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", false
	}
	if name, ok := registry[resolved]; ok {
		return name, true
	}
	// Unregistered: validate it is an AI-Cabin cabin (header present) and
	// derive the name. ErrNoHeader means it is not a cabin — skip silently.
	name, _, err := cabin.ValidateCabin(dir, "")
	if err != nil {
		return "", false
	}
	return name, true
}

// agentServiceForCabin reads the cabin's AGENT_SERVICE Taskfile var (default
// "agent") to match the agent container via its com.docker.compose.service
// label. A read error falls back to the default rather than dropping the cabin
// from the listing (a missing Taskfile is unlikely here since resolveCabinName
// just validated it, but the fallback keeps the function defensive).
func agentServiceForCabin(dir string) string {
	tfPath := filepath.Join(dir, cabin.TaskfileName)
	data, err := os.ReadFile(tfPath)
	if err != nil {
		return cabin.DefaultAgentService
	}
	return cabin.AgentService(data)
}

// runDockerPS queries all containers as JSON (one object per line) via the
// Docker CLI. Uses the Go template form '{{json .}}' for portability: the
// 'json' format shorthand requires Docker 25+, while the template form works
// on older versions too. Requires Docker to be installed and reachable.
func runDockerPS() ([]byte, error) {
	cmd := exec.Command("docker", "ps", "--format", "{{json .}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}
	return out.Bytes(), nil
}
