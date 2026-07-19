// Validates: REQ-002, REQ-016.
// Per: ADR-0061.
// Discipline: C-14.

package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingProgressWriter struct{}

func (failingProgressWriter) Write([]byte) (int, error) {
	return 0, errors.New("progress unavailable")
}

func TestGenerateModuleRejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name string
		opts ModuleOptions
		want string
	}{
		{name: "path name", opts: ModuleOptions{Name: "../billing", Description: "Billing"}, want: "snake_case"},
		{name: "missing description", opts: ModuleOptions{Name: "billing_management"}, want: "description is required"},
		{name: "unknown archetype", opts: ModuleOptions{Name: "billing_management", Description: "Billing", Archetype: "legacy"}, want: "archetype"},
		{name: "duplicate feature", opts: ModuleOptions{Name: "billing_management", Description: "Billing", Features: []string{"invoices", "invoices"}}, want: "duplicate feature"},
		{name: "invalid port", opts: ModuleOptions{Name: "billing_management", Description: "Billing", Ports: []string{"audit.Service"}}, want: "exported Go identifier"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := GenerateModule(test.opts)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("GenerateModule error = %v; want %q", err, test.want)
			}
		})
	}
}

func TestGenerateEntityRejectsUnsafeOrAmbiguousFields(t *testing.T) {
	tests := []struct {
		name   string
		fields []Field
		seq    uint
		want   string
	}{
		{name: "no sequence", fields: []Field{{Name: "amount", Type: "decimal"}}, want: "migration sequence"},
		{name: "no fields", seq: 1, want: "fields are required"},
		{name: "base collision", fields: []Field{{Name: "tenantId", Type: "uuid"}}, seq: 1, want: "collides with BaseEntity"},
		{name: "duplicate", fields: []Field{{Name: "amount", Type: "decimal"}, {Name: "amount", Type: "decimal"}}, seq: 1, want: "duplicate field"},
		{name: "noncanonical", fields: []Field{{Name: "due_date", Type: "datetime"}}, seq: 1, want: "lowerCamelCase"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := GenerateEntity(EntityOptions{
				ModuleName:        "billing_management",
				Name:              "Invoice",
				TableName:         "invoices",
				Fields:            test.fields,
				MigrationSequence: test.seq,
			})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("GenerateEntity error = %v; want %q", err, test.want)
			}
		})
	}
}

func TestGenerateEntityRequiresCanonicalTableName(t *testing.T) {
	_, err := GenerateEntity(EntityOptions{
		ModuleName:        "policy_management",
		Name:              "Policy",
		TableName:         "../policies",
		Fields:            []Field{{Name: "title", Type: "string"}},
		MigrationSequence: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "table name") {
		t.Fatalf("GenerateEntity error = %v; want table-name validation", err)
	}
}

func TestGenerateFeatureRejectsInvalidUseCase(t *testing.T) {
	_, err := GenerateFeature(FeatureOptions{
		ModuleName: "billing_management",
		Name:       "invoice_reporting",
		UseCases:   []string{"generate-report"},
	})
	if err == nil || !strings.Contains(err.Error(), "exported Go identifier") {
		t.Fatalf("GenerateFeature error = %v; want exported identifier failure", err)
	}
}

func TestWriteFilesConfinesPathsAndRefusesOverwrite(t *testing.T) {
	baseDir := t.TempDir()
	if err := WriteFiles(WriteOptions{BaseDir: baseDir, Files: []GeneratedFile{{Path: "../escape.go", Content: "escape"}}, DryRun: true}); err == nil {
		t.Fatal("WriteFiles accepted a traversal path")
	}
	if err := WriteFiles(WriteOptions{BaseDir: baseDir, Files: []GeneratedFile{{Path: "same.go"}, {Path: "same.go"}}, DryRun: true}); err == nil {
		t.Fatal("WriteFiles accepted duplicate output paths")
	}
	if err := WriteFiles(WriteOptions{BaseDir: baseDir, Files: []GeneratedFile{{Path: "tree"}, {Path: "tree/child.go"}}, DryRun: true}); err == nil {
		t.Fatal("WriteFiles accepted a file that is also an output parent")
	}

	existingPath := filepath.Join(baseDir, "existing.go")
	if err := os.WriteFile(existingPath, []byte("owned"), 0o644); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}
	if err := WriteFiles(WriteOptions{BaseDir: baseDir, Files: []GeneratedFile{{Path: "existing.go", Content: "replacement"}}}); err == nil {
		t.Fatal("WriteFiles overwrote an existing file")
	}
	if err := WriteFiles(WriteOptions{BaseDir: baseDir, Files: []GeneratedFile{{Path: "existing.go", Content: "replacement"}}, DryRun: true}); err == nil {
		t.Fatal("WriteFiles dry run ignored an existing file")
	}
	content, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read existing file: %v", err)
	}
	if string(content) != "owned" {
		t.Fatalf("existing file changed to %q", content)
	}
	preflightPath := filepath.Join(baseDir, "preflight.go")
	if err := WriteFiles(WriteOptions{BaseDir: baseDir, Files: []GeneratedFile{
		{Path: "preflight.go", Content: "must not be written"},
		{Path: "existing.go", Content: "replacement"},
	}}); err == nil {
		t.Fatal("WriteFiles accepted a set containing an existing target")
	}
	if _, err := os.Stat(preflightPath); !os.IsNotExist(err) {
		t.Fatalf("WriteFiles left a partial preflight output: %v", err)
	}

	if err := WriteFiles(WriteOptions{BaseDir: baseDir, Files: []GeneratedFile{{Path: "nested/new.go", Content: "new"}}}); err != nil {
		t.Fatalf("WriteFiles new file: %v", err)
	}

	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(baseDir, "linked")); err != nil {
		t.Fatalf("create parent symlink: %v", err)
	}
	if err := WriteFiles(WriteOptions{BaseDir: baseDir, Files: []GeneratedFile{{Path: "linked/escape.go", Content: "escape"}}}); err == nil {
		t.Fatal("WriteFiles followed a symbolic-link output parent")
	}

	baseParent := t.TempDir()
	outsideBase := t.TempDir()
	linkedBase := filepath.Join(baseParent, "linked-base")
	if err := os.Symlink(outsideBase, linkedBase); err != nil {
		t.Fatalf("create base-parent symlink: %v", err)
	}
	if err := WriteFiles(WriteOptions{BaseDir: filepath.Join(linkedBase, "generated"), Files: []GeneratedFile{{Path: "escape.go", Content: "escape"}}}); err == nil {
		t.Fatal("WriteFiles followed a symbolic-link base component")
	}
	if _, err := os.Stat(filepath.Join(outsideBase, "generated", "escape.go")); !os.IsNotExist(err) {
		t.Fatalf("WriteFiles escaped through a base component: %v", err)
	}

	rollbackDir := t.TempDir()
	rollbackPaths := []string{"first.go", "second.go"}
	if err := WriteFiles(WriteOptions{
		BaseDir: rollbackDir,
		Files: []GeneratedFile{
			{Path: rollbackPaths[0], Content: "first"},
			{Path: rollbackPaths[1], Content: "second"},
		},
		Output: failingProgressWriter{},
	}); err == nil {
		t.Fatal("WriteFiles ignored progress failure")
	}
	for _, path := range rollbackPaths {
		if _, err := os.Stat(filepath.Join(rollbackDir, path)); !os.IsNotExist(err) {
			t.Fatalf("WriteFiles did not roll back %s: %v", path, err)
		}
	}
}
