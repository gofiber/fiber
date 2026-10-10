// Package deepcopy clones the loosely typed values OpenAPI metadata is made of
// (maps, slices and security requirements) so a copy shares no container with
// its source.
package deepcopy

import (
	"reflect"
	"slices"
)

// MaxDepth bounds the copy: a cyclic value would otherwise overflow the stack.
// Past it a value is shared rather than copied, and encoding/json reports the
// cycle when the document is encoded.
const MaxDepth = 100

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
	if depth >= MaxDepth {
		return src
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = valueDepth(value, depth+1)
	}
	return dst
}

func valueDepth(src any, depth int) any {
	if src == nil || depth >= MaxDepth {
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

// reflected clones map and slice values of any other concrete type.
func reflected(src any, depth int) any {
	v := reflect.ValueOf(src)
	switch v.Kind() {
	case reflect.Slice:
		if v.IsNil() {
			return src
		}
		copied := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			// A nil element is an invalid reflect.Value; keep the zero value.
			if elem := valueDepth(v.Index(i).Interface(), depth+1); elem != nil {
				copied.Index(i).Set(reflect.ValueOf(elem))
			}
		}
		return copied.Interface()
	case reflect.Map:
		if v.IsNil() {
			return src
		}
		copied := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			// SetMapIndex with an invalid value deletes the key; use the zero value instead.
			val := reflect.Zero(v.Type().Elem())
			if elem := valueDepth(iter.Value().Interface(), depth+1); elem != nil {
				val = reflect.ValueOf(elem)
			}
			copied.SetMapIndex(iter.Key(), val)
		}
		return copied.Interface()
	default:
		return src
	}
}
