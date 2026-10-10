// Package deepcopy clones the loosely typed values OpenAPI metadata is made of
// (maps, slices and security requirements) so a copy shares no container with
// its source.
package deepcopy

import (
	"reflect"
	"slices"
)

// maxDepth bounds the recursion: a very deep value would otherwise overflow the
// stack. Past it a value is shared rather than copied.
const maxDepth = 100

// copier clones values while tracking the containers it is in the middle of
// copying. A pointer, map or slice met again while still being copied is a
// cycle: the copy of it so far is reused, so a value that points back to itself,
// however many times, is copied once and its copy points back to the copy. Only
// the current path is tracked, which keeps the common acyclic copy free of any
// bookkeeping allocation.
type copier struct {
	path []frame
}

// frame is a container being copied and the copy made of it.
type frame struct {
	copied reflect.Value
	key    visit
}

// visit identifies a container by its address, type and length.
type visit struct {
	typ  reflect.Type
	ptr  uintptr
	size int
}

// Map copies a raw OpenAPI object. A nil map stays nil and an empty one stays
// empty, so "properties": {} does not become null.
func Map(src map[string]any) map[string]any {
	var c copier
	return c.mapValue(src, 0)
}

// Value copies any value, descending through maps, slices, arrays, pointers and
// the exported fields of structs.
func Value(src any) any {
	var c copier
	return c.value(src, 0)
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

// enter notes that v is being copied into copied; call leave when it is done.
func (c *copier) enter(v, copied reflect.Value) {
	c.path = append(c.path, frame{key: visitOf(v), copied: copied})
}

func (c *copier) leave() {
	c.path = c.path[:len(c.path)-1]
}

// recall returns the copy under way of v, if v is on the current path.
func (c *copier) recall(v reflect.Value) (reflect.Value, bool) {
	if len(c.path) == 0 {
		return reflect.Value{}, false
	}
	key := visitOf(v)
	for _, f := range slices.Backward(c.path) {
		if f.key == key {
			return f.copied, true
		}
	}
	return reflect.Value{}, false
}

func visitOf(v reflect.Value) visit {
	n := 0
	if v.Kind() == reflect.Slice {
		n = v.Len()
	}
	return visit{typ: v.Type(), ptr: v.Pointer(), size: n}
}

func (c *copier) mapValue(src map[string]any, depth int) map[string]any {
	if src == nil || depth >= maxDepth {
		return src
	}
	rv := reflect.ValueOf(src)
	if copied, ok := c.recall(rv); ok {
		return copied.Interface().(map[string]any) //nolint:errcheck,forcetypeassert // remembered under this exact type
	}
	dst := make(map[string]any, len(src))
	c.enter(rv, reflect.ValueOf(dst))
	for key, value := range src {
		dst[key] = c.value(value, depth+1)
	}
	c.leave()
	return dst
}

func (c *copier) value(src any, depth int) any {
	if src == nil || depth >= maxDepth {
		return src
	}
	switch v := src.(type) {
	case map[string]any:
		return c.mapValue(v, depth)
	case []string:
		return slices.Clone(v)
	case string, bool, int, int64, float64:
		return src
	default:
		return c.copyValue(reflect.ValueOf(src), depth).Interface()
	}
}

func (c *copier) copyValue(v reflect.Value, depth int) reflect.Value {
	if !v.IsValid() || depth >= maxDepth {
		return v
	}
	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(c.copyValue(v.Elem(), depth+1))
		return out
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		if copied, ok := c.recall(v); ok {
			return copied
		}
		out := reflect.New(v.Type().Elem())
		c.enter(v, out)
		out.Elem().Set(c.copyValue(v.Elem(), depth+1))
		c.leave()
		return out
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		if copied, ok := c.recall(v); ok {
			return copied
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		c.enter(v, out)
		iter := v.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), c.copyValue(iter.Value(), depth+1))
		}
		c.leave()
		return out
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		if v.Len() > 0 {
			if copied, ok := c.recall(v); ok {
				return copied
			}
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		if v.Len() > 0 {
			c.enter(v, out)
		}
		for i := range v.Len() {
			out.Index(i).Set(c.copyValue(v.Index(i), depth+1))
		}
		if v.Len() > 0 {
			c.leave()
		}
		return out
	case reflect.Array:
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		for i := range v.Len() {
			out.Index(i).Set(c.copyValue(v.Index(i), depth+1))
		}
		return out
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				out.Field(i).Set(c.copyValue(v.Field(i), depth+1))
			}
		}
		return out
	default:
		return v
	}
}
