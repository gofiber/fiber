package binder

import (
	"encoding/xml"
	"fmt"
)

// xmlBinding is the XML binder for XML request body.
type xmlBinding struct{}

// Name returns the binding name.
func (*xmlBinding) Name() string {
	return "xml"
}

// Bind parses the request body as XML and returns the result.
func (*xmlBinding) Bind(body []byte, out any) error {
	if err := xml.Unmarshal(body, out); err != nil {
		return fmt.Errorf("failed to unmarshal xml: %w", err)
	}

	return nil
}
