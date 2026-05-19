package main

// pk_test.go contains smoke tests for the doctor, verify, and explain
// subcommands. They exercise the command surface through the cobra wiring
// rather than forking subprocesses so they run quickly and deterministically
// in CI.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorReportsCheckResults(t *testing.T) {
	ctx := context.Background()
	checks := []doctorCheck{
		{Name: "always-pass", Run: func(ctx context.Context) (string, bool, error) {
			return "fine", true, nil
		}},
		{Name: "always-fail", Run: func(ctx context.Context) (string, bool, error) {
			return "broken", false, nil
		}},
	}
	var out bytes.Buffer
	err := runDoctorChecks(&out, ctx, checks)
	if err == nil {
		t.Fatal("expected error because one check failed")
	}
	got := out.String()
	if !strings.Contains(got, "[OK] always-pass") {
		t.Fatalf("missing OK line: %q", got)
	}
	if !strings.Contains(got, "[FAIL] always-fail") {
		t.Fatalf("missing FAIL line: %q", got)
	}
}

func TestDoctorPortCheckReportsBusyWhenBound(t *testing.T) {
	// Bind :8080 for the lifetime of the test. If the host already has
	// something on :8080 the bind itself will fail; in that case the doctor
	// would also report busy, so we skip.
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		t.Skipf("cannot bind :8080 in this environment: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	status, ok, err := checkPort8080Free(context.Background())
	if err != nil {
		t.Fatalf("checkPort8080Free returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected port to be reported busy, got status %q", status)
	}
	if status != "in use" {
		t.Fatalf("expected status 'in use', got %q", status)
	}
}

func TestDoctorGoVersionCheck(t *testing.T) {
	status, ok, err := checkGoVersion(context.Background())
	if err != nil {
		t.Fatalf("checkGoVersion returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected current Go (%s) to satisfy >= 1.22", status)
	}
	if !strings.HasPrefix(status, "go1.") {
		t.Fatalf("unexpected version prefix: %q", status)
	}
}

func TestVerifySucceedsInTrivialModule(t *testing.T) {
	dir := writeTempModule(t, `package demo

func Add(a, b int) int { return a + b }
`, `package demo

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Fatal("math broken")
	}
}
`)

	var stdout, stderr bytes.Buffer
	if err := runVerify(context.Background(), &stdout, &stderr, dir); err != nil {
		t.Fatalf("runVerify error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "go vet passed") {
		t.Fatalf("expected vet success in output: %q", out)
	}
	if !strings.Contains(out, "go test passed") {
		t.Fatalf("expected test success in output: %q", out)
	}
}

func TestVerifyReportsTestFailure(t *testing.T) {
	dir := writeTempModule(t, `package demo

func Add(a, b int) int { return a + b }
`, `package demo

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 999 {
		t.Fatal("math broken (intentional)")
	}
}
`)

	var stdout, stderr bytes.Buffer
	err := runVerify(context.Background(), &stdout, &stderr, dir)
	if err == nil {
		t.Fatalf("expected runVerify to fail; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(err.Error(), "go test failed") {
		t.Fatalf("expected go test failure wrapper, got: %v", err)
	}
}

func TestExplainModulesPrintsAllNine(t *testing.T) {
	var out bytes.Buffer
	if err := runExplainModules(&out, false); err != nil {
		t.Fatalf("runExplainModules error: %v", err)
	}
	expected := []string{
		"tenant_management",
		"user_management",
		"auth_management",
		"api_key_management",
		"audit_management",
		"health_management",
		"notification_management",
		"content_management",
		"admin_management",
	}
	got := out.String()
	for _, id := range expected {
		if !strings.Contains(got, id) {
			t.Fatalf("missing module %q in output:\n%s", id, got)
		}
	}
}

func TestExplainModulesJSONShape(t *testing.T) {
	var out bytes.Buffer
	if err := runExplainModules(&out, true); err != nil {
		t.Fatalf("runExplainModules error: %v", err)
	}
	var decoded []moduleInfo
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\npayload: %s", err, out.String())
	}
	if len(decoded) != 9 {
		t.Fatalf("expected 9 modules, got %d", len(decoded))
	}
	for _, m := range decoded {
		if m.ID == "" || m.Name == "" || m.Description == "" {
			t.Fatalf("entry missing required field: %+v", m)
		}
	}
}

func TestRootCommandIncludesAllSubcommands(t *testing.T) {
	root := newRootCmd()
	subs := map[string]bool{}
	for _, c := range root.Commands() {
		subs[c.Name()] = true
	}
	for _, want := range []string{"doctor", "verify", "explain"} {
		if !subs[want] {
			t.Fatalf("missing subcommand %q; have %v", want, subs)
		}
	}
}

// writeTempModule writes a minimal Go module under t.TempDir() with the given
// implementation and test bodies, then returns the module root. The module
// path is fixed to example.com/demo so go vet/test do not need network access.
func writeTempModule(t *testing.T, src, testSrc string) string {
	t.Helper()
	dir := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	must(os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.22\n"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "demo.go"), []byte(src), 0o644))
	must(os.WriteFile(filepath.Join(dir, "demo_test.go"), []byte(testSrc), 0o644))
	return dir
}
