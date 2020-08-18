package compress

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
)

// go test -run Test_Compress
func Test_Compress(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("../filesystem/data/img/map.png", true)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "image/png", resp.Header.Get(fiber.HeaderContentType))
	utils.AssertEqual(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
}
