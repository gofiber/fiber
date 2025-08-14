package utils

import "testing"

func TestEscapePath(t *testing.T) {
	cases := map[string]string{
		"/":        "/",
		"/foo/bar": "/foo/bar",
		"/foo bar": "/foo+bar",
		"/does/not/exist<script>alert('foo');</script>": "/does/not/exist%3Cscript%3Ealert%28%27foo%27%29%3B%3C/script%3E",
	}
	for input, expected := range cases {
		if got := EscapePath(input); got != expected {
			t.Errorf("EscapePath(%q) = %q; want %q", input, got, expected)
		}
	}
}

func BenchmarkEscapePath(b *testing.B) {
	samples := []string{
		"/foo/bar",
		"/does/not/exist<script>alert('foo');</script>",
	}
	for _, sample := range samples {
		b.Run(sample, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = EscapePath(sample)
			}
		})
	}
}
