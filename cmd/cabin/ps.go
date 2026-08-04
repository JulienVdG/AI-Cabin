package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/JulienVdG/AI-Cabin/internal/config"

	"github.com/spf13/cobra"
)

// agentServiceName is the compose service name running the agent in every
// cabin (v1 convention: cabins declare {{.AGENT_SERVICE}} as "agent"). `cabin
// ps` queries this service by name per registered cabin, so only agent
// containers appear. No labels are needed on auxiliary services.
const agentServiceName = "agent"

// psCmd lists running agent containers across all registered cabins. It is
// registry-driven: for each known cabin, it queries that cabin's compose
// project for the agent service (docker compose ps, CWD = cabin dir so the
// project name matches `cabin up`). Auxiliary services are not listed.
var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List running agent containers across registered cabins",
	Run: func(cmd *cobra.Command, args []string) {
		listRunningAgents(os.Stdout, os.Stderr)
	},
}

func init() {
	rootCmd.AddCommand(psCmd)
}

// composeContainer is the subset of `docker compose ps --format json` output
// `cabin ps` consumes: the service (filtered to the agent convention), the
// container name, and the running state.
type composeContainer struct {
	Service string `json:"Service"`
	Name    string `json:"Name"`
	State   string `json:"State"`
}

// listRunningAgents iterates registered cabins and prints each running agent
// container as "cabin-name  container-name  state". Cabins with no running
// agent are omitted. A cabin whose compose query fails (docker not running,
// no compose file) is reported to stderr but does not abort the listing of
// other cabins.
func listRunningAgents(stdout, stderr io.Writer) {
	cabins, err := config.ListCabins()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(cabins) == 0 {
		fmt.Fprintln(stdout, "No cabins registered. Add one with 'cabin cabin add <path> [name]'.")
		return
	}

	var any bool
	for _, c := range cabins {
		containers, err := queryAgentContainers(c.Path)
		if err != nil {
			fmt.Fprintf(stderr, "Warning: cabin %q: %v\n", c.Name, err)
			continue
		}
		for _, ct := range containers {
			if ct.State != "running" {
				continue
			}
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", c.Name, ct.Name, ct.State)
			any = true
		}
	}
	if !any {
		fmt.Fprintln(stdout, "No agent containers running.")
	}
}

// queryAgentContainers runs `docker compose ps --format json <agent>` with the
// cabin dir as CWD (so the compose project name matches `cabin up`, which runs
// docker compose in the cabin dir). Returns the agent service containers
// (any state); the caller filters to running.
func queryAgentContainers(cabinPath string) ([]composeContainer, error) {
	cmd := exec.Command("docker", "compose", "ps", "--format", "json", agentServiceName)
	cmd.Dir = cabinPath
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker compose ps: %w", err)
	}

	// `docker compose ps --format json` emits one JSON object per line
	// (NDJSON), not a JSON array. Decode line by line.
	var containers []composeContainer
	for _, line := range bytes.Split(out.Bytes(), []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ct composeContainer
		if err := json.Unmarshal(line, &ct); err != nil {
			return nil, fmt.Errorf("parse docker compose ps output: %w", err)
		}
		containers = append(containers, ct)
	}
	return containers, nil
}
