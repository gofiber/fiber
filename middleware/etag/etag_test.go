package etag

import (
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// go test -run Test_ETag_Next
func Test_ETag_Next(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -run Test_ETag_SkipError
func Test_ETag_SkipError(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return fiber.ErrForbidden
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusForbidden, resp.StatusCode)
}

// go test -run Test_ETag_NotStatusOK
func Test_ETag_NotStatusOK(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusCreated)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusCreated, resp.StatusCode)
}

// go test -run Test_ETag_NoBody
func Test_ETag_NoBody(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return nil
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
}

// go test -run Test_ETag_NewEtag
func Test_ETag_NewEtag(t *testing.T) {
	t.Run("without HeaderIfNoneMatch", func(t *testing.T) {
		testEtagNewEtag(t, false, false)
	})
	t.Run("with HeaderIfNoneMatch and not matched", func(t *testing.T) {
		testEtagNewEtag(t, true, false)
	})
	t.Run("with HeaderIfNoneMatch and matched", func(t *testing.T) {
		testEtagNewEtag(t, true, true)
	})
}

func testEtagNewEtag(t *testing.T, headerIfNoneMatch, matched bool) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	req := httptest.NewRequest("GET", "/", nil)
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
		b, err := ioutil.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 0, len(b))
	}
}

// go test -run Test_ETag_WeakEtag
func Test_ETag_WeakEtag(t *testing.T) {
	t.Run("without HeaderIfNoneMatch", func(t *testing.T) {
		testEtagWeakEtag(t, false, false)
	})
	t.Run("with HeaderIfNoneMatch and not matched", func(t *testing.T) {
		testEtagWeakEtag(t, true, false)
	})
	t.Run("with HeaderIfNoneMatch and matched", func(t *testing.T) {
		testEtagWeakEtag(t, true, true)
	})
}

func testEtagWeakEtag(t *testing.T, headerIfNoneMatch, matched bool) {
	app := fiber.New()

	app.Use(New(Config{Weak: true}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	req := httptest.NewRequest("GET", "/", nil)
	if headerIfNoneMatch {
		etag := `"non-match"`
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
		b, err := ioutil.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 0, len(b))
	}
}
