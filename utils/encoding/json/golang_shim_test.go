// This file is a shim for dependencies of golang_*_test.go files that are normally provided by the standard library.
// It helps importing those files with minimal changes.
package json

import (
	"bytes"
	"reflect"
	"sync"
	"testing"
)

// Field cache used in golang_bench_test.go
var fieldCache = sync.Map{}

func cachedTypeFields(reflect.Type) {}

// Fake test env for golang_bench_test.go
type testenvShim struct {
}

func (ts testenvShim) Builder() string {
	return ""
}

var testenv testenvShim

// Fake scanner for golang_decode_test.go
type scanner struct {
}

func checkValid(in []byte, scan *scanner) error {
	return nil
}

// Actual isSpace implementation
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// Fake encoder for golang_encode_test.go
type encodeState struct {
	Buffer bytes.Buffer
}

func (es *encodeState) string(s string, escapeHTML bool) {
}

func (es *encodeState) stringBytes(b []byte, escapeHTML bool) {
}

// Fake number test
func isValidNumber(n string) bool {
	return true
}

func assertErrorPresence(t *testing.T, expected error, actual error, prefixes ...interface{}) {
	if expected != nil && actual == nil {
		errorWithPrefixes(t, prefixes, "expected error, but did not get an error")
	} else if expected == nil && actual != nil {
		errorWithPrefixes(t, prefixes, "did not expect error but got %v", actual)
	}
}

func errorWithPrefixes(t *testing.T, prefixes []interface{}, format string, elements ...interface{}) {
	fullFormat := format
	allElements := append(prefixes, elements...)

	for range prefixes {
		fullFormat = "%v: " + fullFormat
	}
	t.Errorf(fullFormat, allElements...)
}
