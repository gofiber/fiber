package hcaptcha

import (
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http/httptest"
	"testing"
)

const (
	TestSecretKey     = "0x0000000000000000000000000000000000000000"
	TestResponseToken = "20000000-aaaa-bbbb-cccc-000000000002" // Got by using this site key: 20000000-ffff-ffff-ffff-000000000002
)

func TestHCaptcha(t *testing.T) {
	app := fiber.New()

	m := New(Config{
		SecretKey: TestSecretKey,
		ResponseKeyFunc: func(c fiber.Ctx) (string, error) {
			return c.Query("token"), nil
		},
	})

	app.Get("/hcaptcha", m, func(c fiber.Ctx) error {
		return c.Status(200).SendString("ok")
	})

	req := httptest.NewRequest("GET", "/hcaptcha?token="+TestResponseToken, nil)
	req.Header.Set("Content-Type", "application/json")

	res, err := app.Test(req, -1)
	defer res.Body.Close()

	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, res.StatusCode, fiber.StatusOK, "Response status code")

	body, err := io.ReadAll(res.Body)

	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "ok", string(body))
}
