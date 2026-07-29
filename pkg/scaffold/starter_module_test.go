// Validates: REQ-002.
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
		`CREATE TABLE IF NOT EXISTS notes (`,
		`/api/v1/notes"`,
	} {
		if !strings.Contains(joined, symbol) {
			t.Errorf("generated output missing %q", symbol)
		}
	}
	if strings.Contains(joined, "notess") {
		t.Error("generated output must not double-pluralize a name already ending in \"s\" (found \"notess\")")
	}
	if res.RegistrationCode["snippet"] == "" {
		t.Error("registration snippet is empty")
	}
}

func TestGenerateStarterModulePluralizesNaturally(t *testing.T) {
	// "notes" already ends in "s": table and routes must stay "notes", not
	// double-pluralize to "notess".
	notesRes, err := GenerateStarterModule(StarterModuleOptions{Name: "notes", Description: "d"})
	if err != nil {
		t.Fatal(err)
	}
	notesJoined := allContent(notesRes)
	if !strings.Contains(notesJoined, `CREATE TABLE IF NOT EXISTS notes (`) {
		t.Error(`name "notes": expected table "notes"`)
	}
	if !strings.Contains(notesJoined, `/api/v1/notes"`) {
		t.Error(`name "notes": expected route "/api/v1/notes"`)
	}
	if strings.Contains(notesJoined, "notess") {
		t.Error(`name "notes": must not double-pluralize to "notess"`)
	}

	// "note" does not end in "s": table and routes must pluralize to "notes".
	noteRes, err := GenerateStarterModule(StarterModuleOptions{Name: "note", Description: "d"})
	if err != nil {
		t.Fatal(err)
	}
	noteJoined := allContent(noteRes)
	if !strings.Contains(noteJoined, `CREATE TABLE IF NOT EXISTS notes (`) {
		t.Error(`name "note": expected table "notes"`)
	}
	if !strings.Contains(noteJoined, `/api/v1/notes"`) {
		t.Error(`name "note": expected route "/api/v1/notes"`)
	}

	// "status" already ends in "s": route must stay "/api/v1/status".
	statusRes, err := GenerateStarterModule(StarterModuleOptions{Name: "status", Description: "d"})
	if err != nil {
		t.Fatal(err)
	}
	statusJoined := allContent(statusRes)
	if !strings.Contains(statusJoined, `/api/v1/status"`) {
		t.Error(`name "status": expected route "/api/v1/status"`)
	}
	if strings.Contains(statusJoined, "statuss") {
		t.Error(`name "status": must not double-pluralize to "statuss"`)
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
