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

// hasLocationOnlyFields reports whether the schema of a struct has a field bound only from the query, the path,
// the headers or the cookies, which makes it describe a request body differently from a response. Fields the
// schema leaves out (a "-" tag, an unexported field) and types with a schema of their own do not count. The answer
// is remembered per registry, as it only depends on the type.
func (reg *schemaRegistry) hasLocationOnlyFields(t reflect.Type) bool {
	if answer, known := reg.locationOnly[t]; known {
		return answer
	}
	answer := locationOnlyIn(t, reg.tagKey(), reg, map[reflect.Type]bool{})
	if reg.locationOnly == nil {
		reg.locationOnly = make(map[reflect.Type]bool)
	}
	reg.locationOnly[t] = answer
	return answer
}

func locationOnlyIn(t reflect.Type, tagKey string, reg *schemaRegistry, seen map[reflect.Type]bool) bool {
	if t.Kind() != reflect.Struct || seen[t] || t == timeType || hasCustomSchema(t, reg) {
		return false
	}
	seen[t] = true
	for i := range t.NumField() {
		field := t.Field(i)
		if parseJSONTag(&field, tagKey).skip {
			continue
		}
		elem := derefElem(field.Type)
		if !field.IsExported() && (!field.Anonymous || elem.Kind() != reflect.Struct) {
			continue
		}
		if field.Tag.Get(tagKey) == "" && taggedForAnotherLocation(&field, tagKey) {
			return true
		}
		// A named struct held by a field becomes its own component with its own key; only an embedded struct
		// and an unnamed one are written into this schema.
		if (field.Anonymous || elem.Name() == "") && locationOnlyIn(elem, tagKey, reg, seen) {
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
