package etag

import (
	"bytes"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"

	"github.com/valyala/fasthttp"
)

// go test -run Test_ETag_Next
func Test_ETag_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -run Test_ETag_SkipError
func Test_ETag_SkipError(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return fiber.ErrForbidden
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusForbidden, resp.StatusCode)
}

// go test -run Test_ETag_NotStatusOK
func Test_ETag_NotStatusOK(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusCreated)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusCreated, resp.StatusCode)
}

// go test -run Test_ETag_NoBody
func Test_ETag_NoBody(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
}

// go test -run Test_ETag_NewEtag
func Test_ETag_NewEtag(t *testing.T) {
	t.Parallel()
	t.Run("without HeaderIfNoneMatch", func(t *testing.T) {
		t.Parallel()
		testETagNewEtag(t, false, false)
	})
	t.Run("with HeaderIfNoneMatch and not matched", func(t *testing.T) {
		t.Parallel()
		testETagNewEtag(t, true, false)
	})
	t.Run("with HeaderIfNoneMatch and matched", func(t *testing.T) {
		t.Parallel()
		testETagNewEtag(t, true, true)
	})
}

func testETagNewEtag(t *testing.T, headerIfNoneMatch, matched bool) { //nolint:revive // We're in a test, so using bools as a flow-control is fine
	t.Helper()

	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	if headerIfNoneMatch {
		etag := `"non-match"`
		if matched {
			etag = `"13-1831710635"`
		}
		req.Header.Set(fiber.HeaderIfNoneMatch, etag)
	}

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	if !headerIfNoneMatch || !matched {
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
		utils.AssertEqual(t, `"13-1831710635"`, resp.Header.Get(fiber.HeaderETag))
		return
	}

	if matched {
		utils.AssertEqual(t, fiber.StatusNotModified, resp.StatusCode)
		b, err := io.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 0, len(b))
	}
}

// go test -run Test_ETag_WeakEtag
func Test_ETag_WeakEtag(t *testing.T) {
	t.Parallel()
	t.Run("without HeaderIfNoneMatch", func(t *testing.T) {
		t.Parallel()
		testETagWeakEtag(t, false, false)
	})
	t.Run("with HeaderIfNoneMatch and not matched", func(t *testing.T) {
		t.Parallel()
		testETagWeakEtag(t, true, false)
	})
	t.Run("with HeaderIfNoneMatch and matched", func(t *testing.T) {
		t.Parallel()
		testETagWeakEtag(t, true, true)
	})
}

func testETagWeakEtag(t *testing.T, headerIfNoneMatch, matched bool) { //nolint:revive // We're in a test, so using bools as a flow-control is fine
	t.Helper()

	app := fiber.New()

	app.Use(New(Config{Weak: true}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	if headerIfNoneMatch {
		etag := `W/"non-match"`
		if matched {
			etag = `W/"13-1831710635"`
		}
		req.Header.Set(fiber.HeaderIfNoneMatch, etag)
	}

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	if !headerIfNoneMatch || !matched {
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
		utils.AssertEqual(t, `W/"13-1831710635"`, resp.Header.Get(fiber.HeaderETag))
		return
	}

	if matched {
		utils.AssertEqual(t, fiber.StatusNotModified, resp.StatusCode)
		b, err := io.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 0, len(b))
	}
}

// go test -run Test_ETag_CustomEtag
func Test_ETag_CustomEtag(t *testing.T) {
	t.Parallel()
	t.Run("without HeaderIfNoneMatch", func(t *testing.T) {
		t.Parallel()
		testETagCustomEtag(t, false, false)
	})
	t.Run("with HeaderIfNoneMatch and not matched", func(t *testing.T) {
		t.Parallel()
		testETagCustomEtag(t, true, false)
	})
	t.Run("with HeaderIfNoneMatch and matched", func(t *testing.T) {
		t.Parallel()
		testETagCustomEtag(t, true, true)
	})
}

func testETagCustomEtag(t *testing.T, headerIfNoneMatch, matched bool) { //nolint:revive // We're in a test, so using bools as a flow-control is fine
	t.Helper()

	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderETag, `"custom"`)
		if bytes.Equal(c.Request().Header.Peek(fiber.HeaderIfNoneMatch), []byte(`"custom"`)) {
			return c.SendStatus(fiber.StatusNotModified)
		}
		return c.SendString("Hello, World!")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	if headerIfNoneMatch {
		etag := `"non-match"`
		if matched {
			etag = `"custom"`
		}
		req.Header.Set(fiber.HeaderIfNoneMatch, etag)
	}

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	if !headerIfNoneMatch || !matched {
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
		utils.AssertEqual(t, `"custom"`, resp.Header.Get(fiber.HeaderETag))
		return
	}

	if matched {
		utils.AssertEqual(t, fiber.StatusNotModified, resp.StatusCode)
		b, err := io.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 0, len(b))
	}
}

// go test -run Test_ETag_CustomEtagPut
func Test_ETag_CustomEtagPut(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Put("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderETag, `"custom"`)
		if !bytes.Equal(c.Request().Header.Peek(fiber.HeaderIfMatch), []byte(`"custom"`)) {
			return c.SendStatus(fiber.StatusPreconditionFailed)
		}
		return c.SendString("Hello, World!")
	})

	req := httptest.NewRequest(fiber.MethodPut, "/", nil)
	req.Header.Set(fiber.HeaderIfMatch, `"non-match"`)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusPreconditionFailed, resp.StatusCode)
}

// go test -v -run=^$ -bench=Benchmark_Etag -benchmem -count=4
func Benchmark_Etag(b *testing.B) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}

	utils.AssertEqual(b, 200, fctx.Response.Header.StatusCode())
	utils.AssertEqual(b, `"13-1831710635"`, string(fctx.Response.Header.Peek(fiber.HeaderETag)))
}
