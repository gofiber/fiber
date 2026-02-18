package binder

import (
	"reflect"
)

const uriTagName = "uri"

// URIBinding is the binder implementation for populating values from route parameters.
type URIBinding struct{}

// Name returns the binding name.
func (*URIBinding) Name() string {
	return uriTagName
}

// Bind parses the URI parameters and returns the result.
func (*URIBinding) Bind(params []string, paramsFunc func(key string, defaultValue ...string) string, out any) error {
	data := make(map[string][]string, len(params))
	for _, param := range params {
		data[param] = append(data[param], paramsFunc(param))
	}

	return parse(uriTag(out), out, data)
}

// uriTag returns the struct tag to use for URI binding.
// It returns "params" if any exported field carries a params tag,
// otherwise it returns the default "uri".
func uriTag(out any) string {
	t := reflect.TypeOf(out)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return uriTagName
	}
	for i := range t.NumField() {
		if f := t.Field(i); f.IsExported() && f.Tag.Get("params") != "" {
			return "params"
		}
	}
	return uriTagName
}

// Reset resets URIBinding binder.
func (*URIBinding) Reset() {
	// Nothing to reset
}
