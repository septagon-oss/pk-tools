// Validates: REQ-002, REQ-004.
// Per: ADR-0052, ADR-0061.
// Discipline: C-14.

package scaffold

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateE2EFlowUsesModuleOwnedContract(t *testing.T) {
	result, err := GenerateE2EFlow(E2EFlowOptions{
		ModuleName: "inventory_management",
		Feature:    "catalog",
		EntityName: "StockItem",
		Fields: []Field{
			{Name: "name", Type: "string", Required: true},
			{Name: "quantity", Type: "integer", Required: true},
		},
	})
	if err != nil {
		t.Fatalf("GenerateE2EFlow: %v", err)
	}
	if len(result.Files) != 2 || result.Files[0].Path != "flows.go" || result.Files[1].Path != "flows_test.go" {
		t.Fatalf("GenerateE2EFlow files = %+v; want canonical flow definition and runner", result.Files)
	}

	all := result.Files[0].Content + result.Files[1].Content
	for _, want := range []string{
		`func Flows() []flow.FlowSpec`,
		`e2eflow.RunFlows(t, Flows())`,
		`github.com/septagon-dev/platformkit-tests/flow`,
		`Fulfills("REQ-004#AC-1")`,
	} {
		if !strings.Contains(all, want) {
			t.Errorf("generated flow suite missing %q", want)
		}
	}
	for _, retired := range []string{
		"github.com/comumcowork/tests",
		"flow.RegisterFlow",
		"LoginThenExecute",
		"func init()",
	} {
		if strings.Contains(all, retired) {
			t.Errorf("generated flow suite contains retired contract %q", retired)
		}
	}
	for _, file := range result.Files {
		if _, err := parser.ParseFile(token.NewFileSet(), file.Path, file.Content, parser.AllErrors); err != nil {
			t.Errorf("generated %s does not parse: %v", file.Path, err)
		}
	}
}

func TestGenerateE2EFlowRejectsUnknownFieldType(t *testing.T) {
	_, err := GenerateE2EFlow(E2EFlowOptions{
		ModuleName: "inventory_management",
		Feature:    "catalog",
		EntityName: "StockItem",
		Fields:     []Field{{Name: "quantity", Type: "integer_guess"}},
	})
	if err == nil || !strings.Contains(err.Error(), `field "quantity"`) {
		t.Fatalf("GenerateE2EFlow error = %v; want field context", err)
	}
}
