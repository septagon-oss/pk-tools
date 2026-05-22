// Package scaffold provides standalone code generation for platformkit modules,
// entities, and features. It extracts the scaffold logic from the MCP tools
// into a reusable library suitable for CLI usage and programmatic access.
package scaffold

import (
	"fmt"
	"strings"

	"github.com/septagon-oss/pk-core/pkg/registry"
)

// ToPascalCase converts a snake_case string to PascalCase.
func ToPascalCase(s string) string {
	parts := strings.Split(s, "_")
	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			result.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return result.String()
}

// ToSnakeCase converts a PascalCase or camelCase string to snake_case.
func ToSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// TypeInfo describes a scaffold field type with all its representations.
// Register custom types via RegisterType to extend the scaffold system.
type TypeInfo struct {
	GoType    string // Go type string, e.g. "string", "int64", "time.Time"
	GORMType  string // GORM column type, e.g. "varchar(255)", "bigint"
	SQLType   string // PostgreSQL type, e.g. "VARCHAR(255)", "INTEGER"
	IsNumeric bool   // Whether this type uses numeric query operators
	TestValue string // Go literal for test data, e.g. `"test-value"`, "42"
}

// Defaults used when a type is not found in the registry.
var defaultTypeInfo = TypeInfo{
	GoType: "string", GORMType: "varchar(255)", SQLType: "TEXT",
	IsNumeric: false, TestValue: `"test"`,
}

var typeRegistry = registry.New(registry.WithDefault[string, TypeInfo](defaultTypeInfo))

// RegisterType registers a field type with all its representations.
// Multiple aliases can map to the same TypeInfo by calling this for each alias.
// Safe to call from init(). Later calls for the same name override.
func RegisterType(name string, info TypeInfo) {
	if name == "" {
		return
	}
	typeRegistry.Register(name, info)
}

// GetTypeInfo returns the TypeInfo for a field type name, or nil if unknown.
func GetTypeInfo(name string) *TypeInfo {
	v, ok := typeRegistry.Get(name)
	if !ok {
		return nil
	}
	return &v
}

func resolveType(t string) TypeInfo {
	return typeRegistry.MustGet(t)
}

// GoTypeFromString converts a user-friendly type name to a Go type string.
func GoTypeFromString(t string) string { return resolveType(t).GoType }

// GORMTypeFromString converts a user-friendly type name to a GORM column type.
func GORMTypeFromString(t string) string { return resolveType(t).GORMType }

// SQLTypeFromString converts a user-friendly type name to a PostgreSQL type.
func SQLTypeFromString(t string) string { return resolveType(t).SQLType }

// IsNumericType returns true for types that should use numeric query operators.
func IsNumericType(t string) bool { return resolveType(t).IsNumeric }

// TestValueForType returns a Go literal suitable for test data.
func TestValueForType(t string) string { return resolveType(t).TestValue }

func init() {
	// Seed default types. Modules can override or add new types via RegisterType.
	stringType := TypeInfo{GoType: "string", GORMType: "varchar(255)", SQLType: "VARCHAR(255)", IsNumeric: false, TestValue: `"test-value"`}
	intType := TypeInfo{GoType: "int64", GORMType: "bigint", SQLType: "INTEGER", IsNumeric: true, TestValue: "42"}
	floatType := TypeInfo{GoType: "float64", GORMType: "decimal(10,2)", SQLType: "NUMERIC(10,2)", IsNumeric: true, TestValue: "3.14"}
	boolType := TypeInfo{GoType: "bool", GORMType: "boolean", SQLType: "BOOLEAN DEFAULT false", IsNumeric: false, TestValue: "true"}
	datetimeType := TypeInfo{GoType: "time.Time", GORMType: "timestamptz", SQLType: "TIMESTAMPTZ", IsNumeric: true, TestValue: "time.Now()"}
	uuidType := TypeInfo{GoType: "uuid.UUID", GORMType: "uuid", SQLType: "UUID", IsNumeric: false, TestValue: "uuid.New()"}
	textType := TypeInfo{GoType: "string", GORMType: "text", SQLType: "TEXT", IsNumeric: false, TestValue: `"test-value"`}
	decimalType := TypeInfo{GoType: "float64", GORMType: "decimal(10,2)", SQLType: "DECIMAL(19,4)", IsNumeric: true, TestValue: "3.14"}
	jsonType := TypeInfo{GoType: "string", GORMType: "varchar(255)", SQLType: "JSONB DEFAULT '{}'", IsNumeric: false, TestValue: `"test"`}

	for name, info := range map[string]TypeInfo{
		"string":    stringType,
		"integer":   intType,
		"int":       intType,
		"number":    floatType,
		"float":     floatType,
		"float64":   floatType,
		"decimal":   decimalType,
		"boolean":   boolType,
		"bool":      boolType,
		"datetime":  datetimeType,
		"timestamp": datetimeType,
		"uuid":      uuidType,
		"text":      textType,
		"json":      jsonType,
		"jsonb":     jsonType,
	} {
		RegisterType(name, info)
	}
}

// ParseFieldSpec parses a "name:type" string into a Field.
// If the type part is missing, defaults to "string".
func ParseFieldSpec(spec string) (Field, error) {
	parts := strings.SplitN(spec, ":", 2)
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return Field{}, fmt.Errorf("empty field name in spec %q", spec)
	}

	f := Field{
		Name:     name,
		Type:     "string",
		Required: false,
	}
	if len(parts) == 2 {
		typePart := strings.TrimSpace(parts[1])
		if typePart != "" {
			f.Type = typePart
		}
	}
	return f, nil
}

// ParseFieldSpecs parses a comma-separated list of "name:type" specs into Fields.
func ParseFieldSpecs(specs string) ([]Field, error) {
	if specs == "" {
		return nil, nil
	}
	parts := strings.Split(specs, ",")
	fields := make([]Field, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		f, err := ParseFieldSpec(part)
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	return fields, nil
}
