// Implements: REQ-002, REQ-016.
// Per: ADR-0017 (composition through dependency injection), ADR-0029 (file purpose declaration).
// Discipline: C-14.

package scaffold

import (
	"fmt"
	"strings"
)

// generateEntityCode generates the Go source for an entity.
func generateEntityCode(moduleName, entityName string, fields []Field) (string, error) {
	snakeName := ToSnakeCase(entityName)
	tableName := snakeName + "s"

	var fieldLines strings.Builder
	var mcpQueryFields strings.Builder
	mcpSemanticTags := []string{moduleName, snakeName}
	needsJSONImport := false
	needsTimeImport := false
	needsUUIDImport := false

	for _, f := range fields {
		typeInfo, err := ResolveType(f.Type)
		if err != nil {
			return "", fmt.Errorf("field %q: %w", f.Name, err)
		}
		goType := typeInfo.GoType
		if goType == "json.RawMessage" {
			needsJSONImport = true
		}
		if goType == "time.Time" {
			needsTimeImport = true
		}
		if goType == "uuid.UUID" {
			needsUUIDImport = true
		}
		gormTag := fmt.Sprintf(`gorm:"type:%s"`, typeInfo.GORMType)
		if f.Required {
			gormTag = fmt.Sprintf(`gorm:"type:%s;not null"`, typeInfo.GORMType)
		}
		jsonTag := fmt.Sprintf(`json:"%s"`, f.Name)
		docTag := ""
		if f.Description != "" {
			docTag = fmt.Sprintf(` doc:"%s"`, f.Description)
		}

		fmt.Fprintf(&fieldLines, "\t%s %s `%s %s%s`\n",
			ToPascalCase(f.Name), goType, gormTag, jsonTag, docTag)

		operators := "mcp.DefaultMCPQueryOperators()"
		if typeInfo.IsNumeric {
			operators = "mcp.NumericMCPQueryOperators()"
		} else if f.Type == "string" || f.Type == "text" {
			operators = "mcp.StringMCPQueryOperators()"
		}
		fmt.Fprintf(&mcpQueryFields, "\t\t{Name: %q, Type: %q, Description: %q, Operators: %s, Sortable: true, Filterable: true},\n",
			f.Name, f.Type, f.Description, operators)
	}

	tagsStr := `"` + strings.Join(mcpSemanticTags, `", "`) + `"`

	importLines := make([]string, 0, 6)
	if needsJSONImport {
		importLines = append(importLines, "\t\"encoding/json\"")
	}
	if needsTimeImport {
		importLines = append(importLines, "\t\"time\"")
	}
	if needsUUIDImport {
		importLines = append(importLines, "\t\"github.com/google/uuid\"")
	}
	if len(importLines) > 0 {
		importLines = append(importLines, "")
	}
	importLines = append(
		importLines,
		"\t\"example.com/platformkit/backend-kit/core/entity/providers/base\"",
		"\t\"example.com/platformkit/backend-kit/core/mcp\"",
	)
	imports := strings.Join(importLines, "\n")

	return fmt.Sprintf(`package entities

import (
%s
)

var _ mcp.MCPEnabled = (*%s)(nil)
var _ mcp.MCPQueryable = (*%s)(nil)

// %s represents a %s in the %s module.
type %s struct {
	base.BaseEntity[*%s] `+"`"+`gorm:"embedded"`+"`"+`

%s}

func (%s) TableName() string {
	return "%s"
}

func (e *%s) MCPToolName() string {
	return "%s"
}

func (e *%s) MCPDescription() string {
	return "%s entity in the %s module."
}

func (e *%s) MCPSemanticTags() []string {
	return []string{%s}
}

func (e *%s) MCPQueryFields() []mcp.MCPQueryField {
	return []mcp.MCPQueryField{
%s	}
}
`, imports,
		entityName, entityName,
		entityName, snakeName, moduleName,
		entityName,
		entityName, fieldLines.String(), entityName, tableName,
		entityName, snakeName,
		entityName, entityName, moduleName,
		entityName, tagsStr,
		entityName, mcpQueryFields.String()), nil
}

// generateEntityTestCode generates a test file for an entity.
func generateEntityTestCode(moduleName, entityName string, _ []Field) string {
	snakeName := ToSnakeCase(entityName)

	return fmt.Sprintf(
		`package entities

import (
	"testing"

	"example.com/platformkit/backend-kit/core/mcp"
)

// Compile-time interface assertions
var _ mcp.MCPEnabled = (*%s)(nil)
var _ mcp.MCPQueryable = (*%s)(nil)

func TestNew%s_TableName(t *testing.T) {
	e := &%s{}
	if got := e.TableName(); got != "%ss" {
		t.Errorf("TableName() = %%q, want %%q", got, "%ss")
	}
}

func TestNew%s_MCPToolName(t *testing.T) {
	e := &%s{}
	if got := e.MCPToolName(); got != "%s" {
		t.Errorf("MCPToolName() = %%q, want %%q", got, "%s")
	}
}

func TestNew%s_MCPDescription(t *testing.T) {
	e := &%s{}
	desc := e.MCPDescription()
	if desc == "" {
		t.Error("MCPDescription() should not be empty")
	}
}

func TestNew%s_MCPSemanticTags(t *testing.T) {
	e := &%s{}
	tags := e.MCPSemanticTags()
	if len(tags) == 0 {
		t.Error("MCPSemanticTags() should not be empty")
	}
	// Should include module and entity name
	found := map[string]bool{}
	for _, tag := range tags {
		found[tag] = true
	}
	if !found["%s"] {
		t.Errorf("MCPSemanticTags() should include module name %%q", "%s")
	}
}

func TestNew%s_MCPQueryFields(t *testing.T) {
	e := &%s{}
	fields := e.MCPQueryFields()
	if len(fields) == 0 {
		t.Error("MCPQueryFields() should not be empty")
	}
	// Verify each field has required attributes
	for _, f := range fields {
		if f.Name == "" {
			t.Error("MCPQueryField.Name should not be empty")
		}
		if f.Type == "" {
			t.Errorf("MCPQueryField %%q has empty Type", f.Name)
		}
	}
}
`,
		entityName, entityName,
		entityName, entityName, snakeName, snakeName,
		entityName, entityName, snakeName, snakeName,
		entityName, entityName,
		entityName, entityName, moduleName, moduleName,
		entityName, entityName,
	)
}

// generateEntityE2ECode generates a colocated E2E config file for an entity.
func generateEntityE2ECode(moduleName, entityName string, fields []Field) string {
	snakeName := ToSnakeCase(entityName)
	basePath := "/admin/" + strings.ReplaceAll(moduleName, "_", "-") + "/" + snakeName + "s"

	var formFields strings.Builder
	var requiredFields strings.Builder
	tableColumns := make([]string, 0, len(fields))
	for _, f := range fields {
		fmt.Fprintf(&formFields, "\t\t%q: %q,\n", f.Name, fmt.Sprintf(`input[name="%s"]`, f.Name))
		if f.Required {
			if requiredFields.Len() > 0 {
				requiredFields.WriteString(", ")
			}
			fmt.Fprintf(&requiredFields, "%q", f.Name)
		}
		tableColumns = append(tableColumns, fmt.Sprintf("%q", f.Name))
	}

	return fmt.Sprintf(
		`package %s

import (
	"example.com/platformkit/frontend-kit/e2e/config"
)

// E2E is the colocated E2E configuration for %s entity.
// Use this in flow definitions for selectors, routes, and capabilities.
var E2E = config.NewEntityConfig(%q, config.EntityOptions{
	ModuleName: %q,
	BasePath:   %q,
	FormFields: map[string]string{
%s	},
	RequiredFields: []string{%s},
	TableColumns:   []string{%s},
	Capabilities: config.CRUDCapabilities{
		Create: config.Capability{Provides: []string{%q}, Requires: []string{"authenticated_user"}},
		Read:   config.Capability{Provides: []string{%q}, Requires: []string{"authenticated_user"}},
		Update: config.Capability{Provides: []string{%q}, Requires: []string{"authenticated_user"}},
		Delete: config.Capability{Provides: []string{%q}, Requires: []string{"authenticated_user"}},
	},
})
	`, snakeName,
		entityName,
		snakeName, moduleName, basePath,
		formFields.String(),
		requiredFields.String(),
		strings.Join(tableColumns, ", "),
		snakeName+"_created",
		snakeName+"_read",
		snakeName+"_updated",
		snakeName+"_deleted",
	)
}

// generateMigrationCode generates SQL migration up and down files.
func generateMigrationCode(_, entityName string, fields []Field) (string, string, error) {
	tableName := ToSnakeCase(entityName) + "s"

	// Up migration.
	var up strings.Builder
	up.WriteString("-- +goose Up\n")
	up.WriteString("-- +goose StatementBegin\n")
	fmt.Fprintf(&up, "CREATE TABLE IF NOT EXISTS %s (\n", tableName)
	up.WriteString("    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),\n")
	up.WriteString("    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),\n")
	up.WriteString("    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),\n")
	up.WriteString("    deleted_at TIMESTAMPTZ,\n")
	up.WriteString("    version INTEGER NOT NULL DEFAULT 1,\n")
	up.WriteString("    tenant_id UUID NOT NULL,\n")
	up.WriteString("    created_by VARCHAR(255),\n")
	up.WriteString("    updated_by VARCHAR(255),\n")

	var indexFields []string
	for _, f := range fields {
		typeInfo, err := ResolveType(f.Type)
		if err != nil {
			return "", "", fmt.Errorf("field %q: %w", f.Name, err)
		}
		sqlType := typeInfo.SQLType
		constraint := ""
		if f.Required {
			constraint = " NOT NULL"
			indexFields = append(indexFields, ToSnakeCase(f.Name))
		}
		fmt.Fprintf(&up, "    %s %s%s,\n", ToSnakeCase(f.Name), sqlType, constraint)
	}

	// Remove trailing comma from last field.
	upStr := up.String()
	lastComma := strings.LastIndex(upStr, ",\n")
	if lastComma > 0 {
		upStr = upStr[:lastComma] + "\n"
	}

	var upBuilder strings.Builder
	upBuilder.WriteString(upStr)
	upBuilder.WriteString(");\n")
	upBuilder.WriteString("-- +goose StatementEnd\n\n")

	// Indexes.
	fmt.Fprintf(&upBuilder, "CREATE INDEX IF NOT EXISTS idx_%s_tenant_id ON %s(tenant_id) WHERE deleted_at IS NULL;\n", tableName, tableName)
	fmt.Fprintf(&upBuilder, "CREATE INDEX IF NOT EXISTS idx_%s_created_at ON %s(created_at) WHERE deleted_at IS NULL;\n", tableName, tableName)
	for _, field := range indexFields {
		fmt.Fprintf(&upBuilder, "CREATE INDEX IF NOT EXISTS idx_%s_%s ON %s(%s) WHERE deleted_at IS NULL;\n", tableName, field, tableName, field)
	}

	upBuilder.WriteString("\n-- Auto-update updated_at\n")
	fmt.Fprintf(&upBuilder, "CREATE OR REPLACE FUNCTION update_%s_updated_at()\n", tableName)
	upBuilder.WriteString("RETURNS TRIGGER AS $$\n")
	upBuilder.WriteString("BEGIN\n")
	upBuilder.WriteString("    NEW.updated_at = NOW();\n")
	upBuilder.WriteString("    RETURN NEW;\n")
	upBuilder.WriteString("END;\n")
	upBuilder.WriteString("$$ LANGUAGE plpgsql;\n\n")
	fmt.Fprintf(&upBuilder, "CREATE TRIGGER trigger_%s_updated_at\n", tableName)
	fmt.Fprintf(&upBuilder, "    BEFORE UPDATE ON %s\n", tableName)
	fmt.Fprintf(&upBuilder, "    FOR EACH ROW EXECUTE FUNCTION update_%s_updated_at();\n", tableName)

	// Down migration.
	var down strings.Builder
	down.WriteString("-- +goose Down\n")
	fmt.Fprintf(&down, "DROP TRIGGER IF EXISTS trigger_%s_updated_at ON %s;\n", tableName, tableName)
	fmt.Fprintf(&down, "DROP FUNCTION IF EXISTS update_%s_updated_at();\n", tableName)
	fmt.Fprintf(&down, "DROP TABLE IF EXISTS %s;\n", tableName)

	return upBuilder.String(), down.String(), nil
}

// generateFeatureFiles generates the core feature files (feature.go, handler.go, service.go).
func generateFeatureFiles(moduleName, featureName string, useCases []string) []GeneratedFile {
	pascalFeature := ToPascalCase(featureName)
	pascalModule := ToPascalCase(moduleName)

	featureGo := fmt.Sprintf(
		`package %s

// feature.go - feature composition and route ownership.
// Per: ADR-0017.
// Discipline: C-14.

import (
	"example.com/platformkit/backend-kit/app/module"
	"example.com/platformkit/backend-kit/app/module/helpers"
)

// New%sFeature creates the %s feature using FeatureBuilder.
func New%sFeature() module.Feature {
	b := helpers.NewFeatureBuilder(%q, module.FeatureMetadata{
		ID:          %q,
		Name:        "%s",
		Description: "%s feature for %s",
		Version:     "1.0.0",
		Category:    %q,
		Tags:        []string{%q, %q},
		Enabled:     true,
	})

	// Register entities (uncomment and adapt):
	// helpers.RegisterEntity[*entities.MyEntity](b, helpers.EntityConfig{
	//     Name:      "MyEntity",
	//     EnableMCP: true,
	// })

	// Domain service
	b.Provider(NewService)

	// Handler with automatic route registration
	helpers.RouteHandler[*Handler](b, NewHandler)

	// Section renderer for the generated admin surface.
	helpers.SectionRenderer[*%sSectionRenderer](b, New%sSectionRenderer)

	// Capabilities
	b.Service("%sService", "1.0.0", "%s service", "Service").
		Permissions("%s:view", "%s:manage")

	return b.Build()
}
`, featureName,
		pascalFeature, featureName,
		pascalFeature, moduleName,
		featureName, pascalFeature,
		pascalFeature, pascalModule,
		moduleName, moduleName, featureName,
		pascalFeature, pascalFeature,
		pascalFeature, pascalFeature,
		moduleName, moduleName,
	)

	handlerGo := fmt.Sprintf(`package %s

// handler.go - HTTP boundary for the feature.
// Per: ADR-0017.
// Discipline: C-14.

import (
	"github.com/danielgtaylor/huma/v2"
)

// Handler handles HTTP requests for the %s feature.
type Handler struct {
	service *Service
}

// NewHandler creates a new handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes implements helpers.RouteRegistrar.
func (h *Handler) RegisterRoutes(api huma.API) {
	// ADD YOUR ROUTES HERE. Example:
	// huma.Register(api, huma.Operation{
	//     OperationID: "list-%s",
	//     Method:      http.MethodGet,
	//     Path:        "/api/v1/%s/%s",
	//     Summary:     "List %s",
	// }, h.List)
}
`, featureName,
		featureName,
		featureName, moduleName, featureName, featureName)

	// "context" is only needed when there are use-case stubs that take
	// a ctx parameter — without use cases the import is unused and the
	// scaffolded module fails to compile.
	contextImport := ""
	if len(useCases) > 0 {
		contextImport = "\t\"context\"\n\n"
	}

	var serviceGo strings.Builder
	serviceGo.WriteString(fmt.Sprintf(`package %s

// service.go - feature application service boundary.
// Per: ADR-0007, ADR-0017.
// Discipline: C-14.

import (
%s	"example.com/platformkit/backend-kit/observability/logger"
)

// Service implements the business logic for %s.
type Service struct {
	logger logger.Logger
}

// NewService creates a new service.
func NewService(logger logger.Logger) *Service {
	return &Service{logger: logger}
}
`, featureName, contextImport, featureName))

	// Add use case stubs.
	for _, uc := range useCases {
		ucPascal := ToPascalCase(uc)
		serviceGo.WriteString(fmt.Sprintf(`
// %s executes the %s use case.
func (s *Service) %s(ctx context.Context) error {
	s.logger.Info(ctx, "Executing %s")
	// IMPLEMENT: add business logic here
	return nil
}
`, ucPascal, uc, ucPascal, uc))
	}

	return []GeneratedFile{
		{Path: "feature.go", Content: featureGo},
		{Path: "handler.go", Content: handlerGo},
		{Path: "service.go", Content: serviceGo.String()},
	}
}

// generateFeatureTestCode generates a test file for a feature.
func generateFeatureTestCode(_, featureName string) string {
	pascalFeature := ToPascalCase(featureName)

	return fmt.Sprintf(
		`package %s

// feature_test.go - feature contract tests.
// Per: ADR-0017.
// Discipline: C-14.

import (
	"testing"
)

func TestNew%sFeature(t *testing.T) {
	f := New%sFeature()
	if f == nil {
		t.Fatal("New%sFeature() returned nil")
	}

	meta := f.Metadata()
	if meta.ID != %q {
		t.Errorf("Feature ID = %%q, want %%q", meta.ID, %q)
	}
	if meta.Name != %q {
		t.Errorf("Feature Name = %%q, want %%q", meta.Name, %q)
	}
	if !meta.Enabled {
		t.Error("Feature should be enabled by default")
	}
	if meta.Version == "" {
		t.Error("Feature Version should not be empty")
	}
}

func TestNew%sFeature_Capabilities(t *testing.T) {
	f := New%sFeature()
	caps := f.Capabilities()

	if len(caps.Services) == 0 {
		t.Error("Feature should declare at least one service")
	}
	if len(caps.Permissions) == 0 {
		t.Error("Feature should declare at least one permission")
	}
}
`, featureName,
		pascalFeature, pascalFeature, pascalFeature,
		featureName, featureName,
		pascalFeature, pascalFeature,
		pascalFeature, pascalFeature,
	)
}

// generateFeatureE2ECode generates a colocated E2E config for a feature.
func generateFeatureE2ECode(moduleName, featureName string) string {
	return fmt.Sprintf(
		`package %s

import (
	"example.com/platformkit/frontend-kit/e2e/config"
)

// E2E is the colocated E2E configuration for the %s feature.
// Use this in flow definitions for pages, actions, routes, and capabilities.
var E2E = config.NewFeatureConfig(%q, config.FeatureOptions{
	ModuleName: %q,
	Pages: map[string]string{
		"main": %q,
	},
	Actions: map[string]string{
		"submit": %q,
	},
	Routes: map[string]string{
		"main": "/admin/%s/%s",
	},
	Capabilities: map[string]config.Capability{
		"view": {
			Provides: []string{"%s.%s.viewed"},
			Requires: []string{"authenticated_user"},
		},
	},
})
`, featureName,
		featureName,
		featureName, moduleName,
		fmt.Sprintf(`[data-page="%s"]`, featureName),
		fmt.Sprintf(`[data-page="%s"] button[type="submit"]`, featureName),
		strings.ReplaceAll(moduleName, "_", "-"), strings.ReplaceAll(featureName, "_", "-"),
		moduleName, featureName,
	)
}

// generateSectionRendererCode generates a section_renderer.go for a feature.
func generateSectionRendererCode(moduleName, featureName string) string {
	pascalFeature := ToPascalCase(featureName)
	sectionPrefix := strings.ReplaceAll(moduleName, "_management", "")

	return fmt.Sprintf(
		`package %s

import (
	"context"
	"strings"

	"example.com/platformkit/backend-kit/app/module"
	"example.com/platformkit/backend-kit/app/module/helpers"
)

// %sSectionRenderer renders admin sections for the %s feature.
// It delegates to GenericCRUDSectionRenderer for consistent Renderable output.
type %sSectionRenderer struct {
	inner *helpers.GenericCRUDSectionRenderer
}

// New%sSectionRenderer creates a new section renderer.
func New%sSectionRenderer() *%sSectionRenderer {
	return &%sSectionRenderer{
		inner: helpers.NewGenericCRUDSectionRenderer(helpers.CRUDRendererConfig{
			ModuleName: %q,
			EntityName: %q,
			PluralName: %q,
			BasePath:   "/api/v1/%s/%s",
			AdminPath:  "/admin/%s/%s",
			Columns:    nil, // CUSTOMIZE: add columns for your entity
		}),
	}
}

// CanRender implements module.SectionRenderer.
func (r *%sSectionRenderer) CanRender(sectionID string) bool {
	if r.inner.CanRender(sectionID) {
		return true
	}
	return strings.HasPrefix(sectionID, "%s-%s")
}

// Render implements module.SectionRenderer.
func (r *%sSectionRenderer) Render(ctx context.Context, sectionID string, requestPath string) module.Renderable {
	return r.inner.Render(ctx, sectionID, requestPath)
}

// Priority implements module.SectionRenderer.
func (r *%sSectionRenderer) Priority() int {
	return r.inner.Priority()
}

var _ module.SectionRenderer = (*%sSectionRenderer)(nil)
`, featureName,
		pascalFeature, featureName,
		pascalFeature,
		pascalFeature, pascalFeature, pascalFeature,
		pascalFeature,
		moduleName, featureName, pascalFeature,
		moduleName, featureName,
		strings.ReplaceAll(moduleName, "_", "-"), strings.ReplaceAll(featureName, "_", "-"),
		pascalFeature, sectionPrefix, featureName,
		pascalFeature,
		pascalFeature,
		pascalFeature,
	)
}

// generateRegistrationCode generates the code snippet to register a module in the Bundle registry.
func generateRegistrationCode(moduleName string) map[string]string {
	return map[string]string{
		"bundleEntry":  fmt.Sprintf(`{ID: "%s", New: %s.NewModule, Features: %s.GetFeatures},`, moduleName, moduleName, moduleName),
		"import":       fmt.Sprintf(`%s "example.com/platformkit/business-modules/%s"`, moduleName, moduleName),
		"usage":        fmt.Sprintf("platformmodules.NewModuleSet().WithModules(%q)", moduleName),
		"file":         "platformkit-business-modules/catalog/moduleregistry/bundle.go",
		"instructions": "1. Add the module import to catalog/moduleregistry/bundle.go\n2. Add a defaultEntries row with ID, New, and Features\n3. Add the ID to defaultModuleIDs only if it belongs in the default preset",
	}
}
