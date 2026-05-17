package cliapp

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

func WriteJSON(cmd *cobra.Command, value any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	return enc.Encode(value)
}

func SortedMapKeys[V any](values map[string]V) []string {
	return slices.Sorted(maps.Keys(values))
}
