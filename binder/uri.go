package binder

// uriBinding is the URI binder for URI parameters.
type uriBinding struct{}

// Name returns the binding name.
func (*uriBinding) Name() string {
	return "uri"
}

// Bind parses the URI parameters and returns the result.
func (b *uriBinding) Bind(params []string, paramsFunc func(key string, defaultValue ...string) string, out any) error {
	data := make(map[string][]string, len(params))
	for _, param := range params {
		data[param] = append(data[param], paramsFunc(param))
	}

	return parse(b.Name(), out, data)
}
