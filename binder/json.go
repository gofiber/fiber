package binder

import (
	"github.com/gofiber/utils/v2"
)

// JSONBinding is the JSON binder for JSON request body.
type JSONBinding struct {
	JSONDecoder utils.JSONUnmarshal
}

// Name returns the binding name.
func (*JSONBinding) Name() string {
	return "json"
}

// Bind parses the request body as JSON and returns the result.
func (b *JSONBinding) Bind(body []byte, out any) error {
	return b.JSONDecoder(body, out)
}

// Reset resets the JSONBinding binder.
func (b *JSONBinding) Reset() {
	b.JSONDecoder = nil
}
