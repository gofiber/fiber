package openapi

import (
	"maps"
	"reflect"
	"slices"

	"github.com/gofiber/fiber/v3/internal/schemautil"
)

// SchemaProvider is implemented by a type that describes its own schema, for what reflection cannot know: a
// decimal written as a string, an ID with a pattern, an enum of constants, or a value that is one of several
// types (see [OneOf]). The method may have a value or a pointer receiver and is called on a zero value, so it
// must not depend on one.
//
// The schema is a generic map. A Go value inside it (a struct, a pointer to one, a slice of them) in a place
// that holds a schema, such as an item of oneOf, anyOf or allOf, `items`, a property or a discriminator
// mapping, is replaced by the schema of its type. A struct there is registered under components.schemas,
// like any model.
type SchemaProvider interface {
	OpenAPISchema() map[string]any
}

var schemaProviderType = reflect.TypeFor[SchemaProvider]()

// OneOf is a schema satisfied by exactly one of the models, for a SchemaProvider that stands for several types.
func OneOf(models ...any) map[string]any {
	return map[string]any{schemaKeyOneOf: slices.Clone(models)}
}

// AnyOf is a schema satisfied by at least one of the models.
func AnyOf(models ...any) map[string]any {
	return map[string]any{schemaKeyAnyOf: slices.Clone(models)}
}

// AllOf is a schema satisfied by all the models, for a type that extends others.
func AllOf(models ...any) map[string]any {
	return map[string]any{schemaKeyAllOf: slices.Clone(models)}
}

// WithDiscriminator adds a discriminator to a schema: property is the property whose value tells the models
// apart, and mapping names the model each value stands for. The models of the mapping must be structs, which
// the document references; a model that is not leaves its value out of the mapping.
func WithDiscriminator(schema map[string]any, property string, mapping map[string]any) map[string]any {
	out := maps.Clone(schema)
	if out == nil {
		out = map[string]any{}
	}
	discriminator := map[string]any{"propertyName": property}
	if len(mapping) > 0 {
		discriminator["mapping"] = maps.Clone(mapping)
	}
	out["discriminator"] = discriminator
	return out
}

// hasCustomSchema reports whether t has a schema of its own, from Config.TypeSchemas or a SchemaProvider.
func hasCustomSchema(t reflect.Type, reg *schemaRegistry) bool {
	if reg != nil {
		if _, ok := reg.typeSchemas[t]; ok {
			return true
		}
	}
	return reflect.PointerTo(t).Implements(schemaProviderType)
}

// customSchema is the schema t describes itself with. Go values in it are replaced by their types' schemas.
func customSchema(t reflect.Type, visited map[reflect.Type]bool, reg *schemaRegistry) map[string]any {
	if reg != nil {
		if configured, ok := reg.typeSchemas[t]; ok {
			return schemaMap(schemaValue(configured, visited, reg))
		}
	}
	provider, ok := reflect.TypeAssert[SchemaProvider](reflect.New(t))
	if !ok {
		return map[string]any{}
	}
	return schemaMap(schemaValue(provider.OpenAPISchema(), visited, reg))
}

func schemaMap(v any) map[string]any {
	if schema, ok := v.(map[string]any); ok {
		return schema
	}
	return map[string]any{}
}

// schemaValue returns the schema a value in a schema position stands for: a schema map has the models inside it
// resolved, a boolean or nil is kept, and any other value is a Go model that is reflected.
func schemaValue(v any, visited map[reflect.Type]bool, reg *schemaRegistry) any {
	switch value := v.(type) {
	case nil, bool, string:
		return value
	case reflect.Type:
		return typeSchemaOrAny(value, visited, reg)
	}
	if object, ok := schemautil.AsMap(v); ok {
		return resolveSchemaModels(object, visited, reg)
	}
	return typeSchemaOrAny(reflect.TypeOf(v), visited, reg)
}

func typeSchemaOrAny(t reflect.Type, visited map[reflect.Type]bool, reg *schemaRegistry) map[string]any {
	if schema := typeSchema(t, visited, reg); schema != nil {
		return schema
	}
	return map[string]any{}
}

// schemaPositions are the keywords whose value is a schema, or a list or map of schemas.
var (
	schemaKeywords    = []string{"items", schemaKeyAdditional, schemaKeyNot, "if", "then", "else", ruleContains, "propertyNames", "unevaluatedItems", "unevaluatedProperties"}
	schemaListKeyword = []string{schemaKeyOneOf, schemaKeyAnyOf, schemaKeyAllOf, "prefixItems"}
	schemaMapKeyword  = []string{schemaKeyProperties, "patternProperties", "$defs", "dependentSchemas"}
)

// resolveSchemaModels returns a copy of schema with the Go models in its schema positions replaced by schemas.
func resolveSchemaModels(schema map[string]any, visited map[reflect.Type]bool, reg *schemaRegistry) map[string]any {
	out := maps.Clone(schema)
	for _, key := range schemaKeywords {
		if value, ok := out[key]; ok {
			out[key] = schemaValue(value, visited, reg)
		}
	}
	for _, key := range schemaListKeyword {
		if list, ok := listOf(out[key]); ok {
			resolved := make([]any, len(list))
			for i := range list {
				resolved[i] = schemaValue(list[i], visited, reg)
			}
			out[key] = resolved
		}
	}
	for _, key := range schemaMapKeyword {
		if entries, ok := schemautil.AsMap(out[key]); ok {
			resolved := make(map[string]any, len(entries))
			for _, name := range sortedKeys(entries) {
				resolved[name] = schemaValue(entries[name], visited, reg)
			}
			out[key] = resolved
		}
	}
	if discriminator, ok := schemautil.AsMap(out["discriminator"]); ok {
		out["discriminator"] = resolveDiscriminator(discriminator, visited, reg)
	}
	return out
}

// listOf reads any slice as a list of values.
func listOf(v any) ([]any, bool) {
	if v == nil {
		return nil, false
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil, false
	}
	list := make([]any, rv.Len())
	for i := range list {
		list[i] = rv.Index(i).Interface()
	}
	return list, true
}

// resolveDiscriminator turns the models of a discriminator mapping into the references that name them.
func resolveDiscriminator(discriminator map[string]any, visited map[reflect.Type]bool, reg *schemaRegistry) map[string]any {
	out := maps.Clone(discriminator)
	mapping, ok := schemautil.AsMap(out["mapping"])
	if !ok {
		return out
	}
	resolved := make(map[string]any, len(mapping))
	for _, value := range sortedKeys(mapping) {
		target := mapping[value]
		if reference, isText := target.(string); isText {
			resolved[value] = reference
			continue
		}
		if reference, isRef := schemaMap(schemaValue(target, visited, reg))[schemaKeyRef].(string); isRef {
			resolved[value] = reference
		}
	}
	if len(resolved) == 0 {
		delete(out, "mapping")
	} else {
		out["mapping"] = resolved
	}
	return out
}
