package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/JulienVdG/AI-Cabin/internal/config"

	"github.com/spf13/cobra"
)

// setenvCmd outputs the resolved profile/environment variables in a shell
// evaluable form, mirroring `cabin completion <shell>`. It has no action of its
// own: running `cabin setenv` prints the Cobra help listing the supported
// shells, one subcommand per shell, each with its sourcing instructions in
// --help. The profile is resolved from --profile, then AI_CABIN_PROFILE, then
// the current profile.
var setenvCmd = &cobra.Command{
	Use:   "setenv",
	Short: "Output environment variables for your shell",
	Long: `Output the resolved profile variables in a form your shell can load.

Choose a shell below to print the variables in that shell's syntax; see each
sub-command's --help for how to source it. The profile is resolved from
--profile, then AI_CABIN_PROFILE, then the current profile.`,
	Args: cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(setenvCmd)
	setenvCmd.AddCommand(
		setenvShell("bash", "Output environment variables for bash",
			[]string{"source <(cabin setenv bash)", "eval  \"$(cabin setenv bash)\""},
			".bashrc", "eval \"$(cabin setenv bash <name>)\""),
		setenvShell("zsh", "Output environment variables for zsh",
			[]string{"source <(cabin setenv zsh)", "eval  \"$(cabin setenv zsh)\""},
			".zshrc", "eval \"$(cabin setenv zsh <name>)\""),
		setenvShell("fish", "Output environment variables for fish",
			[]string{"cabin setenv fish | source"},
			"config.fish", "cabin setenv fish <name> | source"),
	)
}

// setenvShell builds one `cabin setenv <shell>` subcommand: it prints each
// resolved variable with emitEnvVar for the shell's syntax and docs, in its
// --help, the idiom(s) to load the output (completion parity: source and eval
// for sh-flavored shells; fish has no <(...) so only the pipe form). Emitting
// never mutates the parent shell, so the caller must source/eval the output (a
// binary cannot change its parent's env).
func setenvShell(name, short string, loadLines []string, rcFile, persistLine string) *cobra.Command {
	var current strings.Builder
	for i, l := range loadLines {
		if i > 0 {
			current.WriteString("\tor\n")
		}
		current.WriteString("\t" + l + "\n")
	}
	long := fmt.Sprintf(`Output the resolved environment variables for %s, in a form that shell can load.

To load in the current session:
%s
To load for every new session, put one of the lines above in %s (or a direnv .envrc), replacing <name> with the profile you want:
	%s

The profile is resolved from an optional positional <profile>, then --profile, then AI_CABIN_PROFILE, then the current profile.`,
		name, current.String(), rcFile, persistLine)

	return &cobra.Command{
		Use:   name + " [<profile>]",
		Short: short,
		Long:  long,
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Optional positional <profile> is a shorthand for --profile (like
			// the historical `cabin setenv perso`); wins over the flag if both set.
			profile := profileFlag
			if len(args) > 0 {
				profile = args[0]
			}
			vars, err := config.ResolveVars(profile, cliVars)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			// Emit the delta only: a variable already present in the shell env
			// with the same value is left untouched, and a resolved empty value
			// adds nothing (absence behaves like empty for templates), so setenv
			// materializes only what it introduces or changes (profile defaults,
			// AI_CABIN_PROFILE, --var overrides) instead of echoing the whole
			// environment.
			for _, k := range setenvDelta(vars, config.EnvironMap()) {
				fmt.Println(emitEnvVar(name, k, vars[k]))
			}
		},
	}
}

// setenvDelta returns the keys of view that setenv should emit for the shell,
// sorted: a variable the environment already carries with the same value is a
// no-op, and an empty resolved value adds nothing (absence behaves like empty
// for templates), so only what materializes or changes is returned — never the
// noisy unchanged environment. Pure: no I/O, unit-testable.
func setenvDelta(view, env map[string]string) []string {
	keys := make([]string, 0, len(view))
	for k := range view {
		if view[k] != "" {
			if v, present := env[k]; !present || v != view[k] {
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

// emitEnvVar returns the shell statement that sets key=value for the given
// shell. bash and zsh share the export form; fish uses set -gx (exported,
// global). The value is always single-quoted (see shellQuote): only the quote
// character is special inside single quotes, so $, backticks and history are
// never expanded — a setenv that materializes credentials must not corrupt or
// execute them. Pure: no I/O, unit-testable.
func emitEnvVar(shell, key, value string) string {
	quoted := shellQuote(shell, value)
	if shell == "fish" {
		return fmt.Sprintf("set -gx %s %s", key, quoted)
	}
	return fmt.Sprintf("export %s=%s", key, quoted)
}

// shellQuote wraps value in single quotes for the given shell so bash/zsh and
// fish do not expand the content. Inside single quotes the only special
// character is the quote itself: POSIX shells escape an embedded quote as the
// close-quote/backslash-quote/reopen idiom ('\”), fish as a backslash quote
// ('\” is written \').
func shellQuote(shell, value string) string {
	if shell == "fish" {
		value = strings.ReplaceAll(value, `\`, `\\`)
		value = strings.ReplaceAll(value, "'", `\'`)
	} else {
		value = strings.ReplaceAll(value, "'", `'\''`)
	}
	return "'" + value + "'"
}
