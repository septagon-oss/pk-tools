package cliapp_test

// example_test.go provides runnable godoc examples for the cliapp package's
// root-command assembly helpers.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"fmt"

	"github.com/septagon-oss/pk-tools/pkg/cliapp"
	"github.com/spf13/cobra"
)

// ExampleNewRoot demonstrates assembling a root command from RootOptions and
// running it with Execute.
func ExampleNewRoot() {
	greet := &cobra.Command{
		Use: "greet",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "hello from pk-tools")
			return nil
		},
	}

	root := cliapp.NewRoot(cliapp.RootOptions{
		Use:      "demo",
		Short:    "Demo CLI built on pk-tools primitives",
		Version:  "v0.1.0",
		Commands: []*cobra.Command{greet},
	})

	root.SetArgs([]string{"greet"})
	if err := cliapp.Execute(root); err != nil {
		fmt.Println("error:", err)
	}
	// Output: hello from pk-tools
}
