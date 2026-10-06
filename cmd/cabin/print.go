package main

import (
	"fmt"

	"github.com/JulienVdG/AI-Cabin/internal/config"
)

// printVars prints profile variables in the canonical key order
// (config.OrderedVarKeys), so the output is stable across runs and call
// sites (setup, init, show) and matches the persisted profile files.
func printVars(vars map[string]string) {
	fmt.Println("Variables:")
	for _, k := range config.OrderedVarKeys(vars) {
		fmt.Printf("  %s=%s\n", k, vars[k])
	}
}
