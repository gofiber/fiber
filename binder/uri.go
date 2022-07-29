package binder

type uriBinding struct{}

func (*uriBinding) Name() string {
	return "uri"
}

func (b *uriBinding) Bind(params []string, paramsFunc func(key string, defaultValue ...string) string, out any) error {
	data := make(map[string][]string, len(params))
	for _, param := range params {
		data[param] = append(data[param], paramsFunc(param))
	}

	return parse(b.Name(), out, data)
}
