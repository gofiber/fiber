package binder

// URIBinding is the binder implementation for populating values from route parameters.
type URIBinding struct{}

// Name returns the binding name.
func (*URIBinding) Name() string {
	return bindingURI
}

// Bind parses the URI parameters and returns the result.
func (b *URIBinding) Bind(params []string, paramsFunc func(key string, defaultValue ...string) string, out any) error {
	data := acquireBindData(out, len(params))
	defer releaseBindData(data)

	for _, param := range params {
		data.add(param, paramsFunc(param))
	}

	return data.parse(b.Name(), out)
}

// Reset resets URIBinding binder.
func (*URIBinding) Reset() {
	// Nothing to reset
}
