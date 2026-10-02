package openapi

import (
	"reflect"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// modelTagByLocation maps a parameter location to the struct tag Bind reads
// for it, so a model documents the names the binder actually uses.
var modelTagByLocation = map[string]string{
	"query":  fiber.BindSourceQuery,
	"header": fiber.BindSourceHeader,
	"cookie": fiber.BindSourceCookie,
	"path":   fiber.BindSourceURI,
}

// expandParameterModels turns each declared model into one parameter per
// exported field. Embedded structs are flattened as the binder flattens them.
func expandParameterModels(models []fiber.RouteParameterModel, reg *schemaRegistry) []fiber.RouteParameter {
	var params []fiber.RouteParameter
	for i := range models {
		model := &models[i]
		if model.Model == nil {
			continue
		}
		t := derefType(reflect.TypeOf(model.Model))
		if t.Kind() != reflect.Struct {
			continue
		}
		params = appendModelFields(params, t, model.In, modelTagByLocation[model.In], reg, map[reflect.Type]bool{t: true})
	}
	return params
}

func appendModelFields(params []fiber.RouteParameter, t reflect.Type, in, tagKey string, reg *schemaRegistry, expanded map[reflect.Type]bool) []fiber.RouteParameter {
	for i := range t.NumField() {
		field := t.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get(tagKey), ",")
		if name == "-" {
			continue
		}

		fieldType := derefType(field.Type)
		if field.Anonymous && fieldType.Kind() == reflect.Struct && fieldType != timeType && name == "" {
			if !expanded[fieldType] {
				expanded[fieldType] = true
				params = appendModelFields(params, fieldType, in, tagKey, reg, expanded)
			}
			continue
		}
		if !field.IsExported() {
			continue
		}
		if name == "" {
			name = field.Name
		}

		schema := typeSchema(field.Type, nil, reg)
		if schema == nil {
			continue
		}
		applyOpenAPITag(&field, schema)
		required := applyValidateTag(&field, schema)

		param := fiber.RouteParameter{
			Name:     name,
			In:       in,
			Schema:   schema,
			Required: required || in == paramLocationPath,
		}
		// A description and an example belong to the Parameter Object, not
		// to its schema.
		if description, ok := schema["description"].(string); ok {
			param.Description = description
			delete(schema, "description")
		}
		if example, ok := schema["example"]; ok {
			param.Example = example
			delete(schema, "example")
		}
		params = append(params, param)
	}
	return params
}
