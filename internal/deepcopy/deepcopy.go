// Package deepcopy clones the loosely typed values OpenAPI metadata is made of
// (maps, slices and security requirements) so a copy shares no container with
// its source.
package deepcopy

import (
	"reflect"
	"slices"
)

// maxDepth bounds the copy: a cyclic value would otherwise overflow the stack.
// Past it a value is shared rather than copied, and encoding/json reports the
// cycle when the document is encoded.
const maxDepth = 100

// Map copies a raw OpenAPI object. A nil map stays nil and an empty one stays
// empty, so "properties": {} does not become null.
func Map(src map[string]any) map[string]any {
	return mapDepth(src, 0)
}

// Value copies any value, descending through maps and slices of any element type.
func Value(src any) any {
	return valueDepth(src, 0)
}

// Security copies security requirements and their scope lists. A nil list stays
// nil and an empty one stays empty, which differ: empty means "no authentication".
// A scope list stays non-nil so it marshals as [] rather than null.
func Security(src []map[string][]string) []map[string][]string {
	if src == nil {
		return nil
	}
	cloned := make([]map[string][]string, len(src))
	for i, requirement := range src {
		entry := make(map[string][]string, len(requirement))
		for scheme, scopes := range requirement {
			entry[scheme] = append(make([]string, 0, len(scopes)), scopes...)
		}
		cloned[i] = entry
	}
	return cloned
}

func mapDepth(src map[string]any, depth int) map[string]any {
	if src == nil {
		return nil
	}
	if depth >= maxDepth {
		return src
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = valueDepth(value, depth+1)
	}
	return dst
}

func valueDepth(src any, depth int) any {
	if src == nil || depth >= maxDepth {
		return src
	}
	switch value := src.(type) {
	case map[string]any:
		return mapDepth(value, depth)
	case []any:
		copied := make([]any, len(value))
		for i := range value {
			copied[i] = valueDepth(value[i], depth+1)
		}
		return copied
	case []map[string]any:
		copied := make([]map[string]any, len(value))
		for i := range value {
			copied[i] = mapDepth(value[i], depth+1)
		}
		return copied
	case []string:
		return slices.Clone(value)
	default:
		return reflected(src, depth)
	}
}

// reflected clones any other value: maps, slices, arrays, pointers and the exported
// fields of structs, recursively. Unexported struct fields are copied as they are.
func reflected(src any, depth int) any {
	return copyValue(reflect.ValueOf(src), depth).Interface()
}

func copyValue(v reflect.Value, depth int) reflect.Value {
	if !v.IsValid() || depth >= maxDepth {
		return v
	}
	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(copyValue(v.Elem(), depth+1))
		return out
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type().Elem())
		out.Elem().Set(copyValue(v.Elem(), depth+1))
		return out
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), copyValue(iter.Value(), depth+1))
		}
		return out
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			out.Index(i).Set(copyValue(v.Index(i), depth+1))
		}
		return out
	case reflect.Array:
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		for i := range v.Len() {
			out.Index(i).Set(copyValue(v.Index(i), depth+1))
		}
		return out
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				out.Field(i).Set(copyValue(v.Field(i), depth+1))
			}
		}
		return out
	default:
		return v
	}
}
