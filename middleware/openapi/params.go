package openapi

import (
	"reflect"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

var modelTagByLocation = map[fiber.ParamLocation]string{
	fiber.ParamInQuery:  fiber.BindSourceQuery,
	fiber.ParamInHeader: fiber.BindSourceHeader,
	fiber.ParamInCookie: fiber.BindSourceCookie,
	fiber.ParamInPath:   fiber.BindSourceURI,
}

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

func taggedForAnotherLocation(field *reflect.StructField, tagKey string) bool {
	for _, key := range modelTagByLocation {
		if key != tagKey && field.Tag.Get(key) != "" {
			return true
		}
	}
	return false
}

// hasLocationOnlyFields reports whether a struct has a field bound only from the query, the path, the headers or
// the cookies, which makes it describe a request body differently from a response.
func hasLocationOnlyFields(t reflect.Type, tagKey string) bool {
	return hasLocationOnlyFieldsIn(t, tagKey, map[reflect.Type]bool{})
}

func hasLocationOnlyFieldsIn(t reflect.Type, tagKey string, seen map[reflect.Type]bool) bool {
	if t.Kind() != reflect.Struct || seen[t] {
		return false
	}
	seen[t] = true
	for i := range t.NumField() {
		field := t.Field(i)
		if field.Tag.Get(tagKey) == "" && taggedForAnotherLocation(&field, tagKey) {
			return true
		}
		if hasLocationOnlyFieldsIn(derefElem(field.Type), tagKey, seen) {
			return true
		}
	}
	return false
}

func appendModelFields(params []fiber.RouteParameter, t reflect.Type, in fiber.ParamLocation, tagKey string, reg *schemaRegistry, expanded map[reflect.Type]bool) []fiber.RouteParameter {
	for i := range t.NumField() {
		field := t.Field(i)
		name, _, _ := utils.CutByte(field.Tag.Get(tagKey), ',')
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
			if taggedForAnotherLocation(&field, tagKey) {
				continue
			}
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
			Required: required || in == fiber.ParamInPath,
		}
		if description, ok := schema["description"].(string); ok {
			param.Description = description
			delete(schema, "description")
		}
		if example, ok := schema["example"]; ok {
			param.Example = example
			delete(schema, "example")
		}
		param.Schema = reg.withRefSiblings(schema)
		params = append(params, param)
	}
	return params
}

// derefElem strips pointers, slices, arrays and maps from a type, down to what a field holds.
func derefElem(t reflect.Type) reflect.Type {
	for range maxPointerDepth {
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			t = t.Elem()
		default:
			return t
		}
	}
	return t
}
