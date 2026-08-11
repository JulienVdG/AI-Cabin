package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JulienVdG/AI-Cabin/internal/config"
	"github.com/JulienVdG/AI-Cabin/internal/embedded"
	"github.com/JulienVdG/AI-Cabin/internal/skeletons"
	"github.com/JulienVdG/AI-Cabin/internal/writestrategy"

	"github.com/spf13/cobra"
)

// skeletonCmd groups the Class 1 skeleton commands. A skeleton is a fragment
// with a mandatory skeleton.yaml manifest, resolved by name from
// AI_CABIN_SKELETON_DIRS + the embedded catalogue. The concern (desk vs
// project) drives the destination: desks target AI_CABIN_DESK, projects target
// $AI_CABIN_WORKDIR/<name>.
var skeletonCmd = &cobra.Command{
	Use:   "skeleton",
	Short: "Apply Class 1 skeletons (desk, project)",
	Long: `Apply a Class 1 skeleton by name, resolved from AI_CABIN_SKELETON_DIRS (a PATH-style list) + the embedded catalogue.

A desk skeleton (desk/<name>) targets AI_CABIN_DESK; a project skeleton (<name>=projects/<skeleton>) targets $AI_CABIN_WORKDIR/<name>. Project skeletons take per-instance attrs via --attr KEY=VAL (e.g. --attr module=github.com/me/myapp); the positional <name> populates the "project" attr.`,
}

// skeletonApplyForce overwrites existing destination files when set (--force).
// Without it, existing files are skipped (SkipCreator, the no-overwrite default).
var skeletonApplyForce bool

// skeletonApplyAttrs carries the per-instance --attr KEY=VAL overrides. For a
// project skeleton, "project" is set from the positional <name> (unless
// overridden); for a desk skeleton, attrs are unused (desks take none).
var skeletonApplyAttrs []string

// skeletonApplyCmd is the generic power-user form exposing the skeleton engine.
// It parses positionals of the form [desk=]desk/<skeleton> or
// <name>=projects/<skeleton>, resolves the skeleton by name from the union of
// AI_CABIN_SKELETON_DIRS + the embedded catalogue, and copies/templates it to
// the concern's default destination. Desks target AI_CABIN_DESK (the
// profile-init --skeleton wrapper is the adoption accelerator); projects target
// $AI_CABIN_WORKDIR/<name> and there is no dedicated `cabin project init`.
var skeletonApplyCmd = &cobra.Command{
	Use:   "apply [desk=]desk/<skeleton> [<name>=projects/<skeleton>]",
	Short: "Apply a Class 1 skeleton by name",
	Long: `Apply a Class 1 skeleton by name to its concern's default destination.

A positional carries a kind prefix and (for projects) a destination name:
  desk/<skeleton>          -> copy the desk skeleton to AI_CABIN_DESK
  <name>=projects/<skeleton> -> scaffold the project to $AI_CABIN_WORKDIR/<name>

The desk prefix is optional (a desk always targets AI_CABIN_DESK). One or both
forms may be passed; --attr KEY=VAL (repeatable) provides per-instance values
(the positional <name> populates "project"); --force overwrites existing files.`,
	Args: cobra.MinimumNArgs(1),
	// Positionals are completed from the skeleton catalogues by kind.
	ValidArgsFunction: completeSkeletonApply,
	Run: func(cmd *cobra.Command, args []string) {
		vars, err := config.ResolveVars(profileFlag, cliVars)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		merged, err := buildSkeletonLayers()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		creator := writestrategy.FileCreator(writestrategy.SkipCreator{})
		if skeletonApplyForce {
			creator = writestrategy.TruncateCreator{}
		}
		attrs := parseAttrs(skeletonApplyAttrs)

		for _, arg := range args {
			kind, skeleton, destName, err := parseSkeletonArg(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %q: %v\n", arg, err)
				os.Exit(1)
			}

			dest, skeletonRoot, applyAttrs, err := resolveSkeletonTarget(kind, skeleton, destName, vars, attrs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if _, err := fs.Stat(merged, skeletonRoot); err != nil {
				fmt.Fprintf(os.Stderr, "Error: skeleton %q not found in %s/ (available: %s)\n",
					skeleton, kind, strings.Join(listSkeletons(merged, kind), ", "))
				os.Exit(1)
			}

			if err := os.MkdirAll(dest, 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "Error: create dest %q: %v\n", dest, err)
				os.Exit(1)
			}

			written, err := skeletons.Apply(merged, skeletonRoot, dest, vars, applyAttrs, creator)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Applied %s/%s -> %s (%d files)\n", kind, skeleton, dest, len(written))
		}
	},
}

// skeletonKind is the catalogue a skeleton resolves from. The catalogue name
// matches the subdir under root/skeletons/ (desks/, projects/); the positional
// prefix ("desk/" or "projects/") maps to it.
type skeletonKind string

const (
	kindDesks    skeletonKind = "desks"
	kindProjects skeletonKind = "projects"
)

// parseSkeletonArg splits a positional into (kind, skeleton, destName).
//   - "desk/<skeleton>" or "<x>=desk/<skeleton>": kind=desks, destName empty
//     (a desk targets AI_CABIN_DESK, so no per-instance name).
//   - "<name>=projects/<skeleton>": kind=projects, destName=<name>.
//
// The desk prefix accepts "desk" (singular, matches the spec's desk/<skeleton>
// form) mapped to the "desks" catalogue. The project prefix is "projects"
// (plural, matches the catalogue). An unknown prefix or a projects arg without
// a <name>= prefix is an error.
func parseSkeletonArg(arg string) (kind skeletonKind, skeleton, destName string, err error) {
	// Split the optional <name>= prefix from the kind/skeleton part. Without
	// "=", the whole arg is kind/skeleton (the desk short form).
	head, kindPart, hasName := strings.Cut(arg, "=")
	ref := kindPart
	if !hasName {
		ref = arg
	}

	k, name, ok := strings.Cut(ref, "/")
	if !ok {
		return "", "", "", fmt.Errorf("expected [desk=]desk/<skeleton> or <name>=projects/<skeleton>")
	}
	switch k {
	case "desk":
		kind = kindDesks
		// destName stays empty: a desk always targets AI_CABIN_DESK, and the
		// optional <name>= prefix is a label ignored for desks.
	case "projects":
		kind = kindProjects
		if !hasName {
			return "", "", "", errors.New("projects skeleton requires a <name>= prefix (dest is $AI_CABIN_WORKDIR/<name>)")
		}
		destName = head
	default:
		return "", "", "", fmt.Errorf("unknown skeleton kind %q (want desk or projects)", k)
	}
	skeleton = name
	if skeleton == "" {
		return "", "", "", errors.New("empty skeleton name")
	}
	return kind, skeleton, destName, nil
}

// resolveSkeletonTarget maps a parsed (kind, skeleton, destName) to the
// destination path, the skeleton root in the merged FS, and the attrs to pass
// to Apply. For a project, the positional <name> populates the "project" attr
// (unless the caller already passed --attr project=...). For a desk, attrs are
// nil (desks take none).
func resolveSkeletonTarget(kind skeletonKind, skeleton, destName string, vars map[string]string, attrs map[string]string) (dest, skeletonRoot string, applyAttrs map[string]any, err error) {
	skeletonRoot = path.Join(string(kind), skeleton)
	switch kind {
	case kindDesks:
		dest = vars[config.DeskVar]
		if dest == "" {
			return "", "", nil, fmt.Errorf("AI_CABIN_DESK is not set in the profile")
		}
		return dest, skeletonRoot, nil, nil
	case kindProjects:
		if err := validateProjectName(destName); err != nil {
			return "", "", nil, fmt.Errorf("invalid project name %q: %w", destName, err)
		}
		dest = filepath.Join(vars[config.WorkdirVar], destName)
		if vars[config.WorkdirVar] == "" {
			return "", "", nil, fmt.Errorf("AI_CABIN_WORKDIR is not set in the profile")
		}
		applyAttrs = buildProjectAttrs(destName, attrs)
		return dest, skeletonRoot, applyAttrs, nil
	default:
		return "", "", nil, fmt.Errorf("unknown skeleton kind %q", kind)
	}
}

// buildProjectAttrs assembles the attrs for a project skeleton: the positional
// <name> populates "project" (unless the caller overrode it via --attr), and
// the remaining --attr overrides are copied in. Returned as map[string]any for
// render.Execute (top-level {{.project}}, {{.module}}).
func buildProjectAttrs(name string, overrides map[string]string) map[string]any {
	attrs := make(map[string]any, len(overrides)+1)
	attrs["project"] = name
	for k, v := range overrides {
		attrs[k] = v
	}
	return attrs
}

// parseAttrs parses --attr KEY=VAL flags into a map. A malformed entry (no "=")
// is reported; the caller exits on error. Empty input returns nil.
func parseAttrs(flags []string) map[string]string {
	if len(flags) == 0 {
		return nil
	}
	out := make(map[string]string, len(flags))
	for _, f := range flags {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: malformed --attr %q (expected KEY=VAL)\n", f)
			os.Exit(1)
		}
		out[k] = v
	}
	return out
}

// validateProjectName rejects a name that would escape the workdir or is
// otherwise unsafe as a path component: no separator, no "..", not absolute.
// The dest is filepath.Join(workdir, name), so a clean single component is the
// contract.
func validateProjectName(name string) error {
	if name == "" {
		return errors.New("empty")
	}
	if name == ".." || strings.Contains(name, "/") || strings.Contains(name, string(filepath.Separator)) {
		return errors.New("must be a single path component (no / or ..)")
	}
	if filepath.IsAbs(name) {
		return errors.New("must not be absolute")
	}
	return nil
}

// listSkeletons returns the sorted skeleton names available under a kind
// catalogue (desks/ or projects/) in the merged FS, for helpful errors and
// completion. A missing catalogue dir yields an empty slice (no skeletons of
// that kind in any layer).
func listSkeletons(merged fs.FS, kind skeletonKind) []string {
	entries, err := fs.ReadDir(merged, string(kind))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// buildSkeletonLayers builds the skeleton fallback chain (AI_CABIN_SKELETON_DIRS
// + the embedded catalogue) for apply and completion. Shared by skeleton apply,
// profile init --skeleton completion, and the apply positional completion.
func buildSkeletonLayers() (fs.FS, error) {
	vars, err := config.ResolveVars(profileFlag, cliVars)
	if err != nil {
		return nil, err
	}
	emb, err := embedded.Skeletons()
	if err != nil {
		return nil, err
	}
	return skeletons.BuildLayers(config.Vars(vars).SkeletonDirs(), emb)
}

// completeDeskSkeletonNames completes the --skeleton flag of `cabin profile
// init`: desk skeleton names resolved from AI_CABIN_SKELETON_DIRS + the
// embedded `minimal`. NoFileComp: a skeleton name is never an arbitrary file.
func completeDeskSkeletonNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	merged, err := buildSkeletonLayers()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var out []string
	for _, name := range listSkeletons(merged, kindDesks) {
		if strings.HasPrefix(name, toComplete) {
			out = append(out, name)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeSkeletonApply completes the positionals of `cabin skeleton apply`:
// each positional is a [desk=]desk/<skeleton> or <name>=projects/<skeleton>
// form. Completion offers the kind/name pairs found in the catalogues so the
// user can pick a skeleton, then type any <name>= prefix themselves. Returns
// NoFileComp (a skeleton name is never an arbitrary file).
func completeSkeletonApply(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	merged, err := buildSkeletonLayers()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var out []string
	for _, kind := range []skeletonKind{kindDesks, kindProjects} {
		for _, name := range listSkeletons(merged, kind) {
			var cand string
			switch kind {
			case kindDesks:
				cand = "desk/" + name
			case kindProjects:
				cand = "<name>=projects/" + name
			}
			if strings.HasPrefix(cand, toComplete) {
				out = append(out, cand)
			}
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	skeletonApplyCmd.Flags().BoolVarP(&skeletonApplyForce, "force", "f", false,
		"overwrite existing destination files")
	skeletonApplyCmd.Flags().StringArrayVar(&skeletonApplyAttrs, "attr", nil,
		"per-instance attr KEY=VAL (repeatable; project skeletons, e.g. --attr module=github.com/me/myapp)")
	skeletonCmd.AddCommand(skeletonApplyCmd)
	rootCmd.AddCommand(skeletonCmd)
}
