// Implements: REQ-002, REQ-016.
// Per: ADR-0061.
// Discipline: C-14.

package scaffold

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	snakeIdentifierPattern    = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)
	exportedIdentifierPattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	fieldIdentifierPattern    = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	categoryPattern           = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	tagPattern                = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
)

var reservedEntityFields = map[string]struct{}{
	"id":        {},
	"tenantId":  {},
	"createdAt": {},
	"updatedAt": {},
	"deletedAt": {},
	"version":   {},
	"createdBy": {},
	"updatedBy": {},
	"deletedBy": {},
}

func validateModuleOptions(opts ModuleOptions) error {
	if err := validateSnakeIdentifier("module name", opts.Name); err != nil {
		return err
	}
	if err := validateText("module description", opts.Description, true); err != nil {
		return err
	}
	if !categoryPattern.MatchString(opts.Category) {
		return fmt.Errorf("module category %q must be lowercase kebab-case", opts.Category)
	}
	switch opts.Archetype {
	case "service", "registry", "specialized", "infrastructure":
	default:
		return fmt.Errorf("module archetype %q must be service, registry, specialized, or infrastructure", opts.Archetype)
	}
	if err := validateUniqueStrings("feature", opts.Features, func(value string) error {
		return validateSnakeIdentifier("feature name", value)
	}); err != nil {
		return err
	}
	if err := validateUniqueStrings("tag", opts.Tags, func(value string) error {
		if !tagPattern.MatchString(value) {
			return fmt.Errorf("tag %q must contain only lowercase letters, digits, dots, underscores, or hyphens", value)
		}
		return nil
	}); err != nil {
		return err
	}
	return validateUniqueStrings("port", opts.Ports, func(value string) error {
		return validateExportedIdentifier("port", value)
	})
}

func validateEntityOptions(opts EntityOptions) error {
	if err := validateSnakeIdentifier("module name", opts.ModuleName); err != nil {
		return err
	}
	if err := validateExportedIdentifier("entity name", opts.Name); err != nil {
		return err
	}
	if err := validateSnakeIdentifier("table name", opts.TableName); err != nil {
		return err
	}
	if opts.MigrationSequence == 0 {
		return fmt.Errorf("migration sequence must be greater than zero")
	}
	if len(opts.Fields) == 0 {
		return fmt.Errorf("entity fields are required")
	}
	return validateFields(opts.Fields)
}

func validateFeatureOptions(opts FeatureOptions) error {
	if err := validateSnakeIdentifier("module name", opts.ModuleName); err != nil {
		return err
	}
	if err := validateSnakeIdentifier("feature name", opts.Name); err != nil {
		return err
	}
	return validateUniqueStrings("use case", opts.UseCases, func(value string) error {
		return validateExportedIdentifier("use case", value)
	})
}

func validateE2EFlowOptions(opts E2EFlowOptions) error {
	if err := validateSnakeIdentifier("module name", opts.ModuleName); err != nil {
		return err
	}
	if err := validateSnakeIdentifier("feature name", opts.Feature); err != nil {
		return err
	}
	if err := validateExportedIdentifier("entity name", opts.EntityName); err != nil {
		return err
	}
	if len(opts.Fields) == 0 {
		return fmt.Errorf("E2E flow fields are required")
	}
	entityOpts := EntityOptions{
		ModuleName:        opts.ModuleName,
		Name:              opts.EntityName,
		TableName:         opts.TableName,
		Fields:            opts.Fields,
		MigrationSequence: 1,
	}
	return validateEntityOptions(entityOpts)
}

func validateSnakeIdentifier(kind, value string) error {
	if !snakeIdentifierPattern.MatchString(value) {
		return fmt.Errorf("%s %q must be canonical snake_case", kind, value)
	}
	return nil
}

func validateExportedIdentifier(kind, value string) error {
	if !exportedIdentifierPattern.MatchString(value) {
		return fmt.Errorf("%s %q must be an exported Go identifier", kind, value)
	}
	return nil
}

func validateFields(fields []Field) error {
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if err := validateField(field); err != nil {
			return err
		}
		if _, duplicate := seen[field.Name]; duplicate {
			return fmt.Errorf("duplicate field name %q", field.Name)
		}
		seen[field.Name] = struct{}{}
	}
	return nil
}

func validateField(field Field) error {
	if !fieldIdentifierPattern.MatchString(field.Name) {
		return fmt.Errorf("field name %q must be lowerCamelCase", field.Name)
	}
	if _, reserved := reservedEntityFields[field.Name]; reserved {
		return fmt.Errorf("field name %q collides with BaseEntity", field.Name)
	}
	if _, err := ResolveType(field.Type); err != nil {
		return fmt.Errorf("field %q: %w", field.Name, err)
	}
	if err := validateText("field description", field.Description, false); err != nil {
		return fmt.Errorf("field %q: %w", field.Name, err)
	}
	return nil
}

func validateText(kind, value string, required bool) error {
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("%s must not contain surrounding whitespace", kind)
	}
	if required && value == "" {
		return fmt.Errorf("%s is required", kind)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s must not contain control characters", kind)
		}
	}
	return nil
}

func validateUniqueStrings(kind string, values []string, validate func(string) error) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if err := validate(value); err != nil {
			return err
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("duplicate %s %q", kind, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}
