package requestid

import (
	"strings"
	"testing"
)

func Test_RequestIDPolicy_AllBytes(t *testing.T) {
	t.Parallel()
	for n := 1; n <= 33; n++ {
		for pos := range n {
			for value := range 256 {
				input := []byte(strings.Repeat("a", n))
				input[pos] = byte(value)
				want := value >= 0x20 && value <= 0x7e
				if got := isValidRequestID(string(input)); got != want {
					t.Fatalf("n=%d pos=%d byte=%02x: got %v, want %v", n, pos, value, got, want)
				}
			}
		}
	}
}
