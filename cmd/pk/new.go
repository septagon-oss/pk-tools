package main

// Implements: REQ-015.
// Per: ADR-0021.
// Discipline: C-14.
// new.go owns the `pk new` command family. `pk new module` generates a
// starter-shaped PlatformKit module package (scaffold.GenerateStarterModule)
// and materializes it on disk (scaffold.WriteFiles), printing the
// starterapp.WithModules registration snippet the caller pastes into their
// own main.go.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"fmt"

	"github.com/septagon-oss/pk-tools/pkg/scaffold"
	"github.com/spf13/cobra"
)

func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Generate PlatformKit building blocks",
		Long: "new generates starter-shaped building blocks. `pk new module` " +
			"creates a module package you register with starterapp.WithModules.",
	}
	cmd.AddCommand(newNewModuleCmd())
	return cmd
}

func newNewModuleCmd() *cobra.Command {
	var name, description, dir string
	var dryRun bool
	c := &cobra.Command{
		Use:   "module",
		Short: "Generate a custom module for the PlatformKit starter",
		Long: "module generates a starter-shaped module package: module.go, " +
			"store.go, handler.go, module_test.go, and a README, plus the " +
			"starterapp.WithModules snippet you paste into your own main.go " +
			"to register it.",
		RunE: func(c *cobra.Command, args []string) error {
			res, err := scaffold.GenerateStarterModule(scaffold.StarterModuleOptions{
				Name: name, Description: description,
			})
			if err != nil {
				return err
			}
			if err := scaffold.WriteFiles(scaffold.WriteOptions{
				BaseDir: dir, Files: res.Files, DryRun: dryRun, Output: c.OutOrStdout(),
			}); err != nil {
				return err
			}
			if dryRun {
				return nil
			}
			fmt.Fprintf(c.OutOrStdout(), "\nModule %q generated. Register it in your main.go:\n\n%s\n", name, res.RegistrationCode["snippet"])
			fmt.Fprintln(c.OutOrStdout(), "Then run: go run .")
			return nil
		},
	}
	c.Flags().StringVar(&name, "name", "", "module name in snake_case (required)")
	c.Flags().StringVar(&description, "description", "", "one-line module description")
	c.Flags().StringVar(&dir, "dir", ".", "directory to generate into")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be generated without writing")
	_ = c.MarkFlagRequired("name")
	return c
}
