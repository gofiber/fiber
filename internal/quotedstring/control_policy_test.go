package quotedstring

import (
	"fmt"
	"strings"
	"testing"
)

func Test_Escape_AllBytes(t *testing.T) {
	t.Parallel()
	for value := range 256 {
		c := byte(value)
		raw := string([]byte{c})
		escaped := raw
		switch c {
		case '\\', '"':
			escaped = "\\" + escaped
		case '\r':
			escaped = `\r`
		case '\n':
			escaped = `\n`
		default:
			if c < 0x20 || c == 0x7f {
				escaped = fmt.Sprintf("%%%02X", c)
			}
		}
		for n := 1; n <= 33; n++ {
			for pos := range n {
				prefix, suffix := strings.Repeat("a", pos), strings.Repeat("z", n-pos-1)
				input := prefix + raw + suffix
				if got, want := Escape(input), prefix+escaped+suffix; got != want {
					t.Fatalf("n=%d pos=%d byte=%02x: got %q, want %q", n, pos, c, got, want)
				}
			}
		}
	}
}

func Benchmark_Escape(b *testing.B) {
	for _, input := range []string{"simple", "a-long-clean-header-value", "café", `a-long-value-with-a-quote"`, "value\x00"} {
		b.Run(input, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				Escape(input)
			}
		})
	}
}
