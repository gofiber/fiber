package binder

import (
	"fmt"

	"github.com/gofiber/utils/v2"
)

// XMLBinding is the XML binder for XML request body.
type XMLBinding struct {
	XMLDecoder utils.XMLUnmarshal
}

// Name returns the binding name.
func (*XMLBinding) Name() string {
	return "xml"
}

// Bind parses the request body as XML and returns the result.
func (b *XMLBinding) Bind(body []byte, out any) error {
	if err := b.XMLDecoder(body, out); err != nil {
		return fmt.Errorf("failed to unmarshal xml: %w", err)
	}

	return nil
}

// Reset resets the XMLBinding binder.
func (b *XMLBinding) Reset() {
	b.XMLDecoder = nil
}
