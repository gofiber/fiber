//nolint:bodyclose // Much easier to just ignore memory leaks in tests
package redirect

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

func Test_Redirect(t *testing.T) {
	app := *fiber.New()

	app.Use(New(Config{
		Rules: map[string]string{
			"/default": "google.com",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))
	app.Use(New(Config{
		Rules: map[string]string{
			"/default/*": "fiber.wiki",
		},
		StatusCode: fiber.StatusTemporaryRedirect,
	}))
	app.Use(New(Config{
		Rules: map[string]string{
			"/redirect/*": "$1",
		},
		StatusCode: fiber.StatusSeeOther,
	}))
	app.Use(New(Config{
		Rules: map[string]string{
			"/pattern/*": "golang.org",
		},
		StatusCode: fiber.StatusFound,
	}))

	app.Use(New(Config{
		Rules: map[string]string{
			"/": "/swagger",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))
	app.Use(New(Config{
		Rules: map[string]string{
			"/params": "/with_params",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))

	app.Get("/api/*", func(c *fiber.Ctx) error {
		return c.SendString("API")
	})

	app.Get("/new", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	tests := []struct {
		name       string
		url        string
		redirectTo string
		statusCode int
	}{
		{
			name:       "should be returns status StatusFound without a wildcard",
			url:        "/default",
			redirectTo: "google.com",
			statusCode: fiber.StatusMovedPermanently,
		},
		{
			name:       "should be returns status StatusTemporaryRedirect  using wildcard",
			url:        "/default/xyz",
			redirectTo: "fiber.wiki",
			statusCode: fiber.StatusTemporaryRedirect,
		},
		{
			name:       "should be returns status StatusSeeOther without set redirectTo to use the default",
			url:        "/redirect/github.com/gofiber/redirect",
			redirectTo: "github.com/gofiber/redirect",
			statusCode: fiber.StatusSeeOther,
		},
		{
			name:       "should return the status code default",
			url:        "/pattern/xyz",
			redirectTo: "golang.org",
			statusCode: fiber.StatusFound,
		},
		{
			name:       "access URL without rule",
			url:        "/new",
			statusCode: fiber.StatusOK,
		},
		{
			name:       "redirect to swagger route",
			url:        "/",
			redirectTo: "/swagger",
			statusCode: fiber.StatusMovedPermanently,
		},
		{
			name:       "no redirect to swagger route",
			url:        "/api/",
			statusCode: fiber.StatusOK,
		},
		{
			name:       "no redirect to swagger route #2",
			url:        "/api/test",
			statusCode: fiber.StatusOK,
		},
		{
			name:       "redirect with query params",
			url:        "/params?query=abc",
			redirectTo: "/with_params?query=abc",
			statusCode: fiber.StatusMovedPermanently,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, tt.url, nil)
			utils.AssertEqual(t, err, nil)
			req.Header.Set("Location", "github.com/gofiber/redirect")
			resp, err := app.Test(req)

			utils.AssertEqual(t, err, nil)
			utils.AssertEqual(t, tt.statusCode, resp.StatusCode)
			utils.AssertEqual(t, tt.redirectTo, resp.Header.Get("Location"))
		})
	}
}

func Test_Next(t *testing.T) {
	// Case 1 : Next function always returns true
	app := *fiber.New()
	app.Use(New(Config{
		Next: func(*fiber.Ctx) bool {
			return true
		},
		Rules: map[string]string{
			"/default": "google.com",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))

	app.Use(func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", nil)
	utils.AssertEqual(t, err, nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, err, nil)

	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	// Case 2 : Next function always returns false
	app = *fiber.New()
	app.Use(New(Config{
		Next: func(*fiber.Ctx) bool {
			return false
		},
		Rules: map[string]string{
			"/default": "google.com",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", nil)
	utils.AssertEqual(t, err, nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, err, nil)

	utils.AssertEqual(t, fiber.StatusMovedPermanently, resp.StatusCode)
	utils.AssertEqual(t, "google.com", resp.Header.Get("Location"))
}

func Test_NoRules(t *testing.T) {
	// Case 1: No rules with default route defined
	app := *fiber.New()

	app.Use(New(Config{
		StatusCode: fiber.StatusMovedPermanently,
	}))

	app.Use(func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", nil)
	utils.AssertEqual(t, err, nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, err, nil)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	// Case 2: No rules and no default route defined
	app = *fiber.New()

	app.Use(New(Config{
		StatusCode: fiber.StatusMovedPermanently,
	}))

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", nil)
	utils.AssertEqual(t, err, nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, err, nil)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_DefaultConfig(t *testing.T) {
	// Case 1: Default config and no default route
	app := *fiber.New()

	app.Use(New())

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", nil)
	utils.AssertEqual(t, err, nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, err, nil)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)

	// Case 2: Default config and default route
	app = *fiber.New()

	app.Use(New())
	app.Use(func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", nil)
	utils.AssertEqual(t, err, nil)
	resp, err = app.Test(req)

	utils.AssertEqual(t, err, nil)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
}

func Test_RegexRules(t *testing.T) {
	// Case 1: Rules regex is empty
	app := *fiber.New()
	app.Use(New(Config{
		Rules:      map[string]string{},
		StatusCode: fiber.StatusMovedPermanently,
	}))

	app.Use(func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", nil)
	utils.AssertEqual(t, err, nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, err, nil)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	// Case 2: Rules regex map contains valid regex and well-formed replacement URLs
	app = *fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/default": "google.com",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))

	app.Use(func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/default", nil)
	utils.AssertEqual(t, err, nil)
	resp, err = app.Test(req)

	utils.AssertEqual(t, err, nil)
	utils.AssertEqual(t, fiber.StatusMovedPermanently, resp.StatusCode)
	utils.AssertEqual(t, "google.com", resp.Header.Get("Location"))

	// Case 3: Test invalid regex throws panic
	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from invalid regex: ", r)
		}
	}()

	app = *fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"(": "google.com",
		},
		StatusCode: fiber.StatusMovedPermanently,
	}))
	t.Error("Expected panic, got nil")
}
