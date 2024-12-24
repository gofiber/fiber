package binder

// uriBinding is the URI binder for URI parameters.
type URIBinding struct{}

// Name returns the binding name.
func (*URIBinding) Name() string {
	return "uri"
}

// Bind parses the URI parameters and returns the result.
func (b *URIBinding) Bind(params []string, paramsFunc func(key string, defaultValue ...string) string, out any) error {
	data := make(map[string][]string, len(params))
	for _, param := range params {
		data[param] = append(data[param], paramsFunc(param))
	}

	return parse(b.Name(), out, data)
}

// Reset resets URIBinding binder.
func (*URIBinding) Reset() {
	// Nothing to reset
}
