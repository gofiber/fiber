//go:build go1.18

package fiber

import (
	"testing"
)

// go test -v -run=^$ -fuzz=Fuzz_Utils_GetOffer
func FuzzUtilsGetOffer(f *testing.F) {
	bigHeader := `application/json; v=1; foo=bar; q=0.938; extra=param, text/plain;param="big fox"; q=0.43`
	f.Add(bigHeader)
	f.Fuzz(func(_ *testing.T, spec string) {
		getOffer(spec, acceptsOfferType, `application/json;version=1;v=1;foo=bar`, `text/plain;param="big fox"`)
	})
}
