package main

// Implements: REQ-015.
// Per: ADR-0021.
// Discipline: C-14.
// verify.go owns the `pk verify` subcommand, which runs `go vet ./...` and
// `go test ./...` in the current working directory and surfaces both steps
// with consistent prefixes so users can scan the output.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Run go vet and go test in the current Go module",
		Long: "verify runs `go vet ./...` followed by `go test ./...` in the " +
			"current working directory. Both steps must succeed for the command " +
			"to exit zero. Stdout and stderr from the underlying go tooling are " +
			"streamed through verbatim.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVerify(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "")
		},
	}
}

// runVerify executes vet then test in dir. If dir is empty, the commands
// inherit the caller's working directory, which matches the typical
// `pk verify` invocation pattern.
func runVerify(ctx context.Context, stdout, stderr io.Writer, dir string) error {
	if err := runGoStep(ctx, stdout, stderr, dir, "go vet ./...", "vet", "./..."); err != nil {
		return fmt.Errorf("go vet failed: %w", err)
	}
	fmt.Fprintln(stdout, "[OK] go vet passed")

	if err := runGoStep(ctx, stdout, stderr, dir, "go test ./...", "test", "./..."); err != nil {
		return fmt.Errorf("go test failed: %w", err)
	}
	fmt.Fprintln(stdout, "[OK] go test passed")
	return nil
}

// runGoStep wraps exec.CommandContext with banner output. label is a human
// description ("go vet ./..."); args is the actual argv passed to `go`.
func runGoStep(ctx context.Context, stdout, stderr io.Writer, dir, label string, args ...string) error {
	fmt.Fprintf(stdout, "[RUN] %s\n", label)
	cmd := exec.CommandContext(ctx, "go", args...)
	if dir != "" {
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
