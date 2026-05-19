package main

// explain.go owns the `pk explain modules` subcommand, which prints the
// 9-module OSS essentials pack with the metadata each module declares as
// public constants. Detailed port wiring lives in each module's docs and in
// the catalog's Compose() output; for v0.0.0 we surface the catalog overview
// only.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/septagon-oss/pk-modules/pkg/admin"
	"github.com/septagon-oss/pk-modules/pkg/apikey"
	"github.com/septagon-oss/pk-modules/pkg/audit"
	"github.com/septagon-oss/pk-modules/pkg/auth"
	"github.com/septagon-oss/pk-modules/pkg/content"
	"github.com/septagon-oss/pk-modules/pkg/health"
	"github.com/septagon-oss/pk-modules/pkg/notification"
	"github.com/septagon-oss/pk-modules/pkg/tenant"
	"github.com/septagon-oss/pk-modules/pkg/user"
	"github.com/spf13/cobra"
)

// moduleInfo is the wire shape rendered by `pk explain modules --json`.
type moduleInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

func newExplainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Inspect the OSS module catalog",
		Long: "explain surfaces the OSS module catalog so downstream developers " +
			"can quickly see what comes in the v0.0.0 essentials pack without " +
			"booting a PlatformKit application.",
	}
	cmd.AddCommand(newExplainModulesCmd())
	return cmd
}

func newExplainModulesCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "modules",
		Short: "Print the 9-module OSS essentials pack",
		Long: "modules prints the OSS essentials pack (tenant, user, auth, " +
			"api-key, audit, health, notification, content, admin) with each " +
			"module's id, name, description, and version. The metadata is " +
			"sourced from each module's public constants, so it stays in sync " +
			"with the catalog automatically.",
		RunE: func(c *cobra.Command, args []string) error {
			return runExplainModules(c.OutOrStdout(), asJSON)
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "output as JSON")
	return c
}

// catalogModules returns the static 9-module essentials pack. We import each
// module package directly so the constants are the canonical source of truth.
// Note that we do not call NewModule() for each — most modules require a
// sqlite DSN to construct fully, and explain is meant to be cheap.
func catalogModules() []moduleInfo {
	return []moduleInfo{
		{ID: tenant.ModuleID, Name: tenant.ModuleName, Description: tenant.ModuleDescription, Version: tenant.ModuleVersion},
		{ID: user.ModuleID, Name: user.ModuleName, Description: user.ModuleDescription, Version: user.ModuleVersion},
		{ID: auth.ModuleID, Name: auth.ModuleName, Description: auth.ModuleDescription, Version: auth.ModuleVersion},
		{ID: apikey.ModuleID, Name: apikey.ModuleName, Description: apikey.ModuleDescription, Version: apikey.ModuleVersion},
		{ID: audit.ModuleID, Name: audit.ModuleName, Description: audit.ModuleDescription, Version: audit.ModuleVersion},
		{ID: health.ModuleID, Name: health.ModuleName, Description: health.ModuleDescription, Version: health.ModuleVersion},
		{ID: notification.ModuleID, Name: notification.ModuleName, Description: notification.ModuleDescription, Version: notification.ModuleVersion},
		{ID: content.ModuleID, Name: content.ModuleName, Description: content.ModuleDescription, Version: content.ModuleVersion},
		{ID: admin.ModuleID, Name: admin.ModuleName, Description: admin.ModuleDescription, Version: admin.ModuleVersion},
	}
}

func runExplainModules(w io.Writer, asJSON bool) error {
	infos := catalogModules()
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(infos)
	}
	fmt.Fprintln(w, "PlatformKit OSS module catalog (v0.0.0):")
	for _, m := range infos {
		fmt.Fprintf(w, "  %-25s %s — %s\n", m.ID, m.Name, m.Description)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Port wiring details: see each module's Compose() and the pk-modules README.")
	return nil
}
