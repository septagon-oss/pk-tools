// Implements: REQ-002, REQ-004.
// Per: ADR-0052, ADR-0061.
// Discipline: C-14.

package scaffold

import (
	"fmt"
	"strings"
)

// E2EFlowOptions is the canonical input for a module-owned CRUD flow suite.
type E2EFlowOptions struct {
	ModuleName string  `json:"moduleName"`
	Feature    string  `json:"feature"`
	EntityName string  `json:"entityName"`
	Fields     []Field `json:"fields"`
}

// E2EFlowResult contains the module-owned flow definition and runner.
type E2EFlowResult struct {
	ModuleName string          `json:"moduleName"`
	Feature    string          `json:"feature"`
	Files      []GeneratedFile `json:"files"`
}

// GenerateE2EFlow emits the current module-owned flow contract. Generated
// suites expose Flows and use the shared e2eflow runner; init registration and
// application-specific test harness imports are intentionally unsupported.
func GenerateE2EFlow(opts E2EFlowOptions) (E2EFlowResult, error) {
	if strings.TrimSpace(opts.ModuleName) == "" {
		return E2EFlowResult{}, fmt.Errorf("module name is required")
	}
	if strings.TrimSpace(opts.Feature) == "" {
		return E2EFlowResult{}, fmt.Errorf("feature name is required")
	}
	if strings.TrimSpace(opts.EntityName) == "" {
		return E2EFlowResult{}, fmt.Errorf("entity name is required")
	}
	for _, field := range opts.Fields {
		if _, err := ResolveType(field.Type); err != nil {
			return E2EFlowResult{}, fmt.Errorf("field %q: %w", field.Name, err)
		}
	}

	files := normalizeGeneratedGoFiles([]GeneratedFile{
		{Path: "flows.go", Content: generateE2EFlowsCode(opts)},
		{Path: "flows_test.go", Content: generateE2EFlowsTestCode()},
	})
	return E2EFlowResult{
		ModuleName: opts.ModuleName,
		Feature:    opts.Feature,
		Files:      files,
	}, nil
}

func generateE2EFlowsCode(opts E2EFlowOptions) string {
	entity := ToSnakeCase(opts.EntityName)
	plural := entity + "s"
	namespace := strings.TrimSuffix(opts.ModuleName, "_management")
	adminPath := "/admin/" + strings.ReplaceAll(namespace, "_", "-") + "/" + plural
	pageSelector := fmt.Sprintf(`[data-page="%s"], [data-entity="%s"]`, plural, entity)

	var fillSteps strings.Builder
	for _, field := range opts.Fields {
		fmt.Fprintf(
			&fillSteps,
			"\n\t\t\t\tflow.Fill(%q, %q),",
			fmt.Sprintf(`input[name="%s"], select[name="%s"], textarea[name="%s"]`, field.Name, field.Name, field.Name),
			e2eFieldValue(field.Type),
		)
	}

	return fmt.Sprintf(`//go:build e2e

package e2e

import "github.com/septagon-dev/platformkit-tests/flow"

// Flows returns the governed browser flows for %s.%s.
func Flows() []flow.FlowSpec {
	return []flow.FlowSpec{
		flow.DefineFlow(%q).
			Name(%q).
			Module(%q).
			Feature(%q).
			Category(flow.FlowCategoryE2E).
			Tags("@smoke", "@crud", %q).
			Requires("authenticated_user").
			Provides(%q).
			UsesComponent("EntityManagement", "DataGrid").
			Fulfills("REQ-004#AC-1").
			Given("user navigates to the entity overview", flow.Navigate(%q)).
			Then("the entity overview is visible", flow.WaitVisible(%q)).
			Build(),

		flow.DefineFlow(%q).
			Name(%q).
			Module(%q).
			Feature(%q).
			Category(flow.FlowCategoryE2E).
			Tags("@crud", "@mutation", %q).
			Requires("authenticated_user").
			Provides(%q).
			UsesComponent("EntityManagement", "Form", "Button").
			Fulfills("REQ-004#AC-2").
			Given("user opens the create form", flow.Navigate(%q)).
			When("user submits valid entity data",%s
				flow.Click(%q),
			).
			Then("the entity overview is visible", flow.WaitVisible(%q)).
			Build(),
	}
}
`, namespace, opts.Feature,
		namespace+"."+opts.Feature+"."+entity+".list",
		"View "+opts.EntityName+" overview",
		opts.ModuleName,
		opts.Feature,
		"@"+entity,
		entity+"_overview_viewed",
		adminPath,
		pageSelector,
		namespace+"."+opts.Feature+"."+entity+".create",
		"Create "+opts.EntityName,
		opts.ModuleName,
		opts.Feature,
		"@"+entity,
		entity+"_created",
		adminPath+"/new",
		fillSteps.String(),
		`button[type="submit"]`,
		pageSelector,
	)
}

func generateE2EFlowsTestCode() string {
	return `//go:build e2e

package e2e

import (
	"testing"

	e2eflow "github.com/septagon-dev/platformkit-business-modules/tests/e2eflow"
)

func TestFlows(t *testing.T) {
	e2eflow.RunFlows(t, Flows())
}
`
}

func e2eFieldValue(fieldType string) string {
	switch fieldType {
	case "string":
		return "Test Value"
	case "text":
		return "Test description text"
	case "integer":
		return "42"
	case "decimal":
		return "99.99"
	case "boolean":
		return "true"
	case "datetime":
		return "2026-01-15T10:00:00Z"
	case "uuid":
		return "00000000-0000-0000-0000-000000000001"
	case "jsonb":
		return `{"test":true}`
	default:
		panic("unregistered scaffold field type reached E2E generation: " + fieldType)
	}
}
