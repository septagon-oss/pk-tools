package cliapp

// Implements: REQ-015.
// Per: ADR-0021.
// Discipline: C-14.
// output.go owns renderer-neutral command output helpers for machine-readable
// CLI responses.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"encoding/json"
	"maps"
	"slices"

	"github.com/spf13/cobra"
)

// WriteJSON encodes value as a single line of JSON to the command's standard
// output stream, returning any encoding error.
func WriteJSON(cmd *cobra.Command, value any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	return enc.Encode(value)
}

// SortedMapKeys returns the keys of values sorted in ascending lexical order,
// giving deterministic iteration for machine-readable CLI output.
func SortedMapKeys[V any](values map[string]V) []string {
	return slices.Sorted(maps.Keys(values))
}
