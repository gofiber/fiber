package openapi

import (
	"maps"
	"slices"
	"strings"
)

// dialect is what an OpenAPI version permits. Callers ask for a capability, not a version number, so
// what each release adds, and how a document is downgraded without it, is decided here and nowhere else.
type dialect struct {
	// minor is the release's minor number: 0 for 3.0.x, 1 for 3.1.x, 2 for 3.2.x.
	minor int
}

// The minor numbers of the supported releases, which a capability names as the release that added it.
const (
	release31 = 1
	release32 = 2
)

// since reports whether the dialect is the given release or a later one.
func (d dialect) since(release int) bool { return d.minor >= release }

// newDialect reads version, which Config validation has already limited to a supported release. An
// unsupported string ranks as 3.0, the most restrictive dialect.
func newDialect(version string) dialect {
	minor, ok := parseVersion(version)
	if !ok {
		minor = 0
	}
	return dialect{minor: minor}
}

// parseVersion reads the minor version of a 3.0.x, 3.1.x or 3.2.x release (0, 1 or 2).
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

// hasQueryMethod reports whether Path Items can carry the QUERY operation (3.2+).
func (d dialect) hasQueryMethod() bool { return d.since(release32) }

// hasQuerystringLocation reports whether a parameter can be located "in: querystring" (3.2+).
func (d dialect) hasQuerystringLocation() bool { return d.since(release32) }

// hasServerName reports whether a Server Object may carry a name (3.2+).
func (d dialect) hasServerName() bool { return d.since(release32) }

// hasSelf reports whether the document may state its own URI in $self (3.2+).
func (d dialect) hasSelf() bool { return d.since(release32) }

// hasJSONSchemaDialect reports whether the document aligns with JSON Schema 2020-12 (3.1+): it may carry
// info.summary, webhooks and jsonSchemaDialect, a License Object may name an identifier, a schema type may
// be "null", and a Reference Object may sit beside sibling keys.
func (d dialect) hasJSONSchemaDialect() bool { return d.since(release31) }

// license trims a License Object to what the dialect allows: identifier or url, never both, and
// identifier only from 3.1. Identifier is the more precise of the two.
func (d dialect) license(license License) License {
	if d.hasJSONSchemaDialect() {
		license.URL = ""
	} else {
		license.Identifier = ""
	}
	return license
}

// nullable returns a copy of a property schema that also accepts null, the way the dialect spells it:
// a nullable keyword in 3.0, and a "null" type in 3.1 and later, alone or in an anyOf beside a $ref or a
// composition. A schema with none of type, $ref, allOf, anyOf or oneOf already accepts anything. Under 3.0 a
// composition with no type stays as it is, as nullable has no effect without a type there.
func (d dialect) nullable(schema map[string]any) map[string]any {
	schema = maps.Clone(schema)
	if enum, ok := schema["enum"].([]any); ok {
		schema["enum"] = append(slices.Clone(enum), nil)
	}
	if !d.hasJSONSchemaDialect() {
		schema["nullable"] = true
		return schema
	}
	if typ, isType := schema[schemaKeyType].(string); isType {
		schema[schemaKeyType] = []any{typ, schemaTypeNull}
		return schema
	}
	if target, isRef := schema[schemaKeyRef].(string); isRef {
		delete(schema, schemaKeyRef)
		schema["anyOf"] = []any{map[string]any{schemaKeyRef: target}, map[string]any{schemaKeyType: schemaTypeNull}}
		return schema
	}
	for _, keyword := range [...]string{"allOf", "anyOf", "oneOf"} {
		if _, composed := schema[keyword]; composed {
			return map[string]any{"anyOf": []any{schema, map[string]any{schemaKeyType: schemaTypeNull}}}
		}
	}
	return schema
}
