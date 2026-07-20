package tui

// Validates: REQ-015.
// Per: ADR-0021.
// Discipline: C-14.
// renderer_test.go validates deterministic terminal rendering behavior with
// and without ANSI color.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"bytes"
	"strings"
	"testing"
)

func TestRendererCommandStartWithoutColor(t *testing.T) {
	var out bytes.Buffer
	r := New(&out, &out)
	r.Color = false

	r.CommandStart("Running", "go", []string{"test", "./..."}, "/repo")

	got := out.String()
	if got != "Running go [test ./...] in /repo\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestRendererTable(t *testing.T) {
	var out bytes.Buffer
	r := New(&out, &out)
	r.Color = false

	r.Table([]string{"Name", "Status"}, [][]string{{"core", "ok"}, {"modules", "ok"}})

	got := out.String()
	for _, want := range []string{"Name", "Status", "core", "modules"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
}
