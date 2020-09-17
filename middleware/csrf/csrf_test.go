package csrf

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

func Test_CSRF(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Post("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod("GET")
	h(ctx)
	utils.AssertEqual(t, true, strings.Contains(string(ctx.Response.Header.Peek(fiber.HeaderSetCookie)), "_csrf"))

	// Without CSRF cookie
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod("POST")
	h(ctx)
	utils.AssertEqual(t, 403, ctx.Response.StatusCode())

	// Empty/invalid CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.Set("X-CSRF-Token", "johndoe")
	h(ctx)
	utils.AssertEqual(t, 403, ctx.Response.StatusCode())

	// Valid CSRF token
	token := utils.UUID()
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.Set(fiber.HeaderCookie, "_csrf="+token)
	ctx.Request.Header.Set("X-CSRF-Token", token)
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())
}

// go test -run Test_CSRF_Next
func Test_CSRF_Next(t *testing.T) {
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

func Test_CSRF_Invalid_TokenLookup(t *testing.T) {
	defer func() {
		utils.AssertEqual(t, "csrf: Token lookup must in the form of <source>:<key>", recover())
	}()
	app := fiber.New()

	app.Use(New(Config{TokenLookup: "I:am:invalid"}))

	app.Post("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	h(ctx)
}

func Test_CSRF_From_Form(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{TokenLookup: "form:_csrf"}))

	app.Post("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Valid CSRF token
	token := utils.UUID()
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.Set(fiber.HeaderCookie, "_csrf="+token)
	ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	h(ctx)
	utils.AssertEqual(t, 403, ctx.Response.StatusCode())

	ctx.Request.Reset()
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.Set(fiber.HeaderCookie, "_csrf="+token)
	ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	ctx.Request.SetBodyString("_csrf=" + token)
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())
}

func Test_CSRF_From_Query(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{TokenLookup: "query:_csrf"}))

	app.Post("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Valid CSRF token
	token := utils.UUID()
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.Set(fiber.HeaderCookie, "_csrf="+token)
	ctx.Request.SetRequestURI("/?_csrf=" + token)
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())

	ctx.Request.SetRequestURI("/")
	ctx.Response.Reset()
	h(ctx)
	utils.AssertEqual(t, 403, ctx.Response.StatusCode())
	utils.AssertEqual(t, "Forbidden", string(ctx.Response.Body()))
}

func Test_CSRF_From_Param(t *testing.T) {
	app := fiber.New()

	csrfGroup := app.Group("/:csrf", New(Config{TokenLookup: "param:csrf"}))

	csrfGroup.Post("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Valid CSRF token
	token := utils.UUID()
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.Set(fiber.HeaderCookie, "_csrf="+token)
	ctx.Request.SetRequestURI("/" + token)
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())
}
