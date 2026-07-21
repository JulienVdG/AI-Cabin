package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/JulienVdG/AI-Cabin/internal/cabin"
	"github.com/JulienVdG/AI-Cabin/internal/config"

	"github.com/spf13/cobra"
)

// cabinCmd represents the cabin command (registry of cabins: name -> path).
var cabinCmd = &cobra.Command{
	Use:   "cabin",
	Short: "Manage the cabin registry",
	Long:  `The cabin registry maps cabin names to their path on disk (~/.config/ai-cabin/cabins.yaml).`,
}

// cabinAddCmd registers a cabin after validating it is a real cabin directory.
// Signature is "add <path> [name]": the path is primary (what makes a cabin),
// the name is derived (CLI arg > Taskfile ai-cabin.cabin > dir basename).
//
// UX (separation of concerns: internal/config does naive upsert, the CLI owns UX):
//   - name not in registry           -> AddCabin (new entry)
//   - name exists with SAME path     -> idempotent warning on stderr, exit 0
//     (so scripts re-running `add` don't fail on a clean re-run)
//   - name exists with DIFFERENT path -> error unless --force, exit non-zero
var cabinAddCmd = &cobra.Command{
	Use:   "add <path> [name]",
	Short: "Register or update a cabin",
	Long: `Add a cabin to the registry, or update its path if the name already exists.

The path must point to a directory containing a Taskfile.yml with an "ai-cabin:"
block (the cabin name defaults to the directory basename if not declared).
If [name] is omitted, the cabin name is taken from the Taskfile ai-cabin.cabin
field, or the directory basename as a last resort.

Re-adding a cabin already registered with the same path is idempotent (warning,
exit 0). Re-adding with a different path errors out unless --force is given.`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		cabinPath := args[0]
		var nameOverride string
		if len(args) == 2 {
			nameOverride = args[1]
		}

		name, normalizedPath, err := cabin.ValidateCabin(cabinPath, nameOverride)
		if err != nil {
			printValidateError(os.Stderr, err, normalizedPath)
			os.Exit(1)
		}

		existing, err := config.GetCabin(name)
		switch {
		case errors.Is(err, config.ErrCabinNotFound):
			// New entry: proceed to add.
		case err != nil:
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		case existing.Path == normalizedPath:
			// Idempotent re-add (same path): warn on stderr, exit 0 so scripts
			// re-running `add` don't fail on a clean re-run.
			fmt.Fprintf(os.Stderr, "Warning: cabin %q already registered at %s (no change)\n",
				name, normalizedPath)
			return
		case forceAdd:
			// Overwrite with a different path, explicit user consent.
			fmt.Fprintf(os.Stderr, "Warning: overwriting cabin %q (was: %s)\n",
				name, existing.Path)
		default:
			// Different path, no --force: refuse with an actionable message.
			fmt.Fprintf(os.Stderr,
				"Error: cabin %q already exists at %s; use --force to overwrite with %s\n",
				name, existing.Path, normalizedPath)
			os.Exit(1)
		}

		if err := config.AddCabin(name, normalizedPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Registered cabin %q -> %s\n", name, normalizedPath)
	},
}

// forceAdd is set by the --force flag on `cabin add`.
var forceAdd bool

// cabinListCmd represents the cabin list command.
var cabinListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered cabins",
	Run: func(cmd *cobra.Command, args []string) {
		cabins, err := config.ListCabins()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(cabins) == 0 {
			fmt.Println("No cabins registered. Add one with 'cabin cabin add <path> [name]'.")
			return
		}

		// Sort by name for stable output (registry preserves insertion order, but
		// the user expects alphabetical listing).
		sort.Slice(cabins, func(i, j int) bool {
			return cabins[i].Name < cabins[j].Name
		})

		for _, c := range cabins {
			fmt.Printf("%s -> %s\n", c.Name, c.Path)
		}
	},
}

func init() {
	cabinAddCmd.Flags().BoolVarP(&forceAdd, "force", "f", false,
		"overwrite an existing cabin registered with a different path")
	cabinCmd.AddCommand(cabinAddCmd)
	cabinCmd.AddCommand(cabinListCmd)
	rootCmd.AddCommand(cabinCmd)
}

// printValidateError renders a ValidateCabin error with actionable guidance.
// Sentinel errors (ErrNoHeader) get a richer message with a snippet; other
// errors (path/IO/yaml) are printed as-is since they already carry context.
// normalizedPath is the resolved cabin path ValidateCabin returns even on
// error (valid once the path has been resolved, before the header check).
func printValidateError(w io.Writer, err error, normalizedPath string) {
	if errors.Is(err, cabin.ErrNoHeader) {
		fmt.Fprintf(w, "Error: %s: %v\n\n", normalizedPath, err)
		fmt.Fprintln(w, "A cabin Taskfile must declare an ai-cabin: block, e.g.:")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "    ai-cabin:")
		fmt.Fprintln(w, "      cabin: <name>")
		return
	}
	fmt.Fprintf(w, "Error: %v\n", err)
}
