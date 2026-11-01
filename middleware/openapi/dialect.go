package openapi

import (
	"maps"
	"reflect"
	"slices"
	"strings"
)

// dialect is what an OpenAPI version permits; callers ask for a capability, not a version number.
type dialect struct {
	minor int
}

const (
	release31 = 1
	release32 = 2
)

func (d dialect) since(release int) bool { return d.minor >= release }

// newDialect reads a version already validated; an unsupported one ranks as 3.0.
func newDialect(version string) dialect {
	minor, _ := parseVersion(version)
	return dialect{minor: minor}
}

func parseVersion(version string) (minor int, ok bool) { //nolint:nonamedreturns // an int and a bool read better named
	major, rest, found := strings.Cut(version, ".")
	if !found || major != "3" {
		return 0, false
	}
	minorText, patch, found := strings.Cut(rest, ".")
	if !found || len(minorText) != 1 || minorText[0] > '2' || !isASCIIDigit(minorText[0]) || patch == "" {
		return 0, false
	}
	for i := 0; i < len(patch); i++ {
		if !isASCIIDigit(patch[i]) {
			return 0, false
		}
	}
	return int(minorText[0] - '0'), true
}

func (d dialect) hasQueryMethod() bool { return d.since(release32) }

func (d dialect) hasQuerystringLocation() bool { return d.since(release32) }

func (d dialect) hasServerName() bool { return d.since(release32) }

func (d dialect) hasSelf() bool { return d.since(release32) }

// hasJSONSchemaDialect reports whether the document aligns with JSON Schema 2020-12 (3.1+).
func (d dialect) hasJSONSchemaDialect() bool { return d.since(release31) }

// license trims a License Object to what the dialect allows: identifier or url, never both.
func (d dialect) license(license License) License {
	if d.hasJSONSchemaDialect() {
		license.URL = ""
	} else {
		license.Identifier = ""
	}
	return license
}

// nullable returns a copy of a property schema that also accepts null, spelled the dialect's way.
// A schema with no type (a $ref or composition) takes an anyOf with a null-only branch.
func (d dialect) nullable(schema map[string]any) map[string]any {
	schema = maps.Clone(schema)
	if enum, ok := enumValues(schema["enum"]); ok && !slices.Contains(enum, nil) {
		schema["enum"] = append(enum, nil)
	}
	if d.hasJSONSchemaDialect() {
		switch typ := schema[schemaKeyType].(type) {
		case string:
			schema[schemaKeyType] = []any{typ, schemaTypeNull}
			return schema
		case []string:
			types := make([]any, 0, len(typ)+1)
			for _, name := range typ {
				types = append(types, name)
			}
			schema[schemaKeyType] = appendNullType(types)
			return schema
		case []any:
			schema[schemaKeyType] = appendNullType(slices.Clone(typ))
			return schema
		}
	} else if _, typed := schema[schemaKeyType]; typed {
		schema["nullable"] = true
		return schema
	}

	null := map[string]any{schemaKeyType: schemaTypeNull}
	if !d.hasJSONSchemaDialect() {
		null = map[string]any{schemaKeyType: schemaTypeObject, "nullable": true, "enum": []any{nil}}
	}
	if target, isRef := schema[schemaKeyRef].(string); isRef {
		delete(schema, schemaKeyRef)
		schema["anyOf"] = []any{map[string]any{schemaKeyRef: target}, null}
		return schema
	}
	for _, keyword := range [...]string{"allOf", "anyOf", "oneOf"} {
		if _, composed := schema[keyword]; composed {
			return map[string]any{"anyOf": []any{schema, null}}
		}
	}
	return schema
}

func appendNullType(types []any) []any {
	if slices.Contains(types, any(schemaTypeNull)) {
		return types
	}
	return append(types, schemaTypeNull)
}

func enumValues(enum any) ([]any, bool) {
	if values, ok := enum.([]any); ok {
		return slices.Clone(values), true
	}
	list := reflect.ValueOf(enum)
	if !list.IsValid() || list.Kind() != reflect.Slice {
		return nil, false
	}
	values := make([]any, list.Len())
	for i := range values {
		values[i] = list.Index(i).Interface()
	}
	return values, true
}
