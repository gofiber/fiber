package binder

import (
	"github.com/gofiber/utils/v2"
)

// jsonBinding is the JSON binder for JSON request body.
type jsonBinding struct{}

// Name returns the binding name.
func (*jsonBinding) Name() string {
	return "json"
}

// Bind parses the request body as JSON and returns the result.
func (*jsonBinding) Bind(body []byte, jsonDecoder utils.JSONUnmarshal, out any) error {
	return jsonDecoder(body, out)
}
