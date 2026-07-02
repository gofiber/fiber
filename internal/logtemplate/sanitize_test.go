package logtemplate

import (
	"testing"

	"github.com/valyala/bytebufferpool"
)

func TestIsControlByte(t *testing.T) {
	t.Parallel()

	controls := []byte{0x00, '\n', '\r', 0x1b, 0x1f, 0x7f}
	for _, b := range controls {
		if !IsControlByte(b) {
			t.Errorf("IsControlByte(%#x) = false, want true", b)
		}
	}

	allowed := []byte{'\t', ' ', 'a', 'Z', '0', '~', 0x80}
	for _, b := range allowed {
		if IsControlByte(b) {
			t.Errorf("IsControlByte(%#x) = true, want false", b)
		}
	}
}

func TestWriteSanitizedString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "clean", in: "GET /users", want: "GET /users"},
		{name: "tab preserved", in: "a\tb", want: "a\tb"},
		{name: "crlf injection", in: "/admin\r\n200 GET /ok", want: "/admin  200 GET /ok"},
		{name: "null and escape", in: "x\x00\x1by", want: "x  y"},
		{name: "del byte", in: "a\x7fb", want: "a b"},
		{name: "non-ascii untouched", in: "café", want: "café"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			buf := bytebufferpool.Get()
			defer bytebufferpool.Put(buf)

			n, err := WriteSanitizedString(buf, tc.in)
			if err != nil {
				t.Fatalf("WriteSanitizedString returned error: %v", err)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("WriteSanitizedString(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if n != len(tc.want) {
				t.Errorf("WriteSanitizedString(%q) wrote %d bytes, want %d", tc.in, n, len(tc.want))
			}
		})
	}
}

func TestWriteSanitizedBytes(t *testing.T) {
	t.Parallel()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	in := []byte("line1\r\nline2")
	want := "line1  line2"

	n, err := WriteSanitized(buf, in)
	if err != nil {
		t.Fatalf("WriteSanitized returned error: %v", err)
	}
	if got := buf.String(); got != want {
		t.Errorf("WriteSanitized(%q) = %q, want %q", in, got, want)
	}
	if n != len(want) {
		t.Errorf("WriteSanitized wrote %d bytes, want %d", n, len(want))
	}
}
