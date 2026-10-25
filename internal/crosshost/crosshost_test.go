package crosshost

import (
	"testing"

	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_NormalizeNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "nil", in: nil, want: nil},
		{name: "empty", in: []string{}, want: nil},
		{name: "only blanks", in: []string{"", " ", "\t"}, want: nil},
		{name: "kept as given", in: []string{"X-Api-Key", "x-auth-token"}, want: []string{"X-Api-Key", "x-auth-token"}},
		{
			name: "trimmed",
			in:   []string{" X-Api-Key", "X-Auth-Token\t", "  X-Access-Token  "},
			want: []string{"X-Api-Key", "X-Auth-Token", "X-Access-Token"},
		},
		{name: "blanks dropped between names", in: []string{"X-Api-Key", " ", "", "X-Auth-Token"}, want: []string{"X-Api-Key", "X-Auth-Token"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Equal tells nil from empty.
			require.Equal(t, tt.want, NormalizeNames(tt.in))
		})
	}

	t.Run("input and output are independent", func(t *testing.T) {
		t.Parallel()

		in := []string{"X-Api-Key", "X-Auth-Token"}
		out := NormalizeNames(in)

		in[0] = "changed-in"
		require.Equal(t, []string{"X-Api-Key", "X-Auth-Token"}, out)

		out[1] = "changed-out"
		require.Equal(t, []string{"changed-in", "X-Auth-Token"}, in)
	})
}

// requestWith holds every built-in sensitive header and three others.
func requestWith() *fasthttp.RequestHeader {
	h := &fasthttp.RequestHeader{}
	for _, name := range SensitiveHeaders {
		h.Set(name, "k=v")
	}
	h.Set("X-Api-Key", "key")
	h.Set("X-Auth-Token", "tok")
	h.Set(fasthttp.HeaderAccept, "*/*")
	return h
}

func Test_Strip(t *testing.T) {
	t.Parallel()

	t.Run("built-in set", func(t *testing.T) {
		t.Parallel()

		h := requestWith()
		for _, name := range SensitiveHeaders {
			require.NotEmpty(t, fieldname.Lines(h, name, false), name)
		}

		Strip(h, nil)

		for _, name := range SensitiveHeaders {
			require.Empty(t, fieldname.Lines(h, name, false), name)
		}
		require.Len(t, fieldname.Lines(h, "X-Api-Key", false), 1)
		require.Len(t, fieldname.Lines(h, "X-Auth-Token", false), 1)
		require.Len(t, fieldname.Lines(h, fasthttp.HeaderAccept, false), 1)
	})

	t.Run("built-in set and extra names", func(t *testing.T) {
		t.Parallel()

		h := requestWith()

		// An extra name may repeat, or already be built in.
		Strip(h, []string{"authorization", "X-Api-Key", "X-Api-Key"})

		for _, name := range SensitiveHeaders {
			require.Empty(t, fieldname.Lines(h, name, false), name)
		}
		require.Empty(t, fieldname.Lines(h, "X-Api-Key", false))
		require.Len(t, fieldname.Lines(h, "X-Auth-Token", false), 1)
		require.Len(t, fieldname.Lines(h, fasthttp.HeaderAccept, false), 1)
	})

	t.Run("whatever case they are stored under", func(t *testing.T) {
		t.Parallel()

		// Keeps spellings as given, like an HTTP/2 peer's headers.
		h := &fasthttp.RequestHeader{}
		h.DisableNormalizing()
		h.Add("authorization", "a")
		h.Add("Authorization", "b")
		h.Add("proxy-authorization", "c")
		h.Add("x-api-key", "k1")
		h.Add("X-API-KEY", "k2")
		h.Add("X-Auth-Token", "tok")

		// Control: a byte-exact lookup would miss most of these.
		require.Len(t, fieldname.Lines(h, "Authorization", false), 2)
		require.Len(t, fieldname.Lines(h, "Proxy-Authorization", false), 1)
		require.Len(t, fieldname.Lines(h, "X-Api-Key", false), 2)

		Strip(h, []string{"X-Api-Key"})

		require.Empty(t, fieldname.Lines(h, "Authorization", false))
		require.Empty(t, fieldname.Lines(h, "Proxy-Authorization", false))
		require.Empty(t, fieldname.Lines(h, "X-Api-Key", false))
		require.Len(t, fieldname.Lines(h, "X-Auth-Token", false), 1)
	})
}
