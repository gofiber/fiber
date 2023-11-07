//go:build go1.18

package fiber

import (
	"testing"
)

// go test -v -run=^$ -fuzz=FuzzUtilsGetOffer
func FuzzUtilsGetOffer(f *testing.F) {
	inputs := []string{
		`application/json; v=1; foo=bar; q=0.938; extra=param, text/plain;param="big fox"; q=0.43`,
		`text/html, application/xhtml+xml, application/xml;q=0.9, */*;q=0.8`,
		`*/*`,
		`text/plain; q=0.5, text/html, text/x-dvi; q=0.8, text/x-c`,
	}
	for _, input := range inputs {
		f.Add(input)
	}
	f.Fuzz(func(_ *testing.T, spec string) {
		getOffer(spec, acceptsOfferType, `application/json;version=1;v=1;foo=bar`, `text/plain;param="big fox"`)
	})
}
