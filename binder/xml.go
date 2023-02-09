package binder

import (
	"encoding/xml"
)

type xmlBinding struct{}

func (*xmlBinding) Name() string {
	return "xml"
}

func (*xmlBinding) Bind(body []byte, out any) error {
	return xml.Unmarshal(body, out)
}
