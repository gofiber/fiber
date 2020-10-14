package etag

import (
	"net/http/httptest"
	"testing"

	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// go test -run Test_ETag
func Test_ETag(t *testing.T) {
	i := 1<<32 - 1

	t.Logf("%d, %d, %s", int32(i), uint32(i), fasthttp.AppendUint(nil, i))
}

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
