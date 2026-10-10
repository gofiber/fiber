package openapi

import (
	"maps"
	"reflect"
	"strconv"

	"github.com/gofiber/utils/v2"
)

// componentsSchemasRef prefixes every reference the registry hands out.
const componentsSchemasRef = "#/components/schemas/"

// schemaRegistry collects named struct types so each is emitted once under
// components.schemas and referenced elsewhere.
type schemaRegistry struct {
	schemas map[string]map[string]any
	names   map[reflect.Type]string
	// taken maps a component name to the type that owns it. A name a user's
	// Components already uses is taken by a nil type, so it is never reused.
	taken map[string]reflect.Type
}

// newSchemaRegistry starts a registry that never reuses names in Components.schemas.
func newSchemaRegistry(cfg *Config) *schemaRegistry {
	reg := &schemaRegistry{
		schemas: make(map[string]map[string]any),
		names:   make(map[reflect.Type]string),
		taken:   make(map[string]reflect.Type),
	}
	for name := range stringKeyedEntries(cfg.Components["schemas"]) {
		reg.taken[name] = nil
	}
	return reg
}

// resolve turns a schema argument into a schema map; a nil registry reflects inline.
func (reg *schemaRegistry) resolve(schema any) map[string]any {
	switch value := schema.(type) {
	case nil:
		return nil
	case map[string]any:
		if len(value) == 0 {
			return nil
		}
		return maps.Clone(value)
	default:
		return typeSchema(reflect.TypeOf(schema), nil, reg)
	}
}

// ref registers a named struct type and returns a reference to it. The name is
// claimed before the schema is built so self-referencing types do not recurse.
func (reg *schemaRegistry) ref(t reflect.Type, visited map[reflect.Type]bool) map[string]any {
	name, ok := reg.names[t]
	if !ok {
		name = reg.claimName(t)
		reg.names[t] = name
		reg.schemas[name] = nil
		reg.schemas[name] = structSchema(t, visited, reg)
	}
	return map[string]any{schemaKeyRef: componentsSchemasRef + name}
}

// claimName picks a unique component name: the type name, then package-qualified, then numbered.
func (reg *schemaRegistry) claimName(t reflect.Type) string {
	base := componentName(t.Name())
	candidates := []string{base}
	if pkg := t.PkgPath(); pkg != "" {
		if _, after, found := utils.LastCutByte(pkg, '/'); found {
			pkg = after
		}
		candidates = append(candidates, componentName(pkg+"."+t.Name()))
	}
	for _, candidate := range candidates {
		if _, exists := reg.taken[candidate]; !exists {
			reg.taken[candidate] = t
			return candidate
		}
	}
	for i := 2; ; i++ {
		candidate := base + "_" + strconv.Itoa(i)
		if _, exists := reg.taken[candidate]; !exists {
			reg.taken[candidate] = t
			return candidate
		}
	}
}

// componentName restricts a name to ^[a-zA-Z0-9.\-_]+$, so generics like Page[pkg.User] stay valid keys.
func componentName(name string) string {
	if key := keyName(name); key != "" {
		return key
	}
	return "_"
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
