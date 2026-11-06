// Package schemautil holds the JSON Schema rules the root package and the openapi middleware share.
package schemautil

import (
	"encoding/json"
	"reflect"
)

// DefaultType reports the type to add to a parameter schema that names none, or false if it already says what it is.
func DefaultType(schema map[string]any, fallback string) (string, bool) {
	for _, keyword := range [...]string{"type", "$ref", "allOf", "anyOf", "oneOf", "not", "const"} {
		if _, has := schema[keyword]; has {
			return "", false
		}
	}
	values, hasEnum := schema["enum"]
	if !hasEnum {
		return fallback, fallback != ""
	}
	return enumType(values)
}

const (
	typeInteger = "integer"
	typeNumber  = "number"
)

var jsonNumberType = reflect.TypeFor[json.Number]()

// enumType names the JSON Schema type all the enum's values share; nulls are ignored.
func enumType(values any) (string, bool) {
	list := reflect.ValueOf(values)
	if list.Kind() != reflect.Slice {
		return "", false
	}
	shared := ""
	for i := range list.Len() {
		item := list.Index(i)
		if item.Kind() == reflect.Interface {
			item = item.Elem()
		}
		if !item.IsValid() {
			continue
		}
		var typ string
		switch {
		case item.Type() == jsonNumberType:
			typ = typeNumber
		case item.Kind() == reflect.String:
			typ = "string"
		case item.Kind() == reflect.Bool:
			typ = "boolean"
		case item.CanInt() || item.CanUint():
			typ = typeInteger
		case item.CanFloat():
			typ = typeNumber
		default:
			return "", false
		}
		switch {
		case shared == "" || shared == typ:
			shared = typ
		case (shared == typeInteger && typ == typeNumber) || (shared == typeNumber && typ == typeInteger):
			shared = typeNumber
		default:
			return "", false
		}
	}
	return shared, shared != ""
}

// AsMap returns v as a map[string]any when it is one, or a defined string-keyed map such as fiber.Map, copied.
func AsMap(v any) (map[string]any, bool) {
	if plain, ok := v.(map[string]any); ok {
		return plain, true
	}
	value := reflect.ValueOf(v)
	if !value.IsValid() || value.Kind() != reflect.Map {
		return nil, false
	}
	typ := value.Type()
	if typ.Key().Kind() != reflect.String || typ.Elem().Kind() != reflect.Interface || typ.Elem().NumMethod() != 0 {
		return nil, false
	}
	out := make(map[string]any, value.Len())
	for iter := value.MapRange(); iter.Next(); {
		out[iter.Key().String()] = iter.Value().Interface()
	}
	return out, true
}
