// Implements: REQ-002.
// Per: ADR-0061.
// Discipline: C-14.

package scaffold

import (
	"fmt"
	"strings"
	"unicode"
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

// TypeInfo describes one canonical scaffold field type across generated layers.
type TypeInfo struct {
	GoType    string
	GORMType  string
	SQLType   string
	E2EValue  string
	IsNumeric bool
}

const canonicalTypeNames = "boolean, datetime, decimal, integer, jsonb, string, text, uuid"

// ResolveType returns the complete representation of a canonical field type.
// The vocabulary is intentionally closed: adding a type requires updating its
// Go, GORM, SQL, import, query-operator, and E2E representations together.
func ResolveType(name string) (TypeInfo, error) {
	switch name {
	case "string":
		return TypeInfo{GoType: "string", GORMType: "varchar(255)", SQLType: "VARCHAR(255)", E2EValue: "Test Value"}, nil
	case "integer":
		return TypeInfo{GoType: "int64", GORMType: "bigint", SQLType: "BIGINT", E2EValue: "42", IsNumeric: true}, nil
	case "decimal":
		return TypeInfo{GoType: "float64", GORMType: "decimal(10,2)", SQLType: "DECIMAL(19,4)", E2EValue: "99.99", IsNumeric: true}, nil
	case "boolean":
		return TypeInfo{GoType: "bool", GORMType: "boolean", SQLType: "BOOLEAN DEFAULT false", E2EValue: "true"}, nil
	case "datetime":
		return TypeInfo{GoType: "time.Time", GORMType: "timestamptz", SQLType: "TIMESTAMPTZ", E2EValue: "2026-01-15T10:00:00Z", IsNumeric: true}, nil
	case "uuid":
		return TypeInfo{GoType: "uuid.UUID", GORMType: "uuid", SQLType: "UUID", E2EValue: "00000000-0000-0000-0000-000000000001"}, nil
	case "text":
		return TypeInfo{GoType: "string", GORMType: "text", SQLType: "TEXT", E2EValue: "Test description text"}, nil
	case "jsonb":
		return TypeInfo{GoType: "json.RawMessage", GORMType: "jsonb", SQLType: "JSONB DEFAULT '{}'::jsonb", E2EValue: `{"test":true}`}, nil
	default:
		return TypeInfo{}, fmt.Errorf("unknown scaffold field type %q; canonical types: %s", name, canonicalTypeNames)
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
	if err := validateField(f); err != nil {
		return Field{}, err
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
	if err := validateFields(fields); err != nil {
		return nil, err
	}
	return fields, nil
}
