package openapi

import (
	"maps"
	"reflect"
	"strconv"
	"strings"
)

// componentsSchemasRef prefixes every reference the registry hands out.
const componentsSchemasRef = "#/components/schemas/"

// schemaRegistry collects the named struct types met while generating one
// document, so each is emitted once under components.schemas and referenced
// everywhere it appears, the way a hand-written document would.
type schemaRegistry struct {
	schemas map[string]map[string]any
	names   map[reflect.Type]string
	// taken maps a component name to the type that owns it. A name a user's
	// Components already uses is taken by a nil type, so it is never reused.
	taken map[string]reflect.Type
}

// newSchemaRegistry starts a registry that leaves the names in the configured
// Components.schemas to the user.
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

// resolve turns a schema argument into a schema map: a map is cloned, a Go
// value is reflected with named types registered, and nil stays nil. A nil
// registry reflects inline, as SchemaOf does.
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

// ref registers a named struct type on first sight and returns a reference to
// it. The name is claimed before the schema is built, so a type that refers
// back to itself resolves to the same reference instead of recursing.
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

// claimName picks the component name for t: its type name, qualified by its
// package when another type already holds that name, and numbered past that.
func (reg *schemaRegistry) claimName(t reflect.Type) string {
	base := componentName(t.Name())
	candidates := []string{base}
	if pkg := t.PkgPath(); pkg != "" {
		if i := strings.LastIndexByte(pkg, '/'); i >= 0 {
			pkg = pkg[i+1:]
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

// componentName restricts a name to the characters a component key may use,
// ^[a-zA-Z0-9.\-_]+$, so a generic instantiation such as Page[pkg.User] still
// yields a valid key.
func componentName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for i := range len(name) {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '.', c == '-', c == '_':
			_ = b.WriteByte(c) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		default:
			_ = b.WriteByte('_') //nolint:errcheck // strings.Builder.WriteByte never returns an error
		}
	}
	if b.Len() == 0 {
		return "_"
	}
	return b.String()
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
