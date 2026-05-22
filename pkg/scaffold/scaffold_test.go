package scaffold

import (
	"strings"
	"testing"
)

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"billing_management", "BillingManagement"},
		{"user", "User"},
		{"api_key_management", "ApiKeyManagement"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := ToPascalCase(tt.input); got != tt.want {
			t.Errorf("ToPascalCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Invoice", "invoice"},
		{"InvoiceItem", "invoice_item"},
		{"APIKey", "a_p_i_key"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := ToSnakeCase(tt.input); got != tt.want {
			t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGoTypeFromString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"string", "string"},
		{"integer", "int64"},
		{"int", "int64"},
		{"decimal", "float64"},
		{"boolean", "bool"},
		{"datetime", "time.Time"},
		{"uuid", "uuid.UUID"},
		{"unknown", "string"},
	}
	for _, tt := range tests {
		if got := GoTypeFromString(tt.input); got != tt.want {
			t.Errorf("GoTypeFromString(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGORMTypeFromString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"string", "varchar(255)"},
		{"integer", "bigint"},
		{"decimal", "decimal(10,2)"},
		{"boolean", "boolean"},
		{"text", "text"},
	}
	for _, tt := range tests {
		if got := GORMTypeFromString(tt.input); got != tt.want {
			t.Errorf("GORMTypeFromString(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSQLTypeFromString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"string", "VARCHAR(255)"},
		{"integer", "INTEGER"},
		{"decimal", "DECIMAL(19,4)"},
		{"uuid", "UUID"},
		{"jsonb", "JSONB DEFAULT '{}'"},
	}
	for _, tt := range tests {
		if got := SQLTypeFromString(tt.input); got != tt.want {
			t.Errorf("SQLTypeFromString(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseFieldSpec(t *testing.T) {
	tests := []struct {
		input   string
		want    Field
		wantErr bool
	}{
		{"amount:decimal", Field{Name: "amount", Type: "decimal"}, false},
		{"status:string", Field{Name: "status", Type: "string"}, false},
		{"name", Field{Name: "name", Type: "string"}, false},
		{":string", Field{}, true},
		{"", Field{}, true},
	}
	for _, tt := range tests {
		got, err := ParseFieldSpec(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseFieldSpec(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseFieldSpec(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got.Name != tt.want.Name || got.Type != tt.want.Type {
			t.Errorf("ParseFieldSpec(%q) = %+v, want %+v", tt.input, got, tt.want)
		}
	}
}

func TestParseFieldSpecs(t *testing.T) {
	fields, err := ParseFieldSpecs("amount:decimal,status:string,due_date:datetime")
	if err != nil {
		t.Fatalf("ParseFieldSpecs: %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("got %d fields, want 3", len(fields))
	}
	if fields[0].Name != "amount" || fields[0].Type != "decimal" {
		t.Errorf("fields[0] = %+v", fields[0])
	}
	if fields[2].Name != "due_date" || fields[2].Type != "datetime" {
		t.Errorf("fields[2] = %+v", fields[2])
	}

	// Empty string returns nil.
	empty, err := ParseFieldSpecs("")
	if err != nil {
		t.Fatalf("ParseFieldSpecs empty: %v", err)
	}
	if empty != nil {
		t.Errorf("expected nil for empty string, got %v", empty)
	}
}

func TestGenerateModule(t *testing.T) {
	result := GenerateModule("billing_management", "Billing and subscriptions", "commerce", "service",
		[]string{"subscriptions"}, nil)

	if result.ModuleName != "billing_management" {
		t.Errorf("ModuleName = %q", result.ModuleName)
	}

	// Should produce the standardized module skeleton plus feature files.
	fileNames := make(map[string]bool)
	for _, f := range result.Files {
		fileNames[f.Path] = true
	}
	expected := []string{
		"README.md",
		"admin.go",
		"authz.go",
		"contracts/module.go",
		"contracts/permissions.go",
		"contracts/providers.go",
		"contracts/provides/doc.go",
		"contracts/routes.go",
		"dependencies.go",
		"events.go",
		"features/README.md",
		"invocations.go",
		"metadata.go",
		"migrations/README.md",
		"module.go",
		"module_smoke_test.go",
		"settings_provider.go",
	}
	for _, name := range expected {
		if !fileNames[name] {
			t.Errorf("missing %s", name)
		}
	}

	// Module.go should contain the module name.
	for _, f := range result.Files {
		if f.Path == "module.go" {
			if !strings.Contains(f.Content, "billing_management") {
				t.Error("module.go should contain module name")
			}
			if !strings.Contains(f.Content, "BillingManagementModule") {
				t.Error("module.go should contain PascalCase struct")
			}
			if !strings.Contains(f.Content, "ModuleAuthor") {
				t.Error("module.go should contain ModuleAuthor")
			}
			if !strings.Contains(f.Content, "ModuleLicense") {
				t.Error("module.go should contain ModuleLicense")
			}
		}
		if f.Path == "README.md" && !strings.Contains(f.Content, "Archetype: `service`") {
			t.Error("README.md should document the archetype")
		}
	}

	// Registration code should be present.
	if result.RegistrationCode["import"] == "" {
		t.Error("registration code import should not be empty")
	}
}

func TestGenerateModuleAppliesImportProfile(t *testing.T) {
	profile := ImportProfile{
		BackendKit:      "github.com/acme/platformkit-backend-kit",
		BusinessModules: "github.com/acme/platformkit-business-modules",
		FrontendKit:     "github.com/acme/platformkit-frontend-kit",
	}

	result := GenerateModuleWithOptions(ModuleOptions{
		Name:          "billing_management",
		Description:   "Billing and subscriptions",
		Category:      "commerce",
		Archetype:     "service",
		Features:      []string{"subscriptions"},
		ImportProfile: profile,
	})

	combined := result.RegistrationCode["import"]
	for _, file := range result.Files {
		combined += "\n" + file.Content
	}

	for _, needle := range []string{
		"github.com/acme/platformkit-backend-kit/app/module",
		"github.com/acme/platformkit-business-modules/billing_management/features/subscriptions",
		"github.com/acme/platformkit-business-modules/billing_management",
	} {
		if !strings.Contains(combined, needle) {
			t.Fatalf("profiled scaffold output missing %q", needle)
		}
	}
	if strings.Contains(combined, "example.com/platformkit/") {
		t.Fatal("profiled scaffold output still contains neutral import roots")
	}
}

func TestGenerateModuleRegistryArchetypeOmitsMigrations(t *testing.T) {
	result := GenerateModule("translation_management", "Translations", "localization", "registry", nil, nil)

	fileNames := make(map[string]bool)
	for _, f := range result.Files {
		fileNames[f.Path] = true
	}
	if fileNames["migrations/README.md"] {
		t.Error("registry archetype should not scaffold migrations")
	}
}

// TestGenerateModuleUsesRuntimePattern guarantees that newly scaffolded
// modules adopt standard.NewRuntime and avoid the deprecated moduleInstance
// + three-wrapper boilerplate. This is the tripwire that prevents the
// scaffold from regressing to the old pattern as the framework evolves.
func TestGenerateModuleUsesRuntimePattern(t *testing.T) {
	result := GenerateModule("billing_management", "Billing and subscriptions", "commerce", "service",
		[]string{"subscriptions"}, nil)

	var moduleGo string
	for _, f := range result.Files {
		if f.Path == "module.go" {
			moduleGo = f.Content
			break
		}
	}
	if moduleGo == "" {
		t.Fatal("scaffold did not produce module.go")
	}

	mustContain := []string{
		"standard.NewRuntime(",
		"GetModule   = moduleRuntime.GetModule",
		"NewModule   = moduleRuntime.NewModule",
		"GetFeatures = moduleRuntime.GetFeatures",
	}
	for _, needle := range mustContain {
		if !strings.Contains(moduleGo, needle) {
			t.Errorf("module.go should contain %q — scaffold regressed to the pre-Runtime pattern", needle)
		}
	}

	mustNotContain := []string{
		"module.NewSingleton(createModule)",
		"moduleInstance.Get()",
		"moduleInstance.GetAndRegister()",
		"func createModule()",
		"func GetModule() module.Module",
		"func NewModule() module.Module",
	}
	for _, needle := range mustNotContain {
		if strings.Contains(moduleGo, needle) {
			t.Errorf("module.go should NOT contain %q — scaffold still emits the deprecated singleton+wrapper pattern", needle)
		}
	}
}

func TestGenerateEntity(t *testing.T) {
	fields := []Field{
		{Name: "amount", Type: "decimal", Description: "Invoice amount", Required: true},
		{Name: "status", Type: "string", Description: "Invoice status"},
	}
	result := GenerateEntity("billing_management", "Invoice", fields)

	if result.EntityName != "Invoice" {
		t.Errorf("EntityName = %q", result.EntityName)
	}

	// Should produce entity .go, _test.go, e2e.go.
	if len(result.Files) != 3 {
		t.Errorf("got %d files, want 3", len(result.Files))
	}
	if len(result.Migrations) != 2 {
		t.Errorf("got %d migrations, want 2", len(result.Migrations))
	}

	// Entity code should reference billing_management.
	entityCode := result.Files[0].Content
	if !strings.Contains(entityCode, "billing_management") {
		t.Error("entity code should reference module name")
	}
	if !strings.Contains(entityCode, "MCPToolName") {
		t.Error("entity code should contain MCP methods")
	}

	// Migration should contain CREATE TABLE.
	if !strings.Contains(result.Migrations[0].Content, "CREATE TABLE") {
		t.Error("migration up should contain CREATE TABLE")
	}
	if !strings.Contains(result.Migrations[1].Content, "DROP TABLE") {
		t.Error("migration down should contain DROP TABLE")
	}

	// Register snippet should not be empty.
	if result.RegisterSnippet == "" {
		t.Error("RegisterSnippet should not be empty")
	}
}

func TestGenerateFeature(t *testing.T) {
	result := GenerateFeature("billing_management", "reporting", []string{"GenerateReport", "ExportCSV"})

	if result.FeatureName != "reporting" {
		t.Errorf("FeatureName = %q", result.FeatureName)
	}

	// Should have feature.go, handler.go, service.go, feature_test.go, e2e.go, section_renderer.go.
	fileNames := make(map[string]bool)
	for _, f := range result.Files {
		fileNames[f.Path] = true
	}
	expected := []string{"feature.go", "handler.go", "service.go", "feature_test.go", "e2e.go", "section_renderer.go"}
	for _, name := range expected {
		if !fileNames[name] {
			t.Errorf("missing %s", name)
		}
	}

	// Service should contain use case stubs.
	for _, f := range result.Files {
		if f.Path == "service.go" {
			if !strings.Contains(f.Content, "GenerateReport") {
				t.Error("service.go should contain GenerateReport use case")
			}
			if !strings.Contains(f.Content, "ExportCSV") {
				t.Error("service.go should contain ExportCSV use case")
			}
		}
	}
}

func TestIsNumericType(t *testing.T) {
	if !IsNumericType("integer") {
		t.Error("integer should be numeric")
	}
	if !IsNumericType("decimal") {
		t.Error("decimal should be numeric")
	}
	if IsNumericType("string") {
		t.Error("string should not be numeric")
	}
}

func TestTestValueForType(t *testing.T) {
	if v := TestValueForType("string"); v != `"test-value"` {
		t.Errorf("got %q", v)
	}
	if v := TestValueForType("integer"); v != "42" {
		t.Errorf("got %q", v)
	}
	if v := TestValueForType("boolean"); v != "true" {
		t.Errorf("got %q", v)
	}
}
