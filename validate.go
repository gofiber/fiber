package fiber

type Validator interface {
	Validate(v any) error
}
