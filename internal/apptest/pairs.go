package apptest

import (
	"iter"
)

// Pairs yields the name/value pairs of kv, the way the test helpers that take
// headers, cookies or other pairs as alternating variadic strings read them.
// It panics when kv has an odd length, because the last name would silently be
// dropped.
func Pairs(kv []string) iter.Seq2[string, string] {
	if len(kv)%2 != 0 {
		panic("apptest.Pairs: names and values have to come in pairs")
	}
	return func(yield func(string, string) bool) {
		for i := 0; i < len(kv); i += 2 {
			if !yield(kv[i], kv[i+1]) {
				return
			}
		}
	}
}
