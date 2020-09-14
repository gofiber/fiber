package ascii

import (
	"strings"
	"testing"
)

var testStrings = [...]string{
	"",
	"hello",
	"Hello World!",
	"Hello\"World!",
	"Hello\\World!",
	"Hello\nWorld!",
	"Hello\rWorld!",
	"Hello\tWorld!",
	"Hello\bWorld!",
	"Hello\fWorld!",
	"H~llo World!",
	"H~llo",
	"你好",
	"~",
	"\x80",
	"\x7F",
	"\xFF",
	"some kind of long string with only ascii characters.",
	"some kind of long string with a non-ascii character at the end.\xff",
	strings.Repeat("1234567890", 1000),
}

func testString(s string, f func(byte) bool) bool {
	for i := range s {
		if !f(s[i]) {
			return false
		}
	}
	return true
}

func testValid(s string) bool {
	return testString(s, ValidByte)
}

func testValidPrint(s string) bool {
	return testString(s, ValidPrintByte)
}

func TestValid(t *testing.T) {
	testValidationFunction(t, testValid, ValidString)
}

func TestValidPrint(t *testing.T) {
	testValidationFunction(t, testValidPrint, ValidPrintString)
}

func testValidationFunction(t *testing.T, reference, function func(string) bool) {
	for _, test := range testStrings {
		t.Run(limit(test), func(t *testing.T) {
			expect := reference(test)

			if valid := function(test); expect != valid {
				t.Errorf("expected %t but got %t", expect, valid)
			}
		})
	}
}

func BenchmarkValid(b *testing.B) {
	benchmarkValidationFunction(b, ValidString)
}

func BenchmarkValidPrint(b *testing.B) {
	benchmarkValidationFunction(b, ValidPrintString)
}

func benchmarkValidationFunction(b *testing.B, function func(string) bool) {
	for _, test := range testStrings {
		b.Run(limit(test), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = function(test)
			}
			b.SetBytes(int64(len(test)))
		})
	}
}

func limit(s string) string {
	if len(s) > 17 {
		return s[:17] + "..."
	}
	return s
}
