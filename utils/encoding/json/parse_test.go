package json

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseString(t *testing.T) {
	tests := []struct {
		in  string
		out string
		ext string
	}{
		{`""`, `""`, ``},
		{`"1234567890"`, `"1234567890"`, ``},
		{`"Hello World!"`, `"Hello World!"`, ``},
		{`"Hello\"World!"`, `"Hello\"World!"`, ``},
		{`"\\"`, `"\\"`, ``},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			out, ext, err := parseString([]byte(test.in))

			if err != nil {
				t.Errorf("%s => %s", test.in, err)
				return
			}

			if s := string(out); s != test.out {
				t.Error("invalid output")
				t.Logf("expected: %s", test.out)
				t.Logf("found:    %s", s)
			}

			if s := string(ext); s != test.ext {
				t.Error("invalid extra bytes")
				t.Logf("expected: %s", test.ext)
				t.Logf("found:    %s", s)
			}
		})
	}
}

func TestAppendToLower(t *testing.T) {
	tests := []string{
		"",
		"A",
		"a",
		"__segment_internal",
		"someFieldWithALongBName",
		"Hello World!",
		"Hello\"World!",
		"Hello\\World!",
		"Hello\nWorld!",
		"Hello\rWorld!",
		"Hello\tWorld!",
		"Hello\bWorld!",
		"Hello\fWorld!",
		"你好",
		"<",
		">",
		"&",
		"\u001944",
		"\u00c2e>",
		"\u00c2V?",
		"\u000e=8",
		"\u001944\u00c2e>\u00c2V?\u000e=8",
		"ir\u001bQJ\u007f\u0007y\u0015)",
	}

	for _, test := range tests {
		s1 := strings.ToLower(test)
		s2 := string(appendToLower(nil, []byte(test)))

		if s1 != s2 {
			t.Error("lowercase values mismatch")
			t.Log("expected:", s1)
			t.Log("found:   ", s2)
		}
	}
}

func BenchmarkParseString(b *testing.B) {
	s := []byte(`"__segment_internal"`)

	for i := 0; i != b.N; i++ {
		parseString(s)
	}
}

func BenchmarkToLower(b *testing.B) {
	s := []byte("someFieldWithALongName")

	for i := 0; i != b.N; i++ {
		bytes.ToLower(s)
	}
}

func BenchmarkAppendToLower(b *testing.B) {
	a := []byte(nil)
	s := []byte("someFieldWithALongName")

	for i := 0; i != b.N; i++ {
		a = appendToLower(a[:0], s)
	}
}

var benchmarkHasPrefixString = []byte("some random string")
var benchmarkHasPrefixResult = false

func BenchmarkHasPrefix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		benchmarkHasPrefixResult = hasPrefix(benchmarkHasPrefixString, "null")
	}
}

func BenchmarkHasNullPrefix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		benchmarkHasPrefixResult = hasNullPrefix(benchmarkHasPrefixString)
	}
}

func BenchmarkHasTruePrefix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		benchmarkHasPrefixResult = hasTruePrefix(benchmarkHasPrefixString)
	}
}

func BenchmarkHasFalsePrefix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		benchmarkHasPrefixResult = hasFalsePrefix(benchmarkHasPrefixString)
	}
}

func BenchmarkParseStringEscapeNone(b *testing.B) {
	var j = []byte(`"` + strings.Repeat(`a`, 1000) + `"`)
	var s string
	b.SetBytes(int64(len(j)))

	for i := 0; i < b.N; i++ {
		if err := Unmarshal(j, &s); err != nil {
			b.Fatal(err)
		}
		s = ""
	}
}

func BenchmarkParseStringEscapeOne(b *testing.B) {
	var j = []byte(`"` + strings.Repeat(`a`, 998) + `\n"`)
	var s string
	b.SetBytes(int64(len(j)))

	for i := 0; i < b.N; i++ {
		if err := Unmarshal(j, &s); err != nil {
			b.Fatal(err)
		}
		s = ""
	}
}

func BenchmarkParseStringEscapeAll(b *testing.B) {
	var j = []byte(`"` + strings.Repeat(`\`, 1000) + `"`)
	var s string
	b.SetBytes(int64(len(j)))

	for i := 0; i < b.N; i++ {
		if err := Unmarshal(j, &s); err != nil {
			b.Fatal(err)
		}
		s = ""
	}
}
