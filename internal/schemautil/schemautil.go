// Package schemautil holds the JSON Schema rules the root package and the openapi middleware share.
package schemautil

import (
	"reflect"
)

// DefaultType reports the type to add to a parameter schema that names none. A schema that already
// says what it is through a type, a $ref, a composition or a const gets nothing, and an enum
// takes the type its values share.
func DefaultType(schema map[string]any) (string, bool) {
	for _, keyword := range [...]string{"type", "$ref", "allOf", "anyOf", "oneOf", "not", "const"} {
		if _, has := schema[keyword]; has {
			return "", false
		}
	}
	values, hasEnum := schema["enum"]
	if !hasEnum {
		return "string", true
	}
	return enumType(values)
}

// enumType names the JSON Schema type all the enum's values share.
func enumType(values any) (string, bool) {
	list := reflect.ValueOf(values)
	if list.Kind() != reflect.Slice || list.Len() == 0 {
		return "", false
	}
	shared := ""
	for i := range list.Len() {
		item := list.Index(i)
		if item.Kind() == reflect.Interface {
			item = item.Elem()
		}
		var typ string
		switch item.Kind() {
		case reflect.String:
			typ = "string"
		case reflect.Bool:
			typ = "boolean"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			typ = "integer"
		case reflect.Float32, reflect.Float64:
			typ = "number"
		default:
			return "", false
		}
		if shared != "" && shared != typ {
			return "", false
		}
		shared = typ
	}
	return shared, true
}
