package extractors

import (
	"testing"
)

// FuzzIsValidToken68 holds the table scan to the range-compare scan it
// replaced, on whatever the fuzzer can think of.
//
// go test -v -run=^$ -fuzz=FuzzIsValidToken68
func FuzzIsValidToken68(f *testing.F) {
	for _, seed := range token68Samples {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, token string) {
		if isValidToken68(token) != isValidToken68Reference(token) {
			t.Fatalf("table and reference disagree on %q", token)
		}
	})
}
