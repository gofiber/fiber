package crosshost

import (
	"testing"

	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_NewHeaderNames(t *testing.T) {
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
			require.Equal(t, tt.want, NewHeaderNames(tt.in).List())
		})
	}

	t.Run("the list is independent of its input and of what List returns", func(t *testing.T) {
		t.Parallel()
		in := []string{"X-Api-Key", "X-Auth-Token"}
		names := NewHeaderNames(in)

		in[0] = "changed-in"
		require.Equal(t, []string{"X-Api-Key", "X-Auth-Token"}, names.List())

		got := names.List()
		got[1] = "changed-out"
		require.Equal(t, []string{"X-Api-Key", "X-Auth-Token"}, names.List())
	})

	t.Run("the zero value is the empty list", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, HeaderNames{}.List())
	})
}

func Test_HeaderNames_Join(t *testing.T) {
	t.Parallel()
	one := NewHeaderNames([]string{"X-One"})
	two := NewHeaderNames([]string{"X-Two", "X-Three"})

	t.Run("one list after the other", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, []string{"X-One", "X-Two", "X-Three"}, one.Join(two).List())
		require.Equal(t, []string{"X-Two", "X-Three", "X-One"}, two.Join(one).List())
	})

	t.Run("an empty side leaves the other as it is", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, []string{"X-One"}, one.Join(HeaderNames{}).List())
		require.Equal(t, []string{"X-One"}, HeaderNames{}.Join(one).List())
		require.Nil(t, HeaderNames{}.Join(HeaderNames{}).List())
	})

	t.Run("the result shares nothing with either side", func(t *testing.T) {
		t.Parallel()
		joined := one.Join(two)
		joined.names[0], joined.names[1] = "changed", "changed"

		require.Equal(t, []string{"X-One"}, one.List())
		require.Equal(t, []string{"X-Two", "X-Three"}, two.List())
	})
}

func headerWithSensitiveAndOthers() *fasthttp.RequestHeader {
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
		h := headerWithSensitiveAndOthers()
		for _, name := range SensitiveHeaders {
			require.NotEmpty(t, fieldname.Lines(h, name, false), name)
		}

		Strip(h, HeaderNames{})

		for _, name := range SensitiveHeaders {
			require.Empty(t, fieldname.Lines(h, name, false), name)
		}
		require.Len(t, fieldname.Lines(h, "X-Api-Key", false), 1)
		require.Len(t, fieldname.Lines(h, "X-Auth-Token", false), 1)
		require.Len(t, fieldname.Lines(h, fasthttp.HeaderAccept, false), 1)
	})

	t.Run("built-in set and extra names", func(t *testing.T) {
		t.Parallel()
		h := headerWithSensitiveAndOthers()

		// Repeated, and already built in.
		Strip(h, NewHeaderNames([]string{"authorization", "X-Api-Key", "X-Api-Key"}))

		for _, name := range SensitiveHeaders {
			require.Empty(t, fieldname.Lines(h, name, false), name)
		}
		require.Empty(t, fieldname.Lines(h, "X-Api-Key", false))
		require.Len(t, fieldname.Lines(h, "X-Auth-Token", false), 1)
		require.Len(t, fieldname.Lines(h, fasthttp.HeaderAccept, false), 1)
	})

	t.Run("every extra name, whatever its case", func(t *testing.T) {
		t.Parallel()
		h := headerWithSensitiveAndOthers()

		Strip(h, NewHeaderNames([]string{"x-API-key", "X-AUTH-token"}))

		require.Empty(t, fieldname.Lines(h, "X-Api-Key", false))
		require.Empty(t, fieldname.Lines(h, "X-Auth-Token", false))
		require.Len(t, fieldname.Lines(h, fasthttp.HeaderAccept, false), 1)
	})

	t.Run("whatever case they are stored under", func(t *testing.T) {
		t.Parallel()
		h := &fasthttp.RequestHeader{}
		h.DisableNormalizing()
		h.Add("authorization", "a")
		h.Add("Authorization", "b")
		h.Add("proxy-authorization", "c")
		h.Add("x-api-key", "k1")
		h.Add("X-API-KEY", "k2")
		h.Add("X-Auth-Token", "tok")

		// Control: a byte-exact lookup would miss these.
		require.Len(t, fieldname.Lines(h, "Authorization", false), 2)
		require.Len(t, fieldname.Lines(h, "Proxy-Authorization", false), 1)
		require.Len(t, fieldname.Lines(h, "X-Api-Key", false), 2)

		Strip(h, NewHeaderNames([]string{"X-Api-Key"}))

		require.Empty(t, fieldname.Lines(h, "Authorization", false))
		require.Empty(t, fieldname.Lines(h, "Proxy-Authorization", false))
		require.Empty(t, fieldname.Lines(h, "X-Api-Key", false))
		require.Len(t, fieldname.Lines(h, "X-Auth-Token", false), 1)
	})
}
