package utils

import (
	"encoding/json"
)

// JSONExecutor provides the minimal API for basic JSON engine functionality
type JSONExecutor interface {
	Marshal(interface{}) ([]byte, error)
}

// DefaultJSONExecutor is a blank structure, in place to satisfy the API
// of a JSONExecutor
type DefaultJSONExecutor struct {
}

// Marshal takes in an arbitrary interface and returns an encoding of
// the provided interface
func (d *DefaultJSONExecutor) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
