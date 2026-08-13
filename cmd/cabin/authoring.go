package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/JulienVdG/AI-Cabin/internal/authoring"
	"github.com/JulienVdG/AI-Cabin/internal/cabin"
	"github.com/JulienVdG/AI-Cabin/internal/config"
	"github.com/JulienVdG/AI-Cabin/internal/fragments"
	"github.com/JulienVdG/AI-Cabin/internal/writestrategy"

	"github.com/spf13/cobra"
)

// authoringCmd groups the Class 2 cabin authoring commands. Unlike `cabin
// setup` (env bootstrap) and the Class 1 skeletons, authoring assists writing
// a cabin's own files (compose/Dockerfile/Taskfile) from the bundle blueprint
// facet. `show` renders the assembly to buffers (non-destructive), `new` writes
// the same assembly to new files only.
var authoringCmd = &cobra.Command{
	Use:   "authoring",
	Short: "Assemble a cabin (Dockerfile + compose + Taskfile) from blueprints",
	Long: `Assemble a near-complete cabin from the active bundles' blueprint facet.

Two subcommands share the same assembly engine: ` + "`authoring show`" + ` renders
the files to stdout (non-destructive), ` + "`authoring new`" + ` writes them to a
directory (new files only, never overwrites).`,
}

// authoringAgents and authoringFeatures select which bundles to assemble when
// the target path is not a cabin yet (no header); for an existing cabin the
// header drives the selection and these flags are ignored. authoringForce lets
// new overwrite existing files.
var (
	authoringAgents   string
	authoringFeatures string
	authoringForce    bool
)

// authoringShowCmd renders the assembled cabin files to stdout without writing
// anything. If the path holds a cabin (ai-cabin: header) the active bundles are
// derived from the header; otherwise the selection comes from --agents/--features
// (default: the full built-in catalogue).
var authoringShowCmd = &cobra.Command{
	Use:   "show <path>",
	Short: "Render the assembled cabin files to stdout (non-destructive)",
	Long: `Render the assembled cabin files (Dockerfile, docker-compose.yml,
Taskfile.yml) for a cabin at <path>, non-destructive (stdout only, never
writes or edits files).

If <path> is an existing cabin (a Taskfile with an ai-cabin: header), the
active bundles are derived from the header's agents:/features:. Otherwise the
selection comes from --agents/--features (default: the full built-in
catalogue), so the command also serves to preview a cabin for a project that
is not a cabin yet.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, err := resolveAuthoring(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		var df, cf, tf strings.Builder
		if err := authoring.Assemble(res.blueprints, res.selection, &authoring.Files{
			Dockerfile: &df,
			Compose:    &cf,
			Taskfile:   &tf,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: assemble: %v\n", err)
			os.Exit(1)
		}
		if res.aggregated != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", res.aggregated)
		}
		printFiles(os.Stdout, df.String(), cf.String(), tf.String())
	},
}

// authoringNewCmd writes the assembled cabin files to <dest>: the Dockerfile,
// docker-compose.yml and Taskfile.yml, new files only (never overwrites). The
// cabin name is the destination basename.
var authoringNewCmd = &cobra.Command{
	Use:   "new <dest>",
	Short: "Write the assembled cabin files to <dest> (new files only)",
	Long: `Write the assembled cabin files to <dest> (Dockerfile, docker-compose.yml,
Taskfile.yml), new files only — an existing file at the same path is skipped,
never overwritten, unless --force is given (then it is overwritten).

The cabin name is the destination basename (used for the compose
container_name/hostname). The feature selection comes from --agents/--features
(default: the full built-in catalogue), or from <dest>'s header when <dest> is
already a cabin.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		res, err := resolveAuthoring(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if res.aggregated != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", res.aggregated)
		}
		if err := os.MkdirAll(args[0], 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: create dest %q: %v\n", args[0], err)
			os.Exit(1)
		}
		var creator writestrategy.FileCreator = writestrategy.SkipCreator{}
		if authoringForce {
			creator = writestrategy.TruncateCreator{}
		}
		if err := writeNew(res.blueprints, res.selection, creator, args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// authoringResolution is the bundle resolution result: the assembled selection,
// the resolved blueprints, and any aggregated render warning.
type authoringResolution struct {
	selection  authoring.Selection
	blueprints []fragments.BundleBlueprint
	aggregated error
}

// cabinName resolves the compose container_name/hostname from the target path:
// the base of its real path, so "." maps to the current directory name rather
// than a literal "." that would break identifiers.
func cabinName(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", path, err)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return filepath.Base(abs), nil
}

// resolveAuthoring resolves the feature selection and blueprints for show/new:
// from the cabin header when <path> is a cabin, else from --agents/--features
// (default full catalogue). It also builds the fallback chain with the
// cabin-local override layer when present.
func resolveAuthoring(path string) (*authoringResolution, error) {
	vars, err := config.ResolveVars(profileFlag, cliVars)
	if err != nil {
		return nil, err
	}

	name, err := cabinName(path)
	if err != nil {
		return nil, err
	}
	sel := authoring.Selection{Name: name}
	var bundles []cabin.FeatureRef

	header, cabinPath, herr := cabin.Header(path)
	switch {
	case herr == nil:
		sel.Agents = header.Agents
		sel.Features = featureNames(header.Features)
		bundles = cabin.ActiveBundles(header)
	case errors.Is(herr, cabin.ErrNoHeader), errors.Is(herr, fs.ErrNotExist):
		sel.Agents, sel.Features = authoringSelection(authoringAgents, authoringFeatures)
		bundles = authoringBundles(sel)
	default:
		return nil, fmt.Errorf("%s: %w", path, herr)
	}

	merged, _, err := buildFragmentLayers(cabinPath, vars)
	if err != nil {
		return nil, err
	}

	blueprints := fragments.ResolveBlueprints(merged, bundles)

	var aggErr error
	for _, b := range blueprints {
		if b.Err != nil {
			aggErr = errors.Join(aggErr, fmt.Errorf("bundle %q: %w", b.Name, b.Err))
		}
	}
	return &authoringResolution{selection: sel, blueprints: blueprints, aggregated: aggErr}, nil
}

// featureNames maps header feature refs to their bundle names (a feature may
// carry attrs, e.g. port-forward; only the name drives assembly).
func featureNames(refs []cabin.FeatureRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Name)
	}
	return out
}

// authoringSelection returns the agents/features for a project that is not a
// cabin yet, defaulting to the full built-in catalogue when no filter is given.
func authoringSelection(agents, features string) ([]string, []string) {
	if agents == "" {
		agents = "pi,opencode"
	}
	if features == "" {
		features = "git-agent,go"
	}
	return commaList(agents), commaList(features)
}

// authoringBundles builds the feature refs (base + agents + features) for a
// project selection.
func authoringBundles(sel authoring.Selection) []cabin.FeatureRef {
	bundles := []cabin.FeatureRef{{Name: cabin.BaseBundle}}
	for _, a := range sel.Agents {
		bundles = append(bundles, cabin.FeatureRef{Name: "agent-" + a})
	}
	for _, f := range sel.Features {
		bundles = append(bundles, cabin.FeatureRef{Name: f})
	}
	return bundles
}

// commaList splits a comma-separated flag value, trimming whitespace and
// dropping empty entries. A nil/empty input yields nil.
func commaList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// printFiles prints the assembled cabin files to w, each under its filename
// header.
func printFiles(w io.Writer, dockerfile, compose, taskfile string) {
	parts := []struct {
		name string
		body string
	}{
		{authoring.CabinDockerfile, dockerfile},
		{"docker-compose.yml", compose},
		{"Taskfile.yml", taskfile},
	}
	for _, p := range parts {
		fmt.Fprintf(w, "=== %s ===\n", p.name)
		fmt.Fprint(w, p.body)
		if !strings.HasSuffix(p.body, "\n") {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
	}
}

// writeNew assembles the three cabin files into <dest>. Each file is created
// first via the write policy (SkipCreator skips an existing file; TruncateCreator
// overwrites, selected by --force), then the merged content is assembled straight
// into the created writers.
func writeNew(blueprints []fragments.BundleBlueprint, sel authoring.Selection, creator writestrategy.FileCreator, dest string) error {
	files := &authoring.Files{}
	names := []string{authoring.CabinDockerfile, "docker-compose.yml", "Taskfile.yml"}
	writers := []*io.Writer{&files.Dockerfile, &files.Compose, &files.Taskfile}
	for i := range writers {
		w, err := creator.Create(filepath.Join(dest, names[i]))
		if err != nil {
			if errors.Is(err, writestrategy.ErrSkip) {
				fmt.Printf("  skip  %s (exists)\n", names[i])
				continue
			}
			return fmt.Errorf("create %s: %w", names[i], err)
		}
		*writers[i] = w
	}
	if err := authoring.Assemble(blueprints, sel, files); err != nil {
		return err
	}
	var jerr error
	for i := range writers {
		if *writers[i] == nil {
			continue
		}
		if c, ok := (*writers[i]).(io.Closer); ok {
			if err := c.Close(); err != nil {
				jerr = errors.Join(jerr, fmt.Errorf("close %s: %w", names[i], err))
			}
		}
		fmt.Printf("  write %s\n", names[i])
	}
	return jerr
}

func init() {
	authoringShowCmd.Flags().StringVar(&authoringAgents, "agents", "", "agents to assemble when the path is not a cabin (pi,opencode)")
	authoringShowCmd.Flags().StringVar(&authoringFeatures, "features", "", "features to assemble when the path is not a cabin (git-agent,go)")
	authoringNewCmd.Flags().StringVar(&authoringAgents, "agents", "", "agents to assemble (pi,opencode)")
	authoringNewCmd.Flags().StringVar(&authoringFeatures, "features", "", "features to assemble (git-agent,go)")
	authoringNewCmd.Flags().BoolVar(&authoringForce, "force", false, "overwrite an existing file instead of skipping it")
	authoringCmd.AddCommand(authoringShowCmd)
	authoringCmd.AddCommand(authoringNewCmd)
	rootCmd.AddCommand(authoringCmd)
}
