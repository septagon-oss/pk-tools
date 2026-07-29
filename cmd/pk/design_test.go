package main

// Validates: REQ-011, REQ-015.
// Per: ADR-0076.
// Discipline: C-14.
// design_test.go proves the OSS CLI exports native deliveries and keeps token
// receives dry-run-only unless an explicit atomic write is requested.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/septagon-oss/pk-design/pkg/delivery"
	"github.com/septagon-oss/pk-design/pkg/figma"
	"github.com/septagon-oss/pk-design/pkg/handoff"
	"github.com/septagon-oss/pk-design/pkg/themes"
)

func TestDesignExportWritesNativeOSSBundle(t *testing.T) {
	output := filepath.Join(t.TempDir(), "figma-variables.json")
	command := newRootCmd()
	command.SetArgs([]string{
		"design", "export",
		"--out", output,
		"--generated-at", "2026-07-27T12:00:00Z",
	})
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	if err := command.Execute(); err != nil {
		t.Fatalf("design export error = %v", err)
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var bundle figma.Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("invalid exported bundle: %v", err)
	}
	if bundle.Schema != figma.BundleSchemaVersion ||
		bundle.Snapshot.Profile.ID != delivery.OSSProfileID ||
		bundle.Metadata["delivery"] != "native-editable" {
		t.Fatalf("unexpected exported bundle: %#v", bundle)
	}
}

func TestDesignReceiveTokensIsDryRunThenAtomicWrite(t *testing.T) {
	source := filepath.Join(t.TempDir(), "default.tokens.json")
	if err := os.WriteFile(source, themes.DefaultSource(), 0o640); err != nil {
		t.Fatal(err)
	}
	changesPath := filepath.Join(t.TempDir(), "changes.json")
	base, err := delivery.DefaultSnapshot(time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	edited := base
	edited.Tokens = append([]handoff.Token(nil), base.Tokens...)
	for index := range edited.Tokens {
		if edited.Tokens[index].Path == "/color/signal" {
			edited.Tokens[index].Value = "#d7f25c"
		}
	}
	changes, err := handoff.Diff(base, edited, handoff.Provenance{
		Source:      "figma-desktop",
		GeneratedAt: time.Date(2026, 7, 27, 13, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	changeBytes, err := json.Marshal(changes)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(changesPath, changeBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runDesignReceiveTokens(&out, changesPath, source, "human", false); err != nil {
		t.Fatalf("dry-run error = %v", err)
	}
	afterDryRun, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, afterDryRun) {
		t.Fatal("dry-run mutated the source")
	}
	if err := runDesignReceiveTokens(&out, changesPath, source, "human", true); err != nil {
		t.Fatalf("write error = %v", err)
	}
	afterWrite, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	theme, err := themes.ParseDefaultSource(afterWrite)
	if err != nil {
		t.Fatalf("written DTCG is invalid: %v", err)
	}
	if got := theme.Tokens.Values["color.signal"]; got != "#d7f25c" {
		t.Fatalf("written signal = %v", got)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("source mode = %#o, want 0640", got)
	}
}
