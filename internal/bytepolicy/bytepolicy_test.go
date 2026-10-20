package bytepolicy

import (
	"strings"
	"testing"
)

func referenceIndex(s string, policy Policy) int {
	for i := range len(s) {
		c := s[i]
		switch policy {
		case VisibleASCII:
			if c < ' ' || c > '~' {
				return i
			}
		case HeaderValue:
			if c != '\t' && (c < ' ' || c > '~') {
				return i
			}
		case QuotedString:
			if c < ' ' || c == 127 || c == '"' || c == '\\' {
				return i
			}
		}
	}
	return -1
}

func Test_Index_AllBytes(t *testing.T) {
	t.Parallel()
	for _, policy := range []Policy{VisibleASCII, HeaderValue, QuotedString} {
		t.Run(map[Policy]string{VisibleASCII: "visible ASCII", HeaderValue: "header value", QuotedString: "quoted string"}[policy], func(t *testing.T) {
			t.Parallel()
			for n := 1; n <= 33; n++ {
				for pos := range n {
					input := []byte(strings.Repeat("a", n))
					for value := range 256 {
						input[pos] = byte(value)
						s := string(input)
						want := referenceIndex(s, policy)
						if got := Index(s, policy); got != want {
							t.Fatalf("string policy=%d n=%d pos=%d byte=%02x: got %d, want %d", policy, n, pos, value, got, want)
						}
					}
				}
			}
		})
	}
}

func Fuzz_Index(f *testing.F) {
	for _, seed := range []string{"", "clean", "\t\x00\x7f", `"\`, "é\x00é", "abcdefgh\x80\x00jklmnop\t"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, policy := range []Policy{VisibleASCII, HeaderValue, QuotedString} {
			want := referenceIndex(s, policy)
			if got := Index(s, policy); got != want {
				t.Fatalf("string policy=%d input=%q: got %d, want %d", policy, s, got, want)
			}
		}
	})
}
