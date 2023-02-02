package encryptcookie

import (
	"encoding/base64"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"

	"github.com/valyala/fasthttp"
)

var testKey = GenerateKey()

func Test_Middleware_Encrypt_Cookie(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("value=" + c.Cookies("test"))
	})
	app.Post("/", func(c *fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "SomeThing",
		})
		return nil
	})

	h := app.Handler()

	// Test empty cookie
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())
	utils.AssertEqual(t, "value=", string(ctx.Response.Body()))

	// Test invalid cookie
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("test", "Invalid")
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())
	utils.AssertEqual(t, "value=", string(ctx.Response.Body()))
	ctx.Request.Header.SetCookie("test", "ixQURE2XOyZUs0WAOh2ehjWcP7oZb07JvnhWOsmeNUhPsj4+RyI=")
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())
	utils.AssertEqual(t, "value=", string(ctx.Response.Body()))

	// Test valid cookie
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())

	encryptedCookie := fasthttp.Cookie{}
	encryptedCookie.SetKey("test")
	utils.AssertEqual(t, true, ctx.Response.Header.Cookie(&encryptedCookie), "Get cookie value")
	decryptedCookieValue, err := DecryptCookie(string(encryptedCookie.Value()), testKey)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "SomeThing", decryptedCookieValue)

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("test", string(encryptedCookie.Value()))
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())
	utils.AssertEqual(t, "value=SomeThing", string(ctx.Response.Body()))
}

func Test_Encrypt_Cookie_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "SomeThing",
		})
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "SomeThing", resp.Cookies()[0].Value)
}

func Test_Encrypt_Cookie_Except(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
		Except: []string{
			"test1",
		},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test1",
			Value: "SomeThing",
		})
		c.Cookie(&fiber.Cookie{
			Name:  "test2",
			Value: "SomeThing",
		})

		return nil
	})

	h := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())

	rawCookie := fasthttp.Cookie{}
	rawCookie.SetKey("test1")
	utils.AssertEqual(t, true, ctx.Response.Header.Cookie(&rawCookie), "Get cookie value")
	utils.AssertEqual(t, "SomeThing", string(rawCookie.Value()))

	encryptedCookie := fasthttp.Cookie{}
	encryptedCookie.SetKey("test2")
	utils.AssertEqual(t, true, ctx.Response.Header.Cookie(&encryptedCookie), "Get cookie value")
	decryptedCookieValue, err := DecryptCookie(string(encryptedCookie.Value()), testKey)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "SomeThing", decryptedCookieValue)
}

func Test_Encrypt_Cookie_Custom_Encryptor(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
		Encryptor: func(decryptedString, _ string) (string, error) {
			return base64.StdEncoding.EncodeToString([]byte(decryptedString)), nil
		},
		Decryptor: func(encryptedString, _ string) (string, error) {
			decodedBytes, err := base64.StdEncoding.DecodeString(encryptedString)
			return string(decodedBytes), err
		},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("value=" + c.Cookies("test"))
	})
	app.Post("/", func(c *fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "SomeThing",
		})

		return nil
	})

	h := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())

	encryptedCookie := fasthttp.Cookie{}
	encryptedCookie.SetKey("test")
	utils.AssertEqual(t, true, ctx.Response.Header.Cookie(&encryptedCookie), "Get cookie value")
	decodedBytes, err := base64.StdEncoding.DecodeString(string(encryptedCookie.Value()))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "SomeThing", string(decodedBytes))

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("test", string(encryptedCookie.Value()))
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())
	utils.AssertEqual(t, "value=SomeThing", string(ctx.Response.Body()))
}
