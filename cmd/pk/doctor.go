package main

// doctor.go owns the `pk doctor` subcommand, which runs a small battery of
// environment checks (Go version, sqlite driver availability, port 8080
// availability, GOPATH writability) and prints a structured pass/fail report.
//
// Implements: REQ-015.
// Per: ADR-0029 (file purpose declaration).
// Discipline: C-14.

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// doctorCheck is a single doctor probe. Run returns a short status message,
// an ok flag, and an optional error. A non-nil error is rendered alongside
// the failure but never aborts subsequent checks.
type doctorCheck struct {
	Name string
	Run  func(ctx context.Context) (status string, ok bool, err error)
}

// minGoMinor is the minimum supported go1.X release. PlatformKit OSS is built
// against go 1.26 (see go.work / the module go.mod files), so the floor is 1.26:
// users on older toolchains get a clear doctor failure rather than an opaque
// build error later.
const minGoMinor = 26

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check that the PlatformKit OSS dev environment is healthy",
		Long: "doctor verifies a small set of preconditions for working in the " +
			"PlatformKit OSS workspace: Go toolchain version, sqlite driver " +
			"availability, port 8080 availability for the starter-saas demo, and " +
			"GOPATH writability.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctorChecks(cmd.OutOrStdout(), cmd.Context(), defaultDoctorChecks())
		},
	}
}

// defaultDoctorChecks returns the production doctor probe set. It is exposed
// so tests can mutate or substitute individual checks.
func defaultDoctorChecks() []doctorCheck {
	return []doctorCheck{
		{Name: "Go version >= 1.26", Run: checkGoVersion},
		{Name: "modernc.org/sqlite pure-Go driver available", Run: checkSQLite},
		{Name: ":8080 port free for starter-saas", Run: checkPort8080Free},
		{Name: "GOPATH writable", Run: checkGopathWritable},
	}
}

// checkGoVersion parses runtime.Version (which encodes the compiler's go1.X
// release) and rejects anything older than minGoMinor. The check does not
// shell out to `go version` because that would conflate the active toolchain
// with whatever go binary built pk itself.
func checkGoVersion(ctx context.Context) (string, bool, error) {
	v := runtime.Version()
	if !strings.HasPrefix(v, "go1.") {
		return v, false, fmt.Errorf("unrecognized Go version %q", v)
	}
	rest := strings.TrimPrefix(v, "go1.")
	// rest looks like "26", "26.2", or "26rc1" — take the leading digits.
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return v, false, fmt.Errorf("cannot parse minor version from %q", v)
	}
	minor, err := strconv.Atoi(rest[:end])
	if err != nil {
		return v, false, fmt.Errorf("cannot parse minor version from %q: %w", v, err)
	}
	if minor < minGoMinor {
		return v, false, nil
	}
	return v, true, nil
}

// checkSQLite probes whether the current module declares modernc.org/sqlite as
// a dependency. We deliberately do not link the driver into pk itself: the
// pure-Go sqlite package is heavy and only relevant when the user is in a
// PlatformKit module workspace.
func checkSQLite(ctx context.Context) (string, bool, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "modernc.org/sqlite")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "not in current module's deps", false, nil
	}
	return strings.TrimSpace(string(out)), true, nil
}

// checkPort8080Free tries to bind :8080 long enough to confirm it is free. The
// starter-saas demo (and most PlatformKit example apps) defaults to this port.
func checkPort8080Free(ctx context.Context) (string, bool, error) {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		return "in use", false, nil
	}
	_ = ln.Close()
	return "free", true, nil
}

// checkGopathWritable resolves GOPATH (or the default ~/go) and confirms a
// temp file can be written under it. This is a cheap proxy for "go install"
// being able to land binaries.
func checkGopathWritable(ctx context.Context) (string, bool, error) {
	gp := os.Getenv("GOPATH")
	if gp == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false, fmt.Errorf("resolve home: %w", err)
		}
		gp = filepath.Join(home, "go")
	}
	if err := os.MkdirAll(gp, 0o755); err != nil {
		return gp, false, fmt.Errorf("ensure GOPATH: %w", err)
	}
	f, err := os.CreateTemp(gp, ".pk-doctor-*")
	if err != nil {
		return gp, false, err
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return gp, true, nil
}

// runDoctorChecks runs every probe in order, prints a single line per check,
// and returns a non-nil error if any check failed. The returned error is a
// simple sentinel; the detailed report is already on the output stream.
func runDoctorChecks(w io.Writer, ctx context.Context, checks []doctorCheck) error {
	allOK := true
	for _, c := range checks {
		status, ok, err := c.Run(ctx)
		if err != nil {
			ok = false
		}
		symbol := "OK"
		if !ok {
			symbol = "FAIL"
			allOK = false
		}
		if err != nil {
			fmt.Fprintf(w, "[%s] %s: %s (error: %v)\n", symbol, c.Name, status, err)
		} else {
			fmt.Fprintf(w, "[%s] %s: %s\n", symbol, c.Name, status)
		}
	}
	if !allOK {
		return fmt.Errorf("doctor reported failures; see above")
	}
	fmt.Fprintln(w, "doctor: all checks passed")
	return nil
}
