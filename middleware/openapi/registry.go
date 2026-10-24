package openapi

import (
	"maps"
	"reflect"
	"strings"

	"github.com/gofiber/fiber/v3/internal/schemautil"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// componentsSchemasRef prefixes every reference the registry hands out.
const componentsSchemasRef = "#/components/schemas/"

// schemaRegistry collects named struct types so each is emitted once under
// components.schemas and referenced elsewhere.
// schemaKey identifies a registered component: the same Go type documents differently for JSON and
// form bodies, because each reads its own struct tags.
type schemaKey struct {
	t   reflect.Type
	tag string
}

const (
	tagJSON = "json"
	tagForm = "form"
)

type schemaRegistry struct {
	schemas map[string]map[string]any
	names   map[schemaKey]string
	// taken maps a component name to the type that owns it. A name a user's
	// Components already uses is taken by a nil type, so it is never reused.
	taken map[string]reflect.Type
	// tag is the struct tag the schemas under construction read: json, or form for form media types.
	tag string
	// dialect decides how the document's OpenAPI version spells nullable schemas and $ref siblings.
	dialect dialect
}

// newSchemaRegistry starts a registry that never reuses names in Components.schemas.
func newSchemaRegistry(cfg *Config, d dialect) *schemaRegistry {
	reg := &schemaRegistry{
		schemas: make(map[string]map[string]any),
		names:   make(map[schemaKey]string),
		taken:   make(map[string]reflect.Type),
		dialect: d,
	}
	for name := range stringKeyedEntries(cfg.Components["schemas"]) {
		reg.taken[name] = nil
	}
	return reg
}

// resolve turns a schema argument into a schema map; a nil registry reflects inline.
func (reg *schemaRegistry) resolve(schema any) map[string]any {
	if schema == nil {
		return nil
	}
	if value, ok := schemautil.AsMap(schema); ok {
		if len(value) == 0 {
			return nil
		}
		return maps.Clone(value)
	}
	return typeSchema(reflect.TypeOf(schema), nil, reg)
}

// ref registers a named struct type and returns a reference to it. The name is
// claimed before the schema is built so self-referencing types do not recurse. The component is
// built with an empty visited set: it must not depend on the types of whichever schema reached it
// first, or the same type would document differently depending on registration order.
func (reg *schemaRegistry) ref(t reflect.Type) map[string]any {
	key := schemaKey{t: t, tag: reg.tagKey()}
	name, ok := reg.names[key]
	if !ok {
		name = reg.claimName(t, key.tag)
		reg.names[key] = name
		reg.schemas[name] = nil // reserve the entry so a recursive reference finds the name taken
		reg.schemas[name] = structSchema(t, nil, reg)
	}
	return map[string]any{schemaKeyRef: componentsSchemasRef + name}
}

// claimName picks a unique component name: the type name, then package-qualified, then numbered.
func (reg *schemaRegistry) claimName(t reflect.Type, tag string) string {
	suffix := ""
	if tag != tagJSON {
		suffix = "_" + tag
	}
	base := componentName(t.Name()) + suffix
	candidates := []string{base}
	if pkg := t.PkgPath(); pkg != "" {
		if _, after, found := utils.LastCutByte(pkg, '/'); found {
			pkg = after
		}
		candidates = append(candidates, componentName(pkg+"."+t.Name())+suffix)
	}
	for _, candidate := range candidates {
		if _, exists := reg.taken[candidate]; !exists {
			reg.taken[candidate] = t
			return candidate
		}
	}
	for i := 2; ; i++ {
		candidate := base + "_" + utils.FormatInt(int64(i))
		if _, exists := reg.taken[candidate]; !exists {
			reg.taken[candidate] = t
			return candidate
		}
	}
}

// componentName restricts a name to ^[a-zA-Z0-9.\-_]+$. A generic type's package qualifiers are dropped, so
// Page[github.com/x/y.User] is named Page_User rather than carrying an import path into the document.
func componentName(name string) string {
	if open := strings.IndexByte(name, '['); open > 0 && strings.HasSuffix(name, "]") {
		args := splitTypeArguments(name[open+1 : len(name)-1])
		for i, arg := range args {
			args[i] = unqualified(arg)
		}
		name = name[:open] + "_" + strings.Join(args, "_")
	}
	if key := keyName(name); key != "" {
		return key
	}
	return "_"
}

// splitTypeArguments splits a type-argument list at its top-level commas.
func splitTypeArguments(list string) []string {
	var args []string
	depth, from := 0, 0
	for i := 0; i < len(list); i++ {
		switch list[i] {
		case '[':
			depth++
		case ']':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, list[from:i])
				from = i + 1
			}
		}
	}
	return append(args, list[from:])
}

// unqualified drops the package path in front of every type name in a type-argument list element:
// "[]github.com/x/y.User" becomes "[]User".
func unqualified(arg string) string {
	var b strings.Builder
	run := 0
	for i := 0; i <= len(arg); i++ {
		if i < len(arg) && (isASCIIAlphanumeric(arg[i]) || arg[i] == '_' || arg[i] == '.' || arg[i] == '/' || arg[i] == '-') {
			continue
		}
		token := arg[run:i]
		if cut := strings.LastIndexAny(token, "./"); cut >= 0 {
			token = token[cut+1:]
		}
		_, _ = b.WriteString(token) //nolint:errcheck // strings.Builder.WriteString never returns an error
		if i < len(arg) {
			_ = b.WriteByte(arg[i]) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		}
		run = i + 1
	}
	// Spell the shape out in words so it survives the character restriction on component names.
	return utils.Trim(strings.NewReplacer("[]", "Array_", "map[", "Map_", "[", "_", "]", "_", ",", "_", "*", "").Replace(b.String()), '_')
}

// componentSchemas returns the registered schemas keyed for components.schemas.
func (reg *schemaRegistry) componentSchemas() map[string]any {
	if reg == nil || len(reg.schemas) == 0 {
		return nil
	}
	out := make(map[string]any, len(reg.schemas))
	for name, schema := range reg.schemas {
		out[name] = schema
	}
	return out
}

// withRefSiblings returns schema ready to carry sibling keys next to a "$ref". OpenAPI 3.1 allows them; in
// 3.0 a Reference Object holds only "$ref", so the reference moves into "allOf" and the siblings stay beside it.
func (reg *schemaRegistry) withRefSiblings(schema map[string]any) map[string]any {
	target, isRef := schema[schemaKeyRef].(string)
	if reg == nil || reg.dialect.hasJSONSchemaDialect() || !isRef || len(schema) < 2 {
		return schema
	}
	wrapped := make(map[string]any, len(schema))
	for key, value := range schema {
		if key != schemaKeyRef {
			wrapped[key] = value
		}
	}
	wrapped["allOf"] = []any{map[string]any{schemaKeyRef: target}}
	return wrapped
}

// tagKey is the struct tag the schemas being built read.
func (reg *schemaRegistry) tagKey() string {
	if reg == nil || reg.tag == "" {
		return tagJSON
	}
	return reg.tag
}

// isFormMediaType reports whether a media type carries a form body, which binds by `form` tags.
func isFormMediaType(mediaType string) bool {
	base, _, _ := utils.CutByte(mediaType, ';')
	base = utils.TrimSpace(base)
	return utils.EqualFold(base, fiber.MIMEMultipartForm) || utils.EqualFold(base, fiber.MIMEApplicationForm)
}

// contentEntryFor builds the entry for one media type, reading the struct tags that media type binds by.
func contentEntryFor(mediaType string, mt fiber.RouteMediaType, reg *schemaRegistry) map[string]any {
	if reg == nil || !isFormMediaType(mediaType) {
		return contentEntry(mt, reg)
	}
	previous := reg.tag
	reg.tag = tagForm
	defer func() { reg.tag = previous }()
	return contentEntry(mt, reg)
}

// nullable lets a property schema accept null in the document's dialect. A nil registry (SchemaOf)
// leaves the schema alone.
func (reg *schemaRegistry) nullable(schema map[string]any) map[string]any {
	if reg == nil {
		return schema
	}
	return reg.dialect.nullable(schema)
}
