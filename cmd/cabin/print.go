package main

import (
	"fmt"
	"sort"
)

// printVars prints profile variables with keys sorted alphabetically, so the
// output is stable across runs and call sites (setup, init, show).
func printVars(vars map[string]string) {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println("Variables:")
	for _, k := range keys {
		fmt.Printf("  %s=%s\n", k, vars[k])
	}
}
