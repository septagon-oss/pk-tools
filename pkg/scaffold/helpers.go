// Implements: REQ-002.
// Per: ADR-0061.
// Discipline: C-14.

package scaffold

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/septagon-oss/pk-core/pkg/registry"
)

func normalizeGeneratedGoFiles(files []GeneratedFile) []GeneratedFile {
	for i := range files {
		if strings.HasSuffix(files[i].Path, ".go") {
			files[i].Content = NormalizeGeneratedGoSource(files[i].Path, files[i].Content)
		}
	}
	return files
}

// NormalizeGeneratedGoSource enforces the exact C-14 purpose header on Go
// source emitted by extension templates that compose with this scaffolder.
func NormalizeGeneratedGoSource(path, content string) string {
	lines := strings.Split(content, "\n")
	purpose := ""
	for i := 0; i+2 < len(lines); i++ {
		if (!strings.HasPrefix(lines[i], "// Implements: ") && !strings.HasPrefix(lines[i], "// Validates: ")) ||
			!strings.HasPrefix(lines[i+1], "// Per: ADR-") ||
			lines[i+2] != "// Discipline: C-14." {
			continue
		}
		purpose = strings.Join(lines[i:i+3], "\n")
		end := i + 3
		if end < len(lines) && lines[end] == "" {
			end++
		}
		lines = append(lines[:i], lines[end:]...)
		break
	}

	verb := "Implements"
	if strings.HasSuffix(path, "_test.go") {
		verb = "Validates"
	}
	if purpose == "" {
		purpose = "// " + verb + ": REQ-002.\n// Per: ADR-0029.\n// Discipline: C-14."
	} else {
		firstBreak := strings.IndexByte(purpose, '\n')
		separator := strings.Index(purpose[:firstBreak], ": ")
		purpose = "// " + verb + purpose[separator:firstBreak] + purpose[firstBreak:]
	}

	return purpose + "\n\n" + strings.TrimLeft(strings.Join(lines, "\n"), "\n")
}

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
	runes := []rune(s)
	var result strings.Builder
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) &&
			(unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]) ||
				(i+1 < len(runes) && unicode.IsUpper(runes[i-1]) && unicode.IsLower(runes[i+1]))) {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}

// TypeInfo describes a scaffold field type with all its representations.
// Register custom types via RegisterType to extend the scaffold system.
type TypeInfo struct {
	GoType    string // Go type string, e.g. "string", "int64", "time.Time"
	GORMType  string // GORM column type, e.g. "varchar(255)", "bigint"
	SQLType   string // PostgreSQL type, e.g. "VARCHAR(255)", "INTEGER"
	IsNumeric bool   // Whether this type uses numeric query operators
}

var typeRegistry = registry.New[string, TypeInfo]()

// RegisterType registers a field type with all its representations.
// Multiple aliases can map to the same TypeInfo by calling this for each alias.
// Duplicate names are rejected so one type name always has one owner.
func RegisterType(name string, info TypeInfo) error {
	if name == "" {
		return fmt.Errorf("scaffold type name is required")
	}
	if name != strings.TrimSpace(name) || name != strings.ToLower(name) {
		return fmt.Errorf("scaffold type name %q must be lowercase without surrounding whitespace", name)
	}
	if info.GoType == "" || info.GORMType == "" || info.SQLType == "" {
		return fmt.Errorf("scaffold type %q requires Go, GORM, and SQL representations", name)
	}
	if err := typeRegistry.RegisterIfAbsent(name, info); err != nil {
		return fmt.Errorf("register scaffold type %q: %w", name, err)
	}
	return nil
}

// ResolveType returns the complete representation of a registered field type.
// Unknown names fail closed rather than silently generating string columns.
func ResolveType(name string) (TypeInfo, error) {
	v, ok := typeRegistry.Get(name)
	if !ok {
		return TypeInfo{}, fmt.Errorf("unknown scaffold field type %q; registered types: %s", name, strings.Join(registry.SortedKeys(typeRegistry, func(a, b string) bool { return a < b }), ", "))
	}
	return v, nil
}

func init() {
	// Seed default types. Modules can override or add new types via RegisterType.
	stringType := TypeInfo{GoType: "string", GORMType: "varchar(255)", SQLType: "VARCHAR(255)"}
	intType := TypeInfo{GoType: "int64", GORMType: "bigint", SQLType: "BIGINT", IsNumeric: true}
	boolType := TypeInfo{GoType: "bool", GORMType: "boolean", SQLType: "BOOLEAN DEFAULT false"}
	datetimeType := TypeInfo{GoType: "time.Time", GORMType: "timestamptz", SQLType: "TIMESTAMPTZ", IsNumeric: true}
	uuidType := TypeInfo{GoType: "uuid.UUID", GORMType: "uuid", SQLType: "UUID"}
	textType := TypeInfo{GoType: "string", GORMType: "text", SQLType: "TEXT"}
	decimalType := TypeInfo{GoType: "float64", GORMType: "decimal(10,2)", SQLType: "DECIMAL(19,4)", IsNumeric: true}
	jsonType := TypeInfo{GoType: "json.RawMessage", GORMType: "jsonb", SQLType: "JSONB DEFAULT '{}'::jsonb"}

	for name, info := range map[string]TypeInfo{
		"string":   stringType,
		"integer":  intType,
		"decimal":  decimalType,
		"boolean":  boolType,
		"datetime": datetimeType,
		"uuid":     uuidType,
		"text":     textType,
		"jsonb":    jsonType,
	} {
		if err := RegisterType(name, info); err != nil {
			panic(err)
		}
	}
}

// ParseFieldSpec parses the canonical "name:type" field shape.
func ParseFieldSpec(spec string) (Field, error) {
	parts := strings.SplitN(spec, ":", 2)
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return Field{}, fmt.Errorf("empty field name in spec %q", spec)
	}
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return Field{}, fmt.Errorf("field %q must use the canonical name:type shape", name)
	}

	f := Field{
		Name: name,
		Type: strings.TrimSpace(parts[1]),
	}
	if _, err := ResolveType(f.Type); err != nil {
		return Field{}, fmt.Errorf("field %q: %w", f.Name, err)
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
