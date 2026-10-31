package mediatype

import (
	"bytes"
	"iter"
	"testing"

	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Keep a scalar state machine as an oracle for the optimized separator scan.
// Only the first '=' starts a value, including on malformed input.
func referenceParamEnd(params []byte) int {
	inQuotes, escaped, expectValue, seenEq := false, false, false, false
	for i, c := range params {
		switch {
		case escaped:
			escaped = false
		case inQuotes:
			switch c {
			case '\\':
				escaped = true
			case '"':
				inQuotes = false
			}
		case c == '=' && !seenEq:
			expectValue, seenEq = true, true
		case c == '"' && expectValue:
			inQuotes, expectValue = true, false
		case c == ';':
			return i
		case c != ' ' && c != '\t':
			expectValue = false
		}
	}
	return -1
}

func referenceParams(ct []byte) iter.Seq2[[]byte, []byte] {
	return func(yield func(name, value []byte) bool) {
		_, params, ok := utils.CutByte(ct, ';')
		if !ok {
			return
		}
		for len(params) > 0 {
			param := params
			if end := referenceParamEnd(params); end >= 0 {
				param, params = params[:end], params[end+1:]
			} else {
				params = nil
			}
			name, value, ok := utils.CutByte(param, '=')
			if ok && !yield(utils.TrimSpace(name), utils.TrimSpace(value)) {
				return
			}
		}
	}
}

func FuzzParams(f *testing.F) {
	for _, seed := range []string{
		"", "application/json", "text/plain;;;;", "text/plain; charset=utf-8",
		`text/plain; a="x\";\";c=bad"; c=ok`,
		`text/plain; a="x""; c=bad"; c=ok`,
		`text/plain; a="\";c=bad"; c=ok`,
		`text/plain; a="; c=bad"; c=ok`,
		`text/plain; a=b="; charset=bad"; charset=utf-8`,
		`text/plain; a= = "; charset=bad"; charset=utf-8`,
		`text/plain; a="x"="; charset=bad"; charset=utf-8`,
		`;"0=";00000000`,
		"text/plain; a=\t\"x;y\"; b=2", "text/plain; a=\x00; b=\xff",
		`text/plain; a="x\"; end\`,
	} {
		for _, stop := range []uint8{0, 1, 255} {
			f.Add([]byte(seed), stop)
		}
	}
	f.Fuzz(func(t *testing.T, ct []byte, stop uint8) {
		t.Parallel()
		collect := func(seq iter.Seq2[[]byte, []byte]) [][2][]byte {
			var out [][2][]byte
			for name, value := range seq {
				out = append(out, [2][]byte{name, value})
				if len(out) > int(stop) {
					break
				}
			}
			return out
		}
		before := bytes.Clone(ct)
		want := collect(referenceParams(ct))
		seq := Params(ct)
		for range 2 {
			got := collect(seq)
			require.Equal(t, want, got)
			for i := range want {
				for j := range want[i] {
					require.Equal(t, cap(want[i][j]), cap(got[i][j]))
					if len(want[i][j]) > 0 {
						require.Same(t, &want[i][j][0], &got[i][j][0])
					}
				}
			}
		}
		require.Equal(t, before, ct)
	})
}

func FuzzNormalizeRequestContentType(f *testing.F) {
	for _, seed := range []string{
		"", "Application/JSON", "Text/Plain; Flag;; CHARSET=UTF-8",
		`Multipart/Form-Data; X= "; BOUNDARY=bogus"; BOUNDARY=Real`,
		`Multipart/Form-Data; X="\"; BOUNDARY=bogus"; BOUNDARY=Real`,
		`Text/Plain; X=ab="CD; Y=Z`, `Text/Plain; X="a\\"; Y=Z`,
		"Text/Plain; X=\t\"A;B\"; Y=Z", `Text/Plain; X= "A;B`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		t.Parallel()
		var h fasthttp.RequestHeader
		h.SetContentType(input)
		want := bytes.Clone(h.ContentType())
		mediaType, params, _ := bytes.Cut(want, []byte{';'})
		foldASCII(mediaType)
		for len(params) > 0 {
			param := params
			if end := referenceParamEnd(params); end >= 0 {
				param, params = params[:end], params[end+1:]
			} else {
				params = nil
			}
			name, _, _ := bytes.Cut(param, []byte{'='})
			foldASCII(name)
		}
		require.Equal(t, want, NormalizeRequestContentType(&h))
		require.Equal(t, want, h.ContentType())
		require.Equal(t, want, NormalizeRequestContentType(&h))
	})
}

func foldASCII(b []byte) {
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
}
