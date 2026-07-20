package main

// Implements: REQ-015.
// Per: ADR-0021.
// Discipline: C-14.
// main.go is the entry point for the `pk` OSS CLI binary. It wires the
// cobra root command, registers the doctor/verify/explain subcommands, and
// installs a signal-aware context for cancellation.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "pk: %v\n", err)
		os.Exit(1)
	}
}

// newRootCmd assembles the pk root command and its subcommands. It is exposed
// to package tests via the file-local _test package so that command behavior
// can be verified without forking a subprocess.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pk",
		Short: "PlatformKit OSS developer CLI",
		Long: "pk is the developer CLI for PlatformKit OSS.\n\n" +
			"Run `pk doctor` to verify your environment, `pk verify` to run tests, " +
			"and `pk explain modules` to inspect the OSS module catalog.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newDoctorCmd(), newVerifyCmd(), newExplainCmd())
	return root
}
