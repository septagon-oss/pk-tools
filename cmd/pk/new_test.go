package main

// new_test.go verifies the `pk new module` command: dry-run listing, real
// generation into --dir, overwrite refusal, and name validation.
//
// Validates: REQ-015.
// Per: ADR-0029 (file purpose declaration).
// Discipline: C-14.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runPk(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestNewModuleDryRun(t *testing.T) {
	dir := t.TempDir()
	out, err := runPk(t, "new", "module", "--name", "notes", "--dir", dir, "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"notes/module.go", "notes/store.go", "notes/handler.go", "notes/module_test.go", "notes/README.md"} {
		if !strings.Contains(out, path) {
			t.Errorf("dry-run output missing %s\n%s", path, out)
		}
	}
	if _, statErr := os.Stat(filepath.Join(dir, "notes")); !os.IsNotExist(statErr) {
		t.Error("dry-run must not write files")
	}
}

func TestNewModuleWritesAndPrintsSnippet(t *testing.T) {
	dir := t.TempDir()
	out, err := runPk(t, "new", "module", "--name", "notes", "--description", "Notes.", "--dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "notes", "module.go")); statErr != nil {
		t.Fatalf("module.go not written: %v", statErr)
	}
	if !strings.Contains(out, "starterapp.WithModules") {
		t.Errorf("output missing registration snippet:\n%s", out)
	}
}

func TestNewModuleRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	if _, err := runPk(t, "new", "module", "--name", "notes", "--dir", dir); err != nil {
		t.Fatal(err)
	}
	_, err := runPk(t, "new", "module", "--name", "notes", "--dir", dir)
	if err == nil {
		t.Fatal("second run must refuse to overwrite")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "refusing to overwrite")
	}
}

func TestNewModuleRejectsBadName(t *testing.T) {
	_, err := runPk(t, "new", "module", "--name", "Notes", "--dir", t.TempDir())
	if err == nil {
		t.Fatal("expected validation error for PascalCase name")
	}
	if !strings.Contains(err.Error(), "snake_case") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "snake_case")
	}
}
