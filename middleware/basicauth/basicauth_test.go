package basicauth

import (
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	b64 "encoding/base64"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// go test -run Test_BasicAuth_Next
func Test_BasicAuth_Next(t *testing.T) {
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

func Test_Middleware_BasicAuth(t *testing.T) {
	app := fiber.New()

	cfg := Config{
		Users: map[string]string{
			"john":  "doe",
			"admin": "123456",
		},
	}

	app.Use(New(cfg))

	app.Get("/testauth", func(c *fiber.Ctx) error {
		username := c.Locals("username").(string)
		password := c.Locals("password").(string)

		return c.SendString(username + password)
	})

	tests := []struct {
		url        string
		statusCode int
		username   string
		password   string
	}{
		{
			url:        "/testauth",
			statusCode: 200,
			username:   "john",
			password:   "doe",
		},
		{
			url:        "/testauth",
			statusCode: 200,
			username:   "admin",
			password:   "123456",
		},
		{
			url:        "/testauth",
			statusCode: 401,
			username:   "ee",
			password:   "123456",
		},
	}

	for _, tt := range tests {
		// Base64 encode credentials for http auth header
		creds := b64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", tt.username, tt.password)))

		req := httptest.NewRequest("GET", "/testauth", nil)
		req.Header.Add("Authorization", "Basic "+creds)
		resp, err := app.Test(req)

		body, err := ioutil.ReadAll(resp.Body)

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, tt.statusCode, resp.StatusCode)

		// Only check body if statusCode is 200
		if tt.statusCode == 200 {
			utils.AssertEqual(t, fmt.Sprintf("%s%s", tt.username, tt.password), string(body))
		}
	}
}
