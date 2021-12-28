package csrf

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

// ErrorHandler that passes ErrInternalServerError
// if erris nil or not a valid csrf err message.
func errorHandler(c *fiber.Ctx, err error) error {
	if err == nil {
		// called with err = nil
		return fiber.ErrInternalServerError
	}
	switch err.Error() {
	case "missing csrf token in header",
		"missing csrf token in query",
		"missing csrf token in param",
		"missing csrf token in form",
		"missing csrf token in cookie",
		"csrf token not found":
		return fiber.ErrForbidden
	default:
		// not a valid err
		return fiber.ErrInternalServerError
	}
}

func Test_CSRF(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{ErrorHandler: errorHandler}))

	app.Post("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	methods := [4]string{"GET", "HEAD", "OPTIONS", "TRACE"}

	for _, method := range methods {
		// Generate CSRF token
		ctx.Request.Header.SetMethod(method)
		h(ctx)

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
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(method)
		h(ctx)
		token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
		token = strings.Split(strings.Split(token, ";")[0], "=")[1]

		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod("POST")
		ctx.Request.Header.Set("X-CSRF-Token", token)
		h(ctx)
		utils.AssertEqual(t, 200, ctx.Response.StatusCode())
	}
}

// go test -run Test_CSRF_Next
func Test_CSRF_Next(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		ErrorHandler: errorHandler,
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_CSRF_Invalid_KeyLookup(t *testing.T) {
	defer func() {
		utils.AssertEqual(t, "[CSRF] KeyLookup must in the form of <source>:<key>", recover())
	}()
	app := fiber.New()

	app.Use(New(Config{ErrorHandler: errorHandler, KeyLookup: "I:am:invalid"}))

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

	app.Use(New(Config{ErrorHandler: errorHandler, KeyLookup: "form:_csrf"}))

	app.Post("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Invalid CSRF token
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	h(ctx)
	utils.AssertEqual(t, 403, ctx.Response.StatusCode())

	// Generate CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod("GET")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	ctx.Request.SetBodyString("_csrf=" + token)
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())
}

func Test_CSRF_From_Query(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{ErrorHandler: errorHandler, KeyLookup: "query:_csrf"}))

	app.Post("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Invalid CSRF token
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/?_csrf=" + utils.UUID())
	h(ctx)
	utils.AssertEqual(t, 403, ctx.Response.StatusCode())

	// Generate CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.SetRequestURI("/?_csrf=" + token)
	ctx.Request.Header.SetMethod("POST")
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())
	utils.AssertEqual(t, "OK", string(ctx.Response.Body()))
}

func Test_CSRF_From_Param(t *testing.T) {
	app := fiber.New()

	csrfGroup := app.Group("/:csrf", New(Config{ErrorHandler: errorHandler, KeyLookup: "param:csrf"}))

	csrfGroup.Post("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Invalid CSRF token
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/" + utils.UUID())
	h(ctx)
	utils.AssertEqual(t, 403, ctx.Response.StatusCode())

	// Generate CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/" + utils.UUID())
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.SetRequestURI("/" + token)
	ctx.Request.Header.SetMethod("POST")
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())
	utils.AssertEqual(t, "OK", string(ctx.Response.Body()))
}

func Test_CSRF_From_Cookie(t *testing.T) {
	app := fiber.New()

	csrfGroup := app.Group("/", New(Config{ErrorHandler: errorHandler, KeyLookup: "cookie:csrf"}))

	csrfGroup.Post("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Invalid CSRF token
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.Set(fiber.HeaderCookie, "csrf="+utils.UUID()+";")
	h(ctx)
	utils.AssertEqual(t, 403, ctx.Response.StatusCode())

	// Generate CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.Set(fiber.HeaderCookie, "csrf="+token+";")
	ctx.Request.SetRequestURI("/")
	h(ctx)
	utils.AssertEqual(t, 200, ctx.Response.StatusCode())
	utils.AssertEqual(t, "OK", string(ctx.Response.Body()))
}

func Test_CSRF_ErrorHandler_InvalidToken(t *testing.T) {
	app := fiber.New()

	errHandler := func(ctx *fiber.Ctx, err error) error {
		return ctx.Status(419).Send([]byte("invalid CSRF token"))
	}

	app.Use(New(Config{ErrorHandler: errHandler}))

	app.Post("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod("GET")
	h(ctx)

	// invalid CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.Set("X-CSRF-Token", "johndoe")
	h(ctx)
	utils.AssertEqual(t, 419, ctx.Response.StatusCode())
	utils.AssertEqual(t, "invalid CSRF token", string(ctx.Response.Body()))
}

func Test_CSRF_ErrorHandler_EmptyToken(t *testing.T) {
	app := fiber.New()

	errHandler := func(ctx *fiber.Ctx, err error) error {
		return ctx.Status(419).Send([]byte("empty CSRF token"))
	}

	app.Use(New(Config{ErrorHandler: errHandler}))

	app.Post("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod("GET")
	h(ctx)

	// empty CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod("POST")
	h(ctx)
	utils.AssertEqual(t, 419, ctx.Response.StatusCode())
	utils.AssertEqual(t, "empty CSRF token", string(ctx.Response.Body()))
}
