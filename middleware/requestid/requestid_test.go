package requestid

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// go test -run Test_RequestID
func Test_RequestID(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World 👋!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	reqid := resp.Header.Get(fiber.HeaderXRequestID)
	utils.AssertEqual(t, 36, len(reqid))

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Add(fiber.HeaderXRequestID, reqid)

	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, reqid, resp.Header.Get(fiber.HeaderXRequestID))
}

// go test -run Test_RequestID_Next
func Test_RequestID_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, resp.Header.Get(fiber.HeaderXRequestID), "")
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -run Test_RequestID_Locals
func Test_RequestID_Locals(t *testing.T) {
	t.Parallel()
	reqID := "ThisIsARequestId"
	ctxKey := "ThisIsAContextKey"

	app := fiber.New()
	app.Use(New(Config{
		Generator: func() string {
			return reqID
		},
		ContextKey: ctxKey,
	}))

	var ctxVal string

	app.Use(func(c *fiber.Ctx) error {
		ctxVal = c.Locals(ctxKey).(string) //nolint:forcetypeassert,errcheck // We always store a string in here
		return c.Next()
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, reqID, ctxVal)
}
