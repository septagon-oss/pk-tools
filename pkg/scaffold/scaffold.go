// Implements: REQ-002.
// Per: ADR-0029 (file purpose declaration).
// Discipline: C-14.

package scaffold

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Field represents a field definition for entity scaffolding.
type Field struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// GeneratedFile represents a single file produced by scaffold generation.
type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ModuleResult holds the output of module generation.
type ModuleResult struct {
	ModuleName       string            `json:"moduleName"`
	Files            []GeneratedFile   `json:"files"`
	RegistrationCode map[string]string `json:"registrationCode"`
}

// ModuleOptions is the canonical module-scaffolding input contract.
type ModuleOptions struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Archetype   string   `json:"archetype"`
	Tier        string   `json:"tier"`   // experimental | supported | core-certified
	Domain      string   `json:"domain"` // identity-access | commerce | content-sites | ...
	Features    []string `json:"features"`
	Tags        []string `json:"tags"`
	Events      []string `json:"events"`     // dot-separated event names emitted by this module
	Ports       []string `json:"ports"`      // extra cross-module port interfaces this module depends on
	WithAssets  bool     `json:"withAssets"` // emit assets_embed.go, assets_loader.go, and browser/ skeleton

	ImportProfile ImportProfile `json:"-"`
}

// EntityResult holds the output of entity generation.
type EntityResult struct {
	EntityName      string          `json:"entityName"`
	ModuleName      string          `json:"moduleName"`
	Files           []GeneratedFile `json:"files"`
	Migrations      []GeneratedFile `json:"migrations"`
	RegisterSnippet string          `json:"registerSnippet"`
}

// FeatureResult holds the output of feature generation.
type FeatureResult struct {
	FeatureName string          `json:"featureName"`
	ModuleName  string          `json:"moduleName"`
	Files       []GeneratedFile `json:"files"`
}

// GenerateModule creates a complete platformkit module from a
// ModuleOptions specification. Defaults are applied for empty Category
// and Archetype so the function is safe to call with a minimal options
// struct.
func GenerateModule(opts ModuleOptions) ModuleResult {
	if opts.Category == "" {
		opts.Category = "business"
	}
	if opts.Archetype == "" {
		opts.Archetype = "service"
	}

	files := applyImportProfileToFiles(normalizeGeneratedGoFiles(generateModuleFiles(opts)), opts.ImportProfile)
	regCode := applyImportProfileToRegistrationCode(generateRegistrationCode(opts.Name), opts.ImportProfile)

	return ModuleResult{
		ModuleName:       opts.Name,
		Files:            files,
		RegistrationCode: regCode,
	}
}

// GenerateEntity creates an entity with BaseEntity, MCP interfaces, tests, and migrations.
func GenerateEntity(moduleName, entityName string, fields []Field) (EntityResult, error) {
	return GenerateEntityWithProfile(moduleName, entityName, fields, ImportProfile{})
}

// GenerateEntityWithProfile creates an entity and rewrites generated imports
// using the supplied workspace profile.
func GenerateEntityWithProfile(moduleName, entityName string, fields []Field, profile ImportProfile) (EntityResult, error) {
	code, err := generateEntityCode(moduleName, entityName, fields)
	if err != nil {
		return EntityResult{}, fmt.Errorf("generate entity %s: %w", entityName, err)
	}
	testCode := generateEntityTestCode(moduleName, entityName, fields)
	e2eCode := generateEntityE2ECode(moduleName, entityName, fields)
	migUp, migDown, err := generateMigrationCode(moduleName, entityName, fields)
	if err != nil {
		return EntityResult{}, fmt.Errorf("generate entity %s migration: %w", entityName, err)
	}

	registerSnippet := fmt.Sprintf(
		`helpers.RegisterEntity[*entities.%s](b, helpers.EntityConfig{
    Name:      %q,
    EnableMCP: true,
})`,
		entityName, entityName,
	)

	files := applyImportProfileToFiles(normalizeGeneratedGoFiles([]GeneratedFile{
		{Path: ToSnakeCase(entityName) + ".go", Content: code},
		{Path: ToSnakeCase(entityName) + "_test.go", Content: testCode},
		{Path: "e2e.go", Content: e2eCode},
	}), profile)
	migrations := applyImportProfileToFiles([]GeneratedFile{
		{Path: fmt.Sprintf("migrations/001_create_%s.up.sql", ToSnakeCase(entityName)+"s"), Content: migUp},
		{Path: fmt.Sprintf("migrations/001_create_%s.down.sql", ToSnakeCase(entityName)+"s"), Content: migDown},
	}, profile)

	return EntityResult{
		EntityName:      entityName,
		ModuleName:      moduleName,
		Files:           files,
		Migrations:      migrations,
		RegisterSnippet: applyImportProfile(registerSnippet, profile),
	}, nil
}

// GenerateFeature creates a feature with FeatureBuilder, handler, service, tests, and renderer.
func GenerateFeature(moduleName, featureName string, useCases []string) FeatureResult {
	return GenerateFeatureWithProfile(moduleName, featureName, useCases, ImportProfile{})
}

// GenerateFeatureWithProfile creates a feature and rewrites generated imports
// using the supplied workspace profile.
func GenerateFeatureWithProfile(moduleName, featureName string, useCases []string, profile ImportProfile) FeatureResult {
	files := generateFeatureFiles(moduleName, featureName, useCases)
	testFile := generateFeatureTestCode(moduleName, featureName)
	e2eFile := generateFeatureE2ECode(moduleName, featureName)
	rendererFile := generateSectionRendererCode(moduleName, featureName)
	files = append(
		files,
		GeneratedFile{Path: "feature_test.go", Content: testFile},
		GeneratedFile{Path: "e2e.go", Content: e2eFile},
		GeneratedFile{Path: "section_renderer.go", Content: rendererFile},
	)
	return FeatureResult{
		FeatureName: featureName,
		ModuleName:  moduleName,
		Files:       applyImportProfileToFiles(normalizeGeneratedGoFiles(files), profile),
	}
}

// AddEntityToFeature creates entity files intended to be added to an existing feature.
func AddEntityToFeature(moduleName, featureName, entityName string, fields []Field) (EntityResult, error) {
	return AddEntityToFeatureWithProfile(moduleName, featureName, entityName, fields, ImportProfile{})
}

// AddEntityToFeatureWithProfile creates entity files for an existing feature
// and rewrites generated imports using the supplied workspace profile.
func AddEntityToFeatureWithProfile(moduleName, featureName, entityName string, fields []Field, profile ImportProfile) (EntityResult, error) {
	code, err := generateEntityCode(moduleName, entityName, fields)
	if err != nil {
		return EntityResult{}, fmt.Errorf("generate entity %s for feature %s: %w", entityName, featureName, err)
	}
	testCode := generateEntityTestCode(moduleName, entityName, fields)
	migUp, migDown, err := generateMigrationCode(moduleName, entityName, fields)
	if err != nil {
		return EntityResult{}, fmt.Errorf("generate entity %s migration for feature %s: %w", entityName, featureName, err)
	}

	registerSnippet := fmt.Sprintf(
		`helpers.RegisterEntity[*entities.%s](b, helpers.EntityConfig{
    Name:      %q,
    EnableMCP: true,
})`,
		entityName, entityName,
	)

	files := applyImportProfileToFiles(normalizeGeneratedGoFiles([]GeneratedFile{
		{Path: ToSnakeCase(entityName) + ".go", Content: code},
		{Path: ToSnakeCase(entityName) + "_test.go", Content: testCode},
	}), profile)
	migrations := applyImportProfileToFiles([]GeneratedFile{
		{Path: fmt.Sprintf("migrations/001_create_%s.up.sql", ToSnakeCase(entityName)+"s"), Content: migUp},
		{Path: fmt.Sprintf("migrations/001_create_%s.down.sql", ToSnakeCase(entityName)+"s"), Content: migDown},
	}, profile)

	return EntityResult{
		EntityName:      entityName,
		ModuleName:      moduleName,
		Files:           files,
		Migrations:      migrations,
		RegisterSnippet: applyImportProfile(registerSnippet, profile),
	}, nil
}

// WriteFiles writes a slice of GeneratedFile to disk under the given base directory.
// If dryRun is true, it prints what would be written without creating files.
func WriteFiles(baseDir string, files []GeneratedFile, dryRun bool, output io.Writer) error {
	for _, f := range files {
		fullPath := filepath.Join(baseDir, f.Path)
		if dryRun {
			if output != nil {
				if _, err := fmt.Fprintf(output, "  [dry-run] %s (%d bytes)\n", fullPath, len(f.Content)); err != nil {
					return fmt.Errorf("write dry-run output for %s: %w", fullPath, err)
				}
			}
			continue
		}

		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}

		if err := os.WriteFile(fullPath, []byte(f.Content), 0o644); err != nil {
			return fmt.Errorf("write file %s: %w", fullPath, err)
		}
		if output != nil {
			if _, err := fmt.Fprintf(output, "  created %s\n", fullPath); err != nil {
				return fmt.Errorf("write output for %s: %w", fullPath, err)
			}
		}
	}
	return nil
}
