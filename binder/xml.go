package binder

import (
	"encoding/xml"
	"fmt"
)

type xmlBinding struct{}

func (*xmlBinding) Name() string {
	return "xml"
}

func (*xmlBinding) Bind(body []byte, out any) error {
	if err := xml.Unmarshal(body, out); err != nil {
		return fmt.Errorf("failed to unmarshal xml: %w", err)
	}

	return nil
}
