package headerlookup

import (
	"bufio"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

// newCtx returns a context on an app configured with cfg, released when the
// test ends.
func newCtx(t *testing.T, cfg ...fiber.Config) fiber.Ctx {
	t.Helper()

	c := apptest.Ctx(t, fiber.New(cfg...))
	if len(cfg) > 0 && cfg[0].DisableHeaderNormalizing {
		c.Request().Header.DisableNormalizing()
	}
	return c
}

// readRequest parses raw into c's request, the only way to build a message
// carrying a field the way a peer would rather than the way Set does.
func readRequest(t *testing.T, c fiber.Ctx, raw string) {
	t.Helper()

	require.NoError(t, c.Request().Header.Read(bufio.NewReader(strings.NewReader(raw))))
}

func Test_Canonical(t *testing.T) {
	t.Parallel()
	require.True(t, Canonical(newCtx(t)))
	require.False(t, Canonical(newCtx(t, fiber.Config{DisableHeaderNormalizing: true})))
}

func Test_Value(t *testing.T) {
	t.Parallel()
	t.Run("single line", func(t *testing.T) {
		t.Parallel()
		c := newCtx(t)
		c.Request().Header.Set(fiber.HeaderAuthorization, "Bearer abc")
		v, ok := Value(c, fiber.HeaderAuthorization)
		require.True(t, ok)
		require.Equal(t, "Bearer abc", v)
	})

	t.Run("absent is present-and-empty, not a refusal", func(t *testing.T) {
		t.Parallel()
		v, ok := Value(newCtx(t), fiber.HeaderAuthorization)
		require.True(t, ok, "absent must not read as malformed")
		require.Empty(t, v)
	})

	t.Run("a line that is present and empty", func(t *testing.T) {
		t.Parallel()
		c := newCtx(t)
		c.Request().Header.Set("X-Token", "")
		v, ok := Value(c, "X-Token")
		require.True(t, ok)
		require.Empty(t, v)
	})

	t.Run("repeated lines are refused", func(t *testing.T) {
		t.Parallel()
		c := newCtx(t)
		readRequest(t, c, "GET / HTTP/1.1\r\nHost: e.com\r\nAuthorization: Bearer a\r\nAuthorization: Bearer b\r\n\r\n")
		v, ok := Value(c, fiber.HeaderAuthorization)
		require.False(t, ok, "an ambiguous credential has no answer")
		require.Empty(t, v, "the value is empty whenever ok is false, so a caller fails closed")
	})

	t.Run("matches the name case-insensitively", func(t *testing.T) {
		t.Parallel()

		// The spelling HTTP/2 and 3 put on the wire, kept as sent.
		c := newCtx(t, fiber.Config{DisableHeaderNormalizing: true})
		readRequest(t, c, "GET / HTTP/1.1\r\nHost: e.com\r\nauthorization: Bearer abc\r\n\r\n")
		v, ok := Value(c, fiber.HeaderAuthorization)
		require.True(t, ok)
		require.Equal(t, "Bearer abc", v)
	})

	t.Run("repeated lines are refused whatever they are spelled like", func(t *testing.T) {
		t.Parallel()
		c := newCtx(t, fiber.Config{DisableHeaderNormalizing: true})
		readRequest(t, c, "GET / HTTP/1.1\r\nHost: e.com\r\nAuthorization: Bearer a\r\nauthorization: Bearer b\r\n\r\n")
		_, ok := Value(c, fiber.HeaderAuthorization)
		require.False(t, ok)
	})

	t.Run("Immutable hands back a copy", func(t *testing.T) {
		t.Parallel()
		c := newCtx(t, fiber.Config{Immutable: true})
		c.Request().Header.Set("X-Token", "abc")
		v, ok := Value(c, "X-Token")
		require.True(t, ok)
		require.Equal(t, "abc", v)

		// Same length, so fasthttp may write over the bytes just read.
		c.Request().Header.Set("X-Token", "xyz")
		require.Equal(t, "abc", v, "Immutable promises a value that outlives the request's buffer")
	})

	t.Run("a field of a held trailer section is refused", func(t *testing.T) {
		t.Parallel()

		for _, normalize := range []bool{true, false} {
			for _, headers := range []string{"X-User: alice\r\n", ""} {
				c := newCtx(t, fiber.Config{DisableHeaderNormalizing: !normalize})
				readChunked(t, c, headers)
				v, ok := Value(c, "X-User")
				require.False(t, ok, "normalize=%v %q", normalize, headers)
				require.Empty(t, v, "normalize=%v %q", normalize, headers)
			}

			c := newCtx(t, fiber.Config{DisableHeaderNormalizing: !normalize})
			readChunked(t, c, "Origin: https://app.example\r\n")
			v, ok := Value(c, "Origin")
			require.True(t, ok, "normalize=%v", normalize)
			require.Equal(t, "https://app.example", v, "normalize=%v", normalize)
		}
	})
}

// readChunked parses into c a whole chunked request with the given header
// lines and an X-User trailer field.
func readChunked(t *testing.T, c fiber.Ctx, headers string) {
	t.Helper()

	raw := "POST / HTTP/1.1\r\nHost: e.com\r\n" + headers + "Transfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\nX-User: admin\r\n\r\n"
	require.NoError(t, c.Request().Read(bufio.NewReader(strings.NewReader(raw))))
}

func Test_Combined(t *testing.T) {
	t.Parallel()
	t.Run("single line is unchanged", func(t *testing.T) {
		t.Parallel()
		c := newCtx(t)
		c.Request().Header.Set("X-API-Key", "abc123")
		require.Equal(t, "abc123", Combined(c, "X-API-Key"))
	})

	t.Run("absent and empty both read as nothing", func(t *testing.T) {
		t.Parallel()
		for _, normalize := range []bool{true, false} {
			c := newCtx(t, fiber.Config{DisableHeaderNormalizing: !normalize})
			require.Empty(t, Combined(c, "X-API-Key"), "normalize=%v", normalize)
			c.Request().Header.Set("X-API-Key", "")
			require.Empty(t, Combined(c, "X-API-Key"), "normalize=%v", normalize)
		}
	})

	for _, tc := range []struct {
		name  string
		lines string // the field lines after Host, less the CRLF ending the last
		want  string
	}{
		{"repeated lines read as one", "Accept: text/html\r\naccept: application/json", "text/html, application/json"},
		// Both halves join every line they were sent, so a present-and-empty
		// line is a value of nothing, not an absent line.
		{"an empty repeated line keeps its place", "Accept:\r\naccept: application/json", ", application/json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, normalize := range []bool{true, false} {
				c := newCtx(t, fiber.Config{DisableHeaderNormalizing: !normalize})
				readRequest(t, c, "GET / HTTP/1.1\r\nHost: e.com\r\n"+tc.lines+"\r\n\r\n")
				require.Equal(t, tc.want, Combined(c, "Accept"), "normalize=%v", normalize)
			}
		})
	}

	t.Run("Cookie keeps its own separator", func(t *testing.T) {
		t.Parallel()
		c := newCtx(t)
		readRequest(t, c, "GET / HTTP/1.1\r\nHost: e.com\r\nCookie: a=1\r\nCookie: b=2\r\n\r\n")
		require.Equal(t, "a=1; b=2", Combined(c, fiber.HeaderCookie))
	})

	t.Run("Immutable hands back a copy", func(t *testing.T) {
		t.Parallel()
		c := newCtx(t, fiber.Config{Immutable: true})
		c.Request().Header.Set("X-Token", "abc")
		v := Combined(c, "X-Token")
		c.Request().Header.Set("X-Token", "xyz")
		require.Equal(t, "abc", v)
	})

	t.Run("a field of a held trailer section is not read", func(t *testing.T) {
		t.Parallel()

		for _, normalize := range []bool{true, false} {
			for _, headers := range []string{"X-User: alice\r\n", ""} {
				c := newCtx(t, fiber.Config{DisableHeaderNormalizing: !normalize})
				readChunked(t, c, headers)
				require.Empty(t, Combined(c, "X-User"), "normalize=%v %q", normalize, headers)
			}

			c := newCtx(t, fiber.Config{DisableHeaderNormalizing: !normalize})
			readChunked(t, c, "X-Key: k\r\nX-Key: l\r\n")
			require.Equal(t, "k, l", Combined(c, "X-Key"), "normalize=%v", normalize)
		}
	})
}

// Test_Combined_DoesNotAllocate pins the fold path, which allocated three
// times per header read before it walked the store itself.
//
// Not parallel, and top level: AllocsPerRun counts allocations process-wide
// and refuses to run under a parallel parent.
func Test_Combined_DoesNotAllocate(t *testing.T) {
	for _, mode := range []struct {
		name string
		cfg  fiber.Config
		raw  string
	}{
		{name: "normalized", cfg: fiber.Config{}, raw: "GET / HTTP/1.1\r\nHost: e.com\r\nX-API-Key: abc123\r\n\r\n"},
		{name: "as sent", cfg: fiber.Config{DisableHeaderNormalizing: true}, raw: "GET / HTTP/1.1\r\nHost: e.com\r\nx-api-key: abc123\r\n\r\n"},
	} {
		c := newCtx(t, mode.cfg)
		readRequest(t, c, mode.raw)

		read := func() {
			if v := Combined(c, "X-API-Key"); v != "abc123" {
				t.Errorf("%s: got %q", mode.name, v)
			}
		}
		read() // warm anything the header store collects once

		require.Zero(t, testing.AllocsPerRun(100, read), "%s", mode.name)
	}
}
