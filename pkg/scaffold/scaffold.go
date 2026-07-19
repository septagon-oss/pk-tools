// Implements: REQ-002.
// Per: ADR-0029 (file purpose declaration).
// Discipline: C-14.

package scaffold

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// WriteOptions is the canonical contract for materializing a generated file
// set. Existing paths are never replaced and a failed write rolls back files
// created by the current call.
type WriteOptions struct {
	BaseDir string
	Files   []GeneratedFile
	DryRun  bool
	Output  io.Writer
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
	Features    []string `json:"features"`
	Tags        []string `json:"tags"`
	Ports       []string `json:"ports"` // extra cross-module port interfaces this module depends on

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

// EntityOptions is the single input contract for entity generation.
type EntityOptions struct {
	ModuleName        string        `json:"moduleName"`
	Name              string        `json:"name"`
	TableName         string        `json:"tableName"`
	Fields            []Field       `json:"fields"`
	MigrationSequence uint          `json:"migrationSequence"`
	ImportProfile     ImportProfile `json:"-"`
}

// FeatureResult holds the output of feature generation.
type FeatureResult struct {
	FeatureName string          `json:"featureName"`
	ModuleName  string          `json:"moduleName"`
	Files       []GeneratedFile `json:"files"`
}

// FeatureOptions is the single input contract for feature generation.
type FeatureOptions struct {
	ModuleName    string        `json:"moduleName"`
	Name          string        `json:"name"`
	UseCases      []string      `json:"useCases"`
	ImportProfile ImportProfile `json:"-"`
}

// GenerateModule creates a complete platformkit module from one validated
// ModuleOptions contract.
func GenerateModule(opts ModuleOptions) (ModuleResult, error) {
	if opts.Category == "" {
		opts.Category = "business"
	}
	if opts.Archetype == "" {
		opts.Archetype = "service"
	}
	if err := validateModuleOptions(opts); err != nil {
		return ModuleResult{}, err
	}

	files := applyImportProfileToFiles(normalizeGeneratedGoFiles(generateModuleFiles(opts)), opts.ImportProfile)
	regCode := applyImportProfileToRegistrationCode(generateRegistrationCode(opts.Name), opts.ImportProfile)

	return ModuleResult{
		ModuleName:       opts.Name,
		Files:            files,
		RegistrationCode: regCode,
	}, nil
}

// GenerateEntity creates an entity with BaseEntity, MCP interfaces, tests, and
// an explicitly sequenced append-only migration pair.
func GenerateEntity(opts EntityOptions) (EntityResult, error) {
	if err := validateEntityOptions(opts); err != nil {
		return EntityResult{}, err
	}
	code, err := generateEntityCode(opts.ModuleName, opts.Name, opts.TableName, opts.Fields)
	if err != nil {
		return EntityResult{}, fmt.Errorf("generate entity %s: %w", opts.Name, err)
	}
	testCode := generateEntityTestCode(opts.ModuleName, opts.Name, opts.TableName, opts.Fields)
	e2eCode := generateEntityE2ECode(opts.ModuleName, opts.Name, opts.TableName, opts.Fields)
	migUp, migDown, err := generateMigrationCode(opts.ModuleName, opts.TableName, opts.Fields)
	if err != nil {
		return EntityResult{}, fmt.Errorf("generate entity %s migration: %w", opts.Name, err)
	}

	registerSnippet := fmt.Sprintf(
		`helpers.RegisterEntity[*entities.%s](b, helpers.EntityConfig{
    Name:      %q,
    EnableMCP: true,
})`,
		opts.Name, opts.Name,
	)

	snakeName := ToSnakeCase(opts.Name)
	files := applyImportProfileToFiles(normalizeGeneratedGoFiles([]GeneratedFile{
		{Path: snakeName + ".go", Content: code},
		{Path: snakeName + "_test.go", Content: testCode},
		{Path: snakeName + "_e2e.go", Content: e2eCode},
	}), opts.ImportProfile)
	migrationPrefix := fmt.Sprintf("%04d_create_%s", opts.MigrationSequence, opts.TableName)
	migrations := applyImportProfileToFiles([]GeneratedFile{
		{Path: "migrations/" + migrationPrefix + ".up.sql", Content: migUp},
		{Path: "migrations/" + migrationPrefix + ".down.sql", Content: migDown},
	}, opts.ImportProfile)

	return EntityResult{
		EntityName:      opts.Name,
		ModuleName:      opts.ModuleName,
		Files:           files,
		Migrations:      migrations,
		RegisterSnippet: applyImportProfile(registerSnippet, opts.ImportProfile),
	}, nil
}

// GenerateFeature creates one validated FeatureBuilder-based vertical slice.
func GenerateFeature(opts FeatureOptions) (FeatureResult, error) {
	if err := validateFeatureOptions(opts); err != nil {
		return FeatureResult{}, err
	}
	files := generateFeatureFiles(opts.ModuleName, opts.Name, opts.UseCases)
	testFile := generateFeatureTestCode(opts.ModuleName, opts.Name)
	e2eFile := generateFeatureE2ECode(opts.ModuleName, opts.Name)
	files = append(
		files,
		GeneratedFile{Path: "feature_test.go", Content: testFile},
		GeneratedFile{Path: "e2e.go", Content: e2eFile},
	)
	return FeatureResult{
		FeatureName: opts.Name,
		ModuleName:  opts.ModuleName,
		Files:       applyImportProfileToFiles(normalizeGeneratedGoFiles(files), opts.ImportProfile),
	}, nil
}

// WriteFiles validates and materializes one complete generated file set.
func WriteFiles(opts WriteOptions) error {
	if opts.BaseDir == "" {
		return fmt.Errorf("base directory is required")
	}
	if len(opts.Files) == 0 {
		return fmt.Errorf("at least one generated file is required")
	}
	basePath, err := filepath.Abs(opts.BaseDir)
	if err != nil {
		return fmt.Errorf("resolve base directory %q: %w", opts.BaseDir, err)
	}
	if err := validateGeneratedBase(basePath); err != nil {
		return err
	}
	fullPaths := make([]string, len(opts.Files))
	seen := make(map[string]struct{}, len(opts.Files))
	generatedParents := make(map[string]struct{})
	for _, f := range opts.Files {
		if !filepath.IsLocal(f.Path) || filepath.Clean(f.Path) != f.Path || f.Path == "." {
			return fmt.Errorf("generated file path %q must be a canonical relative path", f.Path)
		}
		if _, duplicate := seen[f.Path]; duplicate {
			return fmt.Errorf("duplicate generated file path %q", f.Path)
		}
		seen[f.Path] = struct{}{}
		for parent := filepath.Dir(f.Path); parent != "."; parent = filepath.Dir(parent) {
			generatedParents[parent] = struct{}{}
		}
	}
	for path := range seen {
		if _, conflict := generatedParents[path]; conflict {
			return fmt.Errorf("generated path %q is both a file and a parent directory", path)
		}
	}
	for i, f := range opts.Files {
		fullPaths[i] = filepath.Join(basePath, f.Path)
		if err := validateGeneratedParent(basePath, f.Path); err != nil {
			return err
		}
		if _, statErr := os.Lstat(fullPaths[i]); statErr == nil {
			return fmt.Errorf("refusing to overwrite existing path %s", fullPaths[i])
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("inspect output path %s: %w", fullPaths[i], statErr)
		}
	}
	if opts.DryRun {
		for i, f := range opts.Files {
			if opts.Output != nil {
				if _, err := fmt.Fprintf(opts.Output, "  [dry-run] %s (%d bytes)\n", fullPaths[i], len(f.Content)); err != nil {
					return fmt.Errorf("write dry-run output for %s: %w", fullPaths[i], err)
				}
			}
		}
		return nil
	}

	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return fmt.Errorf("create base directory %s: %w", basePath, err)
	}
	if err := validateGeneratedBase(basePath); err != nil {
		return err
	}
	root, err := os.OpenRoot(basePath)
	if err != nil {
		return fmt.Errorf("open generated output root %s: %w", basePath, err)
	}
	defer root.Close()

	created := make([]string, 0, len(opts.Files))
	for i, f := range opts.Files {
		fullPath := fullPaths[i]
		relativeDir := filepath.Dir(f.Path)
		if relativeDir != "." {
			if err := root.MkdirAll(relativeDir, 0o755); err != nil {
				return rollbackGeneratedFiles(root, fmt.Errorf("create directory %s: %w", filepath.Dir(fullPath), err), created)
			}
		}

		file, err := root.OpenFile(f.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return rollbackGeneratedFiles(root, fmt.Errorf("create file %s: %w", fullPath, err), created)
		}
		created = append(created, f.Path)
		if _, err := io.WriteString(file, f.Content); err != nil {
			_ = file.Close()
			return rollbackGeneratedFiles(root, fmt.Errorf("write file %s: %w", fullPath, err), created)
		}
		if err := file.Close(); err != nil {
			return rollbackGeneratedFiles(root, fmt.Errorf("close file %s: %w", fullPath, err), created)
		}
		if opts.Output != nil {
			if _, err := fmt.Fprintf(opts.Output, "  created %s\n", fullPath); err != nil {
				return rollbackGeneratedFiles(root, fmt.Errorf("write output for %s: %w", fullPath, err), created)
			}
		}
	}
	return nil
}

func validateGeneratedBase(basePath string) error {
	for current := basePath; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("generated output base component %s must not be a symbolic link", current)
			}
			if !info.IsDir() {
				return fmt.Errorf("generated output base component %s is not a directory", current)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect generated output base component %s: %w", current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
	}
}

func validateGeneratedParent(basePath, relativePath string) error {
	baseInfo, err := os.Lstat(basePath)
	if err == nil {
		if baseInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("base directory %s must not be a symbolic link", basePath)
		}
		if !baseInfo.IsDir() {
			return fmt.Errorf("base path %s is not a directory", basePath)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect base directory %s: %w", basePath, err)
	}

	current := basePath
	parent := filepath.Dir(relativePath)
	if parent == "." {
		return nil
	}
	for component := range strings.SplitSeq(parent, string(os.PathSeparator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect generated output parent %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("generated output parent %s must not be a symbolic link", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("generated output parent %s is not a directory", current)
		}
	}
	return nil
}

func rollbackGeneratedFiles(root *os.Root, cause error, paths []string) error {
	errs := []error{cause}
	for i := len(paths) - 1; i >= 0; i-- {
		if err := root.Remove(paths[i]); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("roll back generated file %s: %w", paths[i], err))
		}
	}
	return errors.Join(errs...)
}
