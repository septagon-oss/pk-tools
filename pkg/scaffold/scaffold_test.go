// Validates: REQ-002.
// Per: ADR-0029 (file purpose declaration).
// Discipline: C-14.

package scaffold

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func mustGenerateModule(t *testing.T, opts ModuleOptions) ModuleResult {
	t.Helper()
	result, err := GenerateModule(opts)
	if err != nil {
		t.Fatalf("GenerateModule: %v", err)
	}
	return result
}

func mustGenerateEntity(t *testing.T, moduleName, entityName string, fields []Field) EntityResult {
	t.Helper()
	result, err := GenerateEntity(EntityOptions{
		ModuleName:        moduleName,
		Name:              entityName,
		TableName:         ToSnakeCase(entityName) + "s",
		Fields:            fields,
		MigrationSequence: 1,
	})
	if err != nil {
		t.Fatalf("GenerateEntity: %v", err)
	}
	return result
}

func mustGenerateFeature(t *testing.T, moduleName, featureName string, useCases []string) FeatureResult {
	t.Helper()
	result, err := GenerateFeature(FeatureOptions{
		ModuleName: moduleName,
		Name:       featureName,
		UseCases:   useCases,
	})
	if err != nil {
		t.Fatalf("GenerateFeature: %v", err)
	}
	return result
}

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
		{"APIKey", "api_key"},
		{"HTTPServer", "http_server"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := ToSnakeCase(tt.input); got != tt.want {
			t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveType(t *testing.T) {
	tests := []struct {
		input    string
		wantGo   string
		wantGORM string
		wantSQL  string
		numeric  bool
	}{
		{"string", "string", "varchar(255)", "VARCHAR(255)", false},
		{"integer", "int64", "bigint", "BIGINT", true},
		{"decimal", "float64", "decimal(10,2)", "DECIMAL(19,4)", true},
		{"boolean", "bool", "boolean", "BOOLEAN DEFAULT false", false},
		{"datetime", "time.Time", "timestamptz", "TIMESTAMPTZ", true},
		{"uuid", "uuid.UUID", "uuid", "UUID", false},
	}
	for _, tt := range tests {
		got, err := ResolveType(tt.input)
		if err != nil {
			t.Fatalf("ResolveType(%q): %v", tt.input, err)
		}
		if got.GoType != tt.wantGo || got.GORMType != tt.wantGORM || got.SQLType != tt.wantSQL || got.IsNumeric != tt.numeric {
			t.Errorf("ResolveType(%q) = %+v", tt.input, got)
		}
	}
}

func TestResolveTypeRejectsUnknownName(t *testing.T) {
	if _, err := ResolveType("unknown"); err == nil {
		t.Fatal("ResolveType(unknown) should fail")
	} else if !strings.Contains(err.Error(), "canonical types:") {
		t.Fatalf("ResolveType(unknown) error = %q; want canonical type inventory", err)
	}
}

func TestResolveTypeRejectsRetiredAliases(t *testing.T) {
	for _, alias := range []string{"int", "number", "float", "float64", "bool", "timestamp", "json"} {
		if _, err := ResolveType(alias); err == nil {
			t.Errorf("ResolveType(%q) accepted a retired alias", alias)
		}
	}
}

func TestJSONBTypeUsesNativeRepresentations(t *testing.T) {
	info, err := ResolveType("jsonb")
	if err != nil {
		t.Fatalf("ResolveType(jsonb): %v", err)
	}
	if info.GoType != "json.RawMessage" || info.GORMType != "jsonb" || !strings.HasPrefix(info.SQLType, "JSONB") {
		t.Fatalf("ResolveType(jsonb) = %+v; want native JSON representations", info)
	}

	result := mustGenerateEntity(t, "content_management", "Document", []Field{{Name: "metadata", Type: "jsonb"}})
	if !strings.Contains(result.Files[0].Content, `"encoding/json"`) || !strings.Contains(result.Files[0].Content, "Metadata json.RawMessage") {
		t.Fatalf("generated JSONB entity does not use json.RawMessage:\n%s", result.Files[0].Content)
	}
}

func TestResolveTypeVocabularyIsClosedAndComplete(t *testing.T) {
	for name := range strings.SplitSeq(canonicalTypeNames, ", ") {
		info, err := ResolveType(name)
		if err != nil {
			t.Fatalf("ResolveType(%q): %v", name, err)
		}
		if info.GoType == "" || info.GORMType == "" || info.SQLType == "" || info.E2EValue == "" {
			t.Errorf("ResolveType(%q) has an incomplete cross-layer contract: %+v", name, info)
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
		{"name", Field{}, true},
		{"name:", Field{}, true},
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
	fields, err := ParseFieldSpecs("amount:decimal,status:string,dueDate:datetime")
	if err != nil {
		t.Fatalf("ParseFieldSpecs: %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("got %d fields, want 3", len(fields))
	}
	if fields[0].Name != "amount" || fields[0].Type != "decimal" {
		t.Errorf("fields[0] = %+v", fields[0])
	}
	if fields[2].Name != "dueDate" || fields[2].Type != "datetime" {
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
	result := mustGenerateModule(t, ModuleOptions{
		Name:        "billing_management",
		Description: "Billing and subscriptions",
		Category:    "commerce",
		Archetype:   "service",
		Features:    []string{"subscriptions"},
	})

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
		"authz.go",
		"contracts/module.go",
		"contracts/permissions.go",
		"contracts/provides/doc.go",
		"dependencies.go",
		"features/README.md",
		"invocations.go",
		"metadata.go",
		"migrations/README.md",
		"module.go",
		"module_smoke_test.go",
		"surfaces.go",
	}
	for _, name := range expected {
		if !fileNames[name] {
			t.Errorf("missing %s", name)
		}
	}
	for _, retired := range []string{
		"contracts/events.go",
		"contracts/providers.go",
		"contracts/routes.go",
		"events.go",
		"migrations/001_initial.up.sql",
		"migrations/001_initial.down.sql",
		"settings_provider.go",
	} {
		if fileNames[retired] {
			t.Errorf("generated no-op or alias artifact %s", retired)
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
		Ports:           "github.com/acme/platformkit-ports/port",
	}

	result := mustGenerateModule(t, ModuleOptions{
		Name:          "billing_management",
		Description:   "Billing and subscriptions",
		Category:      "commerce",
		Archetype:     "service",
		Features:      []string{"subscriptions"},
		ImportProfile: profile,
	})

	var combined strings.Builder
	combined.WriteString(result.RegistrationCode["import"])
	for _, file := range result.Files {
		combined.WriteString("\n" + file.Content)
	}

	for _, needle := range []string{
		"github.com/acme/platformkit-backend-kit/app/module",
		"github.com/acme/platformkit-business-modules/billing_management/features/subscriptions",
		"github.com/acme/platformkit-business-modules/billing_management",
	} {
		if !strings.Contains(combined.String(), needle) {
			t.Fatalf("profiled scaffold output missing %q", needle)
		}
	}
	if strings.Contains(combined.String(), "example.com/platformkit/") {
		t.Fatal("profiled scaffold output still contains neutral import roots")
	}
}

func TestGenerateModuleGoFilesParse(t *testing.T) {
	result := mustGenerateModule(t, ModuleOptions{
		Name:        "stock_exchange_management",
		Description: "Stock exchange module",
		Features:    []string{"orders"},
		Archetype:   "service",
	})

	for _, file := range result.Files {
		if !strings.HasSuffix(file.Path, ".go") {
			continue
		}
		if _, err := parser.ParseFile(token.NewFileSet(), file.Path, file.Content, parser.AllErrors); err != nil {
			t.Errorf("generated %s does not parse: %v", file.Path, err)
		}
	}
}

func TestGenerateModuleRegistryArchetypeOmitsMigrations(t *testing.T) {
	result := mustGenerateModule(t, ModuleOptions{
		Name:        "translation_management",
		Description: "Translations",
		Category:    "localization",
		Archetype:   "registry",
	})

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
	result := mustGenerateModule(t, ModuleOptions{
		Name:        "billing_management",
		Description: "Billing and subscriptions",
		Category:    "commerce",
		Archetype:   "service",
		Features:    []string{"subscriptions"},
	})

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
	result := mustGenerateEntity(t, "billing_management", "Invoice", fields)

	if result.EntityName != "Invoice" {
		t.Errorf("EntityName = %q", result.EntityName)
	}

	// Entity files are colocated in one package and use an entity-specific E2E
	// name, so multiple entities never overwrite or redeclare each other.
	if len(result.Files) != 3 {
		t.Errorf("got %d files, want 3", len(result.Files))
	}
	if result.Files[2].Path != "invoice_e2e.go" {
		t.Errorf("entity E2E path = %q; want invoice_e2e.go", result.Files[2].Path)
	}
	if len(result.Migrations) != 2 {
		t.Errorf("got %d migrations, want 2", len(result.Migrations))
	}
	if result.Migrations[0].Path != "migrations/0001_create_invoices.up.sql" || result.Migrations[1].Path != "migrations/0001_create_invoices.down.sql" {
		t.Fatalf("migration paths = %q, %q; want explicit four-digit sequence", result.Migrations[0].Path, result.Migrations[1].Path)
	}

	// Entity code should reference billing_management.
	entityCode := result.Files[0].Content
	if !strings.Contains(entityCode, "billing_management") {
		t.Error("entity code should reference module name")
	}
	if !strings.Contains(entityCode, "MCPToolName") {
		t.Error("entity code should contain MCP methods")
	}
	e2eCode := result.Files[2].Content
	for _, canonical := range []string{"package entities", "var InvoiceE2E ="} {
		if !strings.Contains(e2eCode, canonical) {
			t.Errorf("entity E2E config missing collision-free contract %q", canonical)
		}
	}
	for _, retired := range []string{"DefaultCRUDCapabilities", "CRUDCapabilities:", "TableColumns: map[string]string"} {
		if strings.Contains(e2eCode, retired) {
			t.Errorf("entity E2E config contains retired contract %q", retired)
		}
	}
	for _, canonical := range []string{"TableColumns:   []string", "Capabilities: config.CRUDCapabilities", `ModuleName: "billing_management"`} {
		if !strings.Contains(e2eCode, canonical) {
			t.Errorf("entity E2E config missing canonical contract %q", canonical)
		}
	}

	// Migration should contain CREATE TABLE.
	if !strings.Contains(result.Migrations[0].Content, "CREATE TABLE") {
		t.Error("migration up should contain CREATE TABLE")
	}
	if !strings.Contains(result.Migrations[1].Content, "DROP TABLE") {
		t.Error("migration down should contain DROP TABLE")
	}
	for _, permissive := range []string{"IF NOT EXISTS", "IF EXISTS", "OR REPLACE"} {
		for _, migration := range result.Migrations {
			if strings.Contains(migration.Content, permissive) {
				t.Errorf("%s contains permissive migration clause %q", migration.Path, permissive)
			}
		}
	}

	// Register snippet should not be empty.
	if result.RegisterSnippet == "" {
		t.Error("RegisterSnippet should not be empty")
	}
}

func TestGenerateFeature(t *testing.T) {
	result := mustGenerateFeature(t, "billing_management", "reporting", []string{"GenerateReport", "ExportCSV"})

	if result.FeatureName != "reporting" {
		t.Errorf("FeatureName = %q", result.FeatureName)
	}

	// A feature with explicit use cases gets a fail-fast service boundary. Route
	// handlers are generated only from real route specifications, never as empty
	// registrars.
	fileNames := make(map[string]bool)
	for _, f := range result.Files {
		fileNames[f.Path] = true
	}
	expected := []string{"feature.go", "service.go", "feature_test.go", "e2e.go"}
	for _, name := range expected {
		if !fileNames[name] {
			t.Errorf("missing %s", name)
		}
	}
	if fileNames["handler.go"] {
		t.Error("feature scaffold emitted an empty route registrar")
	}
	if fileNames["section_renderer.go"] {
		t.Error("feature scaffold emitted an unconfigured section renderer")
	}

	// Service boundaries must fail explicitly until domain logic is supplied.
	for _, f := range result.Files {
		if f.Path == "service.go" {
			if !strings.Contains(f.Content, "GenerateReport") {
				t.Error("service.go should contain GenerateReport use case")
			}
			if !strings.Contains(f.Content, "ExportCSV") {
				t.Error("service.go should contain ExportCSV use case")
			}
			for _, canonical := range []string{"errors.ErrUnsupported", "logger is required"} {
				if !strings.Contains(f.Content, canonical) {
					t.Errorf("service.go missing fail-fast contract %q", canonical)
				}
			}
			if strings.Contains(f.Content, "return nil\n") {
				t.Error("service.go reports false success from an unimplemented use case")
			}
		}
		if f.Path == "e2e.go" {
			if strings.Contains(f.Content, "FlowCapability") || !strings.Contains(f.Content, "map[string]config.Capability") {
				t.Error("e2e.go should use the canonical config.Capability contract")
			}
		}
	}
}

func TestGenerateEntityRejectsUnknownFieldType(t *testing.T) {
	_, err := GenerateEntity(EntityOptions{
		ModuleName:        "billing_management",
		Name:              "Invoice",
		TableName:         "invoices",
		Fields:            []Field{{Name: "amount", Type: "currency_guess"}},
		MigrationSequence: 1,
	})
	if err == nil {
		t.Fatal("GenerateEntity should reject an unknown field type")
	}
	if !strings.Contains(err.Error(), `field "amount"`) || !strings.Contains(err.Error(), `"currency_guess"`) {
		t.Fatalf("GenerateEntity error = %q; want field and type context", err)
	}
}

func TestGenerateEntityUsesCanonicalAcronymFileNames(t *testing.T) {
	result := mustGenerateEntity(t, "auth_management", "APIKey", []Field{{Name: "name", Type: "string"}})
	if result.Files[0].Path != "api_key.go" || result.Files[1].Path != "api_key_test.go" {
		t.Fatalf("GenerateEntity APIKey paths = %q, %q; want api_key.go, api_key_test.go", result.Files[0].Path, result.Files[1].Path)
	}
	if !strings.Contains(result.Files[0].Content, `return "api_keys"`) {
		t.Fatalf("GenerateEntity APIKey table name is not canonical: %s", result.Files[0].Content)
	}
}

func TestGenerateEntityUsesExplicitIrregularTableName(t *testing.T) {
	result, err := GenerateEntity(EntityOptions{
		ModuleName:        "policy_management",
		Name:              "Policy",
		TableName:         "policies",
		Fields:            []Field{{Name: "title", Type: "string"}},
		MigrationSequence: 27,
	})
	if err != nil {
		t.Fatalf("GenerateEntity: %v", err)
	}
	if !strings.Contains(result.Files[0].Content, `return "policies"`) {
		t.Fatalf("entity ignored explicit table name:\n%s", result.Files[0].Content)
	}
	if result.Migrations[0].Path != "migrations/0027_create_policies.up.sql" {
		t.Fatalf("migration path = %q; want explicit table and sequence", result.Migrations[0].Path)
	}
}

func TestGeneratedGoFilesStartWithExactPurposeHeader(t *testing.T) {
	moduleResult := mustGenerateModule(t, ModuleOptions{
		Name:        "inventory_management",
		Description: "Inventory",
		Features:    []string{"items"},
	})
	entityResult := mustGenerateEntity(t, "inventory_management", "Item", []Field{{Name: "name", Type: "string"}})
	featureResult := mustGenerateFeature(t, "inventory_management", "items", []string{"ListItems"})

	files := append([]GeneratedFile{}, moduleResult.Files...)
	files = append(files, entityResult.Files...)
	files = append(files, featureResult.Files...)
	for _, file := range files {
		if !strings.HasSuffix(file.Path, ".go") {
			continue
		}
		lines := strings.Split(file.Content, "\n")
		if len(lines) < 4 {
			t.Fatalf("%s has no complete purpose header", file.Path)
		}
		verb := "// Implements: "
		if strings.HasSuffix(file.Path, "_test.go") {
			verb = "// Validates: "
		}
		if !strings.HasPrefix(lines[0], verb) {
			t.Errorf("%s first line = %q; want prefix %q", file.Path, lines[0], verb)
		}
		if !strings.HasPrefix(lines[1], "// Per: ADR-") {
			t.Errorf("%s second line = %q; want ADR reference", file.Path, lines[1])
		}
		if lines[2] != "// Discipline: C-14." {
			t.Errorf("%s third line = %q; want exact C-14 discipline", file.Path, lines[2])
		}
		if lines[3] != "" {
			t.Errorf("%s purpose header is not separated from source", file.Path)
		}
		if strings.Contains(file.Content, "%!") {
			t.Errorf("%s contains an unresolved formatting directive", file.Path)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), file.Path, file.Content, parser.AllErrors); err != nil {
			t.Errorf("generated %s does not parse: %v", file.Path, err)
		}
	}
}

func TestGenerateModuleIncludesPlatformVerticalSliceBoundaries(t *testing.T) {
	result := mustGenerateModule(t, ModuleOptions{
		Name:        "inventory_management",
		Description: "Inventory",
		Features:    []string{"items"},
	})
	files := map[string]string{}
	for _, file := range result.Files {
		files[file.Path] = file.Content
	}
	for _, path := range []string{"transactions.go", "jobs.go", "features/items/feature.go", "features/items/feature_test.go", "features/items/e2e.go"} {
		if files[path] == "" {
			t.Fatalf("generated module is missing vertical-slice file %q", path)
		}
	}
	for _, noOp := range []string{"features/items/service.go", "features/items/handler.go"} {
		if files[noOp] != "" {
			t.Fatalf("generated module contains no-op vertical-slice file %q", noOp)
		}
	}
	if !strings.Contains(files["transactions.go"], "crud.NewUnitOfWork") {
		t.Fatal("generated transaction boundary does not use crud.UnitOfWork")
	}
	if !strings.Contains(files["jobs.go"], "jobs.NewTypedHandler") {
		t.Fatal("generated job boundary does not use typed jobs")
	}
	if !strings.Contains(files["jobs.go"], "jobs.ScheduleOnce") {
		t.Fatal("generated job boundary does not use safe one-shot scheduling")
	}
	if strings.Contains(files["features/items/feature.go"], "SectionRenderer") || files["features/items/section_renderer.go"] != "" {
		t.Fatal("generated feature contains an unconfigured section renderer")
	}
}
