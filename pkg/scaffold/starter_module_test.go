// Implements: REQ-002.
// Per: ADR-0029 (file purpose declaration).
// Discipline: C-14.

package scaffold

// starter_module_test.go verifies GenerateStarterModule: option validation,
// the emitted starter-shaped file set, and that every generated Go source
// parses cleanly.

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateStarterModuleValidatesName(t *testing.T) {
	for _, bad := range []string{"", "Notes", "my-notes", "1notes", "my notes"} {
		if _, err := GenerateStarterModule(StarterModuleOptions{Name: bad}); err == nil {
			t.Errorf("name %q: expected error, got nil", bad)
		}
	}
}

func TestGenerateStarterModuleEmitsStarterShape(t *testing.T) {
	res, err := GenerateStarterModule(StarterModuleOptions{
		Name:        "notes",
		Description: "Tenant-scoped notes.",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"notes/module.go":      false,
		"notes/store.go":       false,
		"notes/handler.go":     false,
		"notes/module_test.go": false,
		"notes/README.md":      false,
	}
	for _, f := range res.Files {
		if _, ok := want[f.Path]; !ok {
			t.Errorf("unexpected file %s", f.Path)
			continue
		}
		want[f.Path] = true
	}
	for path, seen := range want {
		if !seen {
			t.Errorf("missing file %s", path)
		}
	}
	joined := allContent(res)
	for _, symbol := range []string{
		`ModuleID          = "notes"`,
		"func NewModule(",
		"func (m *Module) Compose()",
		"func (m *Module) HTTPHandler()",
		"portslib.RequestActor",
		"starterapp.WithModules",
	} {
		if !strings.Contains(joined, symbol) {
			t.Errorf("generated output missing %q", symbol)
		}
	}
	if res.RegistrationCode["snippet"] == "" {
		t.Error("registration snippet is empty")
	}
}

func TestGenerateStarterModuleGoFilesParse(t *testing.T) {
	res, err := GenerateStarterModule(StarterModuleOptions{Name: "notes", Description: "d"})
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, f := range res.Files {
		if !strings.HasSuffix(f.Path, ".go") {
			continue
		}
		if _, err := parser.ParseFile(fset, f.Path, f.Content, parser.AllErrors); err != nil {
			t.Errorf("%s does not parse: %v", f.Path, err)
		}
	}
}

func allContent(res ModuleResult) string {
	var b strings.Builder
	for _, f := range res.Files {
		b.WriteString(f.Content)
	}
	for _, v := range res.RegistrationCode {
		b.WriteString(v)
	}
	return b.String()
}
