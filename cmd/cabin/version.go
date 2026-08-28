package main

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// versionInfo holds the build-derived version facts for display.
type versionInfo struct {
	Tag      string // the git tag when the revision is exactly a tag (VCS stamp)
	Module   string // the module version for a `go install @VERSION` build
	Revision string // the VCS revision (git sha) when built from a checkout
	Time     string // the VCS revision time
	Modified bool   // whether the working tree was dirty at build time
}

// readVersion extracts version facts from the Go build info embedded in the
// binary. The tag comes from the VCS stamping (git tag at the exact revision),
// the module version from `go install @VERSION`. Together they describe exactly
// which commit — and under which release name — this binary was built from.
func readVersion() versionInfo {
	var v versionInfo
	if bi, ok := debug.ReadBuildInfo(); ok {
		v.Module = bi.Main.Version
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.version":
				v.Tag = s.Value
			case "vcs.revision":
				v.Revision = s.Value
			case "vcs.time":
				v.Time = s.Value
			case "vcs.modified":
				v.Modified = s.Value == "true"
			}
		}
	}
	return v
}

// versionCmd prints the cabin version and, when built from a checkout, the
// VCS facts that pin the exact source revision.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the cabin version and build info",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		v := readVersion()

		// The tag wins (it is the release name), then the module version from
		// `go install @VERSION`, then devel for a plain local build.
		switch {
		case v.Tag != "":
			fmt.Printf("cabin %s\n", v.Tag)
		case v.Module != "" && v.Module != "(devel)":
			fmt.Printf("cabin %s\n", v.Module)
		default:
			fmt.Println("cabin (devel)")
		}

		if v.Revision != "" {
			fmt.Printf("revision %s\n", v.Revision)
			if v.Time != "" {
				fmt.Printf("build date %s\n", v.Time)
			}
			fmt.Printf("modified %v\n", v.Modified)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
