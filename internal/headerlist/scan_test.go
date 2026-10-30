package headerlist

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_NextPrev(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		list string
		want []string
	}{
		{name: "empty", list: "", want: []string{""}},
		{name: "single", list: "203.0.113.50", want: []string{"203.0.113.50"}},
		{name: "empty elements", list: ",a,,,b,", want: []string{"", "a", "", "", "b", ""}},
		{name: "only separators", list: ",,", want: []string{"", "", ""}},
		{name: "raw whitespace", list: " \ta\t,\vb\r\n, ", want: []string{" \ta\t", "\vb\r\n", " "}},
		{name: "quotes are literal", list: `"a,b",c`, want: []string{`"a`, `b"`, "c"}},
		{name: "raw bytes", list: "\x00\xff,é,::ffff:192.0.2.1", want: []string{"\x00\xff", "é", "::ffff:192.0.2.1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var forward, backward []string
			for start := 0; start <= len(tc.list); {
				element, next := Next(tc.list, start)
				require.Greater(t, next, start)
				forward = append(forward, element)
				start = next
			}
			for end := len(tc.list); end >= 0; {
				element, prev := Prev(tc.list, end)
				require.Less(t, prev, end)
				backward = append(backward, element)
				end = prev
			}
			require.Equal(t, tc.want, forward)
			slices.Reverse(backward)
			require.Equal(t, tc.want, backward)
		})
	}
}
