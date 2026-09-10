package headerlookup

import (
	"bufio"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/appconfig"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// newCtx returns a context on an app configured with cfg, released when the
// test ends.
func newCtx(t *testing.T, cfg ...fiber.Config) fiber.Ctx {
	t.Helper()

	app := fiber.New(cfg...)
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })
	if len(cfg) > 0 && cfg[0].DisableHeaderNormalizing {
		c.Request().Header.DisableNormalizing()
	}
	return c
}

// readRequest parses raw into c's request, which is the only way to build a
// message carrying a field the way a peer would rather than the way Set does.
func readRequest(t *testing.T, c fiber.Ctx, raw string) {
	t.Helper()

	require.NoError(t, c.Request().Header.Read(bufio.NewReader(strings.NewReader(raw))))
}

func Test_Canonical(t *testing.T) {
	t.Parallel()

	require.True(t, Canonical(newCtx(t)))
	require.False(t, Canonical(newCtx(t, fiber.Config{DisableHeaderNormalizing: true})))
}

func Test_Hot(t *testing.T) {
	t.Parallel()

	t.Run("reads the app", func(t *testing.T) {
		t.Parallel()

		c := newCtx(t, fiber.Config{Immutable: true, DisableHeaderNormalizing: true, UnescapePath: true})
		require.Equal(t, appconfig.Hot{Immutable: true, DisableHeaderNormalizing: true, UnescapePath: true}, hot(c))
		require.Equal(t, appconfig.Hot{}, hot(newCtx(t)))
	})

	t.Run("fallback matches the app read", func(t *testing.T) {
		t.Parallel()

		// The by-value path answers the same as the direct read, so a caller
		// reaching it is not told anything different.
		cfg := fiber.Config{Immutable: true, UnescapePath: true}
		app := fiber.New(cfg)
		live := app.Config()
		require.Equal(t, hotFromConfig(&live), hot(newCtx(t, cfg)))
	})
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

		// Same length, so fasthttp may write it over the bytes just read.
		c.Request().Header.Set("X-Token", "xyz")
		require.Equal(t, "abc", v, "Immutable promises a value that outlives the request's buffer")
	})
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

		c := newCtx(t)
		require.Empty(t, Combined(c, "X-API-Key"))
		c.Request().Header.Set("X-API-Key", "")
		require.Empty(t, Combined(c, "X-API-Key"))
	})

	t.Run("repeated lines are one value", func(t *testing.T) {
		t.Parallel()

		for _, normalize := range []bool{true, false} {
			c := newCtx(t, fiber.Config{DisableHeaderNormalizing: !normalize})
			readRequest(t, c, "GET / HTTP/1.1\r\nHost: e.com\r\nAccept: text/html\r\naccept: application/json\r\n\r\n")
			require.Equal(t, "text/html, application/json", Combined(c, "Accept"), "normalize=%v", normalize)
		}
	})

	t.Run("an empty line among repeated ones keeps its place", func(t *testing.T) {
		t.Parallel()

		// Both halves of Combined join every line they were sent, so a line
		// that is present and empty is a value of nothing rather than a line
		// that was not there.
		for _, normalize := range []bool{true, false} {
			c := newCtx(t, fiber.Config{DisableHeaderNormalizing: !normalize})
			readRequest(t, c, "GET / HTTP/1.1\r\nHost: e.com\r\nAccept:\r\naccept: application/json\r\n\r\n")
			require.Equal(t, ", application/json", Combined(c, "Accept"), "normalize=%v", normalize)
		}
	})

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
}

// Test_Combined_DoesNotAllocate pins the fold path, which collected its matches
// through a callback into a slice and so allocated three times for every header
// read by an application that keeps the spelling its peers sent.
//
// Deliberately not parallel, and top level rather than a subtest: AllocsPerRun
// counts allocations process-wide and refuses to run under a parallel parent.
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
