package etag

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// go test -run Test_ETag_Next
func Test_ETag_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -run Test_ETag_SkipError
func Test_ETag_SkipError(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(_ fiber.Ctx) error {
		return fiber.ErrForbidden
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

// go test -run Test_ETag_NotStatusOK
func Test_ETag_NotStatusOK(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusCreated)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusCreated, resp.StatusCode)
}

// go test -run Test_ETag_NoBody
func Test_ETag_NoBody(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(_ fiber.Ctx) error {
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// go test -run Test_ETag_NewEtag
func Test_ETag_NewEtag(t *testing.T) {
	t.Parallel()
	t.Run("without HeaderIfNoneMatch", func(t *testing.T) {
		t.Parallel()
		testETagNewEtag(t, "", fiber.StatusOK)
	})
	t.Run("with HeaderIfNoneMatch and not matched", func(t *testing.T) {
		t.Parallel()
		testETagNewEtag(t, `"non-match"`, fiber.StatusOK)
	})
	t.Run("with HeaderIfNoneMatch and matched", func(t *testing.T) {
		t.Parallel()
		testETagNewEtag(t, `"13-1831710635"`, fiber.StatusNotModified)
	})
}

func testETagNewEtag(t *testing.T, headerIfNoneMatch string, expectedStatus int) {
	t.Helper()

	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	if headerIfNoneMatch != "" {
		req.Header.Set(fiber.HeaderIfNoneMatch, headerIfNoneMatch)
	}

	resp, err := app.Test(req)
	require.NoError(t, err)

	if expectedStatus == fiber.StatusOK {
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		require.Equal(t, `"13-1831710635"`, resp.Header.Get(fiber.HeaderETag))
		return
	}

	require.Equal(t, fiber.StatusNotModified, resp.StatusCode)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Empty(t, b)
}

// go test -run Test_ETag_WeakEtag
func Test_ETag_WeakEtag(t *testing.T) {
	t.Parallel()
	t.Run("without HeaderIfNoneMatch", func(t *testing.T) {
		t.Parallel()
		testETagWeakEtag(t, "", fiber.StatusOK)
	})
	t.Run("with HeaderIfNoneMatch and not matched", func(t *testing.T) {
		t.Parallel()
		testETagWeakEtag(t, `W/"non-match"`, fiber.StatusOK)
	})
	t.Run("with HeaderIfNoneMatch and matched", func(t *testing.T) {
		t.Parallel()
		testETagWeakEtag(t, `W/"13-1831710635"`, fiber.StatusNotModified)
	})
}

func testETagWeakEtag(t *testing.T, headerIfNoneMatch string, expectedStatus int) {
	t.Helper()

	app := fiber.New()

	app.Use(New(Config{Weak: true}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	if headerIfNoneMatch != "" {
		req.Header.Set(fiber.HeaderIfNoneMatch, headerIfNoneMatch)
	}

	resp, err := app.Test(req)
	require.NoError(t, err)

	if expectedStatus == fiber.StatusOK {
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		require.Equal(t, `W/"13-1831710635"`, resp.Header.Get(fiber.HeaderETag))
		return
	}

	require.Equal(t, fiber.StatusNotModified, resp.StatusCode)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Empty(t, b)
}

// go test -run Test_ETag_CustomEtag
func Test_ETag_CustomEtag(t *testing.T) {
	t.Parallel()
	t.Run("without HeaderIfNoneMatch", func(t *testing.T) {
		t.Parallel()
		testETagCustomEtag(t, "", fiber.StatusOK)
	})
	t.Run("with HeaderIfNoneMatch and not matched", func(t *testing.T) {
		t.Parallel()
		testETagCustomEtag(t, `"non-match"`, fiber.StatusOK)
	})
	t.Run("with HeaderIfNoneMatch and matched", func(t *testing.T) {
		t.Parallel()
		testETagCustomEtag(t, `"custom"`, fiber.StatusNotModified)
	})
}

func testETagCustomEtag(t *testing.T, headerIfNoneMatch string, expectedStatus int) {
	t.Helper()

	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderETag, `"custom"`)
		if bytes.Equal(c.Request().Header.Peek(fiber.HeaderIfNoneMatch), []byte(`"custom"`)) {
			return c.SendStatus(fiber.StatusNotModified)
		}
		return c.SendString("Hello, World!")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	if headerIfNoneMatch != "" {
		req.Header.Set(fiber.HeaderIfNoneMatch, headerIfNoneMatch)
	}

	resp, err := app.Test(req)
	require.NoError(t, err)

	if expectedStatus == fiber.StatusOK {
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		require.Equal(t, `"custom"`, resp.Header.Get(fiber.HeaderETag))
		return
	}

	require.Equal(t, fiber.StatusNotModified, resp.StatusCode)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Empty(t, b)
}

// go test -run Test_ETag_CustomEtagPut
func Test_ETag_CustomEtagPut(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Put("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderETag, `"custom"`)
		if !bytes.Equal(c.Request().Header.Peek(fiber.HeaderIfMatch), []byte(`"custom"`)) {
			return c.SendStatus(fiber.StatusPreconditionFailed)
		}
		return c.SendString("Hello, World!")
	})

	req := httptest.NewRequest(fiber.MethodPut, "/", http.NoBody)
	req.Header.Set(fiber.HeaderIfMatch, `"non-match"`)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusPreconditionFailed, resp.StatusCode)
}

// go test -v -run=^$ -bench=Benchmark_Etag -benchmem -count=4
func Benchmark_Etag(b *testing.B) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, 200, fctx.Response.Header.StatusCode())
	require.Equal(b, `"13-1831710635"`, string(fctx.Response.Header.Peek(fiber.HeaderETag)))
}

// go test -run Test_ETag_WeakComparison
func Test_ETag_WeakComparison(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		ifNoneMatch    string
		weak           bool
		expectedStatus int
	}{
		{name: "weak client tag matches strong server tag", ifNoneMatch: `W/"13-1831710635"`, weak: false, expectedStatus: fiber.StatusNotModified},
		{name: "strong client tag matches weak server tag", ifNoneMatch: `"13-1831710635"`, weak: true, expectedStatus: fiber.StatusNotModified},
		{name: "match in list after weak tag", ifNoneMatch: `W/"non-match", "13-1831710635"`, weak: false, expectedStatus: fiber.StatusNotModified},
		{name: "weak match in list", ifNoneMatch: `"non-match", W/"13-1831710635"`, weak: false, expectedStatus: fiber.StatusNotModified},
		{name: "wildcard", ifNoneMatch: `*`, weak: false, expectedStatus: fiber.StatusNotModified},
		{name: "no match in list", ifNoneMatch: `W/"non-match", "other"`, weak: false, expectedStatus: fiber.StatusOK},
		{name: "unquoted tag never matches", ifNoneMatch: `13-1831710635`, weak: false, expectedStatus: fiber.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Weak: tc.weak}))
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("Hello, World!")
			})

			req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			req.Header.Set(fiber.HeaderIfNoneMatch, tc.ifNoneMatch)

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tc.expectedStatus, resp.StatusCode)

			expectedTag := `"13-1831710635"`
			if tc.weak {
				expectedTag = `W/"13-1831710635"`
			}
			require.Equal(t, expectedTag, resp.Header.Get(fiber.HeaderETag))

			if tc.expectedStatus == fiber.StatusNotModified {
				b, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Empty(t, b)
			}
		})
	}
}

// go test -run Test_ETag_etagWeakMatch
func Test_ETag_etagWeakMatch(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{name: "identical strong tags", a: `"abc"`, b: `"abc"`, expected: true},
		{name: "weak client vs strong server", a: `W/"abc"`, b: `"abc"`, expected: true},
		{name: "strong client vs weak server", a: `"abc"`, b: `W/"abc"`, expected: true},
		{name: "both weak", a: `W/"abc"`, b: `W/"abc"`, expected: true},
		{name: "different values", a: `"abc"`, b: `"def"`, expected: false},
		{name: "unquoted client tag", a: `abc`, b: `"abc"`, expected: false},
		{name: "unquoted server tag", a: `"abc"`, b: `abc`, expected: false},
		{name: "empty client tag", a: ``, b: `"abc"`, expected: false},
		{name: "empty server tag", a: `"abc"`, b: ``, expected: false},
		{name: "weak prefix only", a: `W/`, b: `"abc"`, expected: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, etagWeakMatch([]byte(tc.a), []byte(tc.b)))
		})
	}
}
