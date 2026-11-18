package openapi

import (
	"maps"
	"reflect"
	"strings"

	"github.com/gofiber/fiber/v3/internal/schemautil"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

const componentsSchemasRef = "#/components/schemas/"

// schemaRegistry collects named struct types so each is emitted once under components.schemas.
// schemaKey identifies a component: JSON and form bodies read different struct tags.
type schemaKey struct {
	t   reflect.Type
	tag string
}

const (
	tagJSON = "json"
	tagForm = "form"
	tagXML  = "xml"
)

type schemaRegistry struct {
	schemas map[string]map[string]any
	names   map[schemaKey]string
	// taken maps a component name to the type that owns it; a user-defined name is taken by nil.
	taken map[string]reflect.Type
	// typeSchemas and naming come from Config.TypeSchemas and Config.SchemaName.
	typeSchemas map[reflect.Type]any
	naming      func(reflect.Type) string
	tag         string
	dialect     dialect
}

// configured reports whether Config.TypeSchemas gives t its schema, which is then written inline.
func (reg *schemaRegistry) configured(t reflect.Type) bool {
	_, ok := reg.typeSchemas[t]
	return ok
}

// newSchemaRegistry starts a registry that never reuses names in Components.schemas.
func newSchemaRegistry(cfg *Config, d dialect) *schemaRegistry {
	reg := &schemaRegistry{
		schemas: make(map[string]map[string]any),
		names:   make(map[schemaKey]string),
		taken:   make(map[string]reflect.Type),
		dialect: d,

		typeSchemas: cfg.TypeSchemas,
		naming:      cfg.SchemaName,
	}
	for name := range stringKeyedEntries(cfg.Components["schemas"]) {
		reg.taken[name] = nil
	}
	return reg
}

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

// itemSchemas reports whether the document may carry itemSchema in a media type.
func (reg *schemaRegistry) itemSchemas() bool {
	return reg != nil && reg.dialect.hasItemSchema()
}

// ref registers a named struct type and returns a reference to it. The name is claimed first so
// self-referencing types do not recurse; the component is built independent of registration order.
func (reg *schemaRegistry) ref(t reflect.Type) map[string]any {
	key := schemaKey{t: t, tag: reg.tagKey()}
	name, ok := reg.names[key]
	if !ok {
		name = reg.claimName(t, key.tag)
		reg.names[key] = name
		reg.schemas[name] = nil // reserve the entry so a recursive reference finds the name taken
		if hasCustomSchema(t, reg) {
			reg.schemas[name] = customSchema(t, nil, reg)
		} else {
			reg.schemas[name] = structSchema(t, nil, reg)
		}
	}
	return map[string]any{schemaKeyRef: componentsSchemasRef + name}
}

func (reg *schemaRegistry) claimName(t reflect.Type, tag string) string {
	suffix := ""
	if tag != tagJSON {
		suffix = "_" + tag
	}
	typeName := t.Name()
	if reg.naming != nil {
		if custom := reg.naming(t); custom != "" {
			typeName = custom
		}
	}
	base := componentName(typeName) + suffix
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

// componentName restricts a name to ^[a-zA-Z0-9.\-_]+$; generic package qualifiers are dropped.
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

// unqualified drops the package path before every type name in a type-argument list element.
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
	return utils.Trim(strings.NewReplacer("[]", "Array_", "map[", "Map_", "[", "_", "]", "_", ",", "_", "*", "").Replace(b.String()), '_')
}

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

// withRefSiblings returns schema ready to carry keys next to a "$ref"; 3.0 moves the reference into "allOf".
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
	wrapped[schemaKeyAllOf] = []any{map[string]any{schemaKeyRef: target}}
	return wrapped
}

func (reg *schemaRegistry) tagKey() string {
	if reg == nil || reg.tag == "" {
		return tagJSON
	}
	return reg.tag
}

func isFormMediaType(mediaType string) bool {
	base, _, _ := utils.CutByte(mediaType, ';')
	base = utils.TrimSpace(base)
	return utils.EqualFold(base, fiber.MIMEMultipartForm) || utils.EqualFold(base, fiber.MIMEApplicationForm)
}

// isXMLMediaType reports whether a media type carries XML, which is described by `xml` tags.
func isXMLMediaType(mediaType string) bool {
	base, _, _ := utils.CutByte(mediaType, ';')
	base = utils.TrimSpace(base)
	return utils.EqualFold(base, fiber.MIMEApplicationXML) || utils.EqualFold(base, fiber.MIMETextXML) ||
		(len(base) > len("+xml") && utils.EqualFold(base[len(base)-len("+xml"):], "+xml"))
}

func contentEntryFor(mediaType string, mt *fiber.RouteMediaType, reg *schemaRegistry) map[string]any {
	if reg == nil {
		return contentEntry(mt, reg)
	}
	var tag string
	switch {
	case isFormMediaType(mediaType):
		tag = tagForm
	case isXMLMediaType(mediaType):
		tag = tagXML
	default:
		return contentEntry(mt, reg)
	}
	previous := reg.tag
	reg.tag = tag
	defer func() { reg.tag = previous }()
	return contentEntry(mt, reg)
}

// nullable lets a property schema accept null in the document's dialect.
func (reg *schemaRegistry) nullable(schema map[string]any) map[string]any {
	if reg == nil {
		return schema
	}
	return reg.dialect.nullable(schema)
}
