//nolint:bodyclose // Much easier to just ignore memory leaks in tests
package keyauth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

const CorrectKey = "specials: !$%,.#\"!?~`<>@$^*(){}[]|/\\123"

func TestAuthSources(t *testing.T) {
	// define test cases
	testSources := []string{"header", "cookie", "query", "param", "form"}

	tests := []struct {
		route         string
		authTokenName string
		description   string
		APIKey        string
		expectedCode  int
		expectedBody  string
	}{
		{
			route:         "/",
			authTokenName: "access_token",
			description:   "auth with correct key",
			APIKey:        CorrectKey,
			expectedCode:  200,
			expectedBody:  "Success!",
		},
		{
			route:         "/",
			authTokenName: "access_token",
			description:   "auth with no key",
			APIKey:        "",
			expectedCode:  401, // 404 in case of param authentication
			expectedBody:  "missing or malformed API Key",
		},
		{
			route:         "/",
			authTokenName: "access_token",
			description:   "auth with wrong key",
			APIKey:        "WRONGKEY",
			expectedCode:  401,
			expectedBody:  "missing or malformed API Key",
		},
	}

	for _, authSource := range testSources {
		t.Run(authSource, func(t *testing.T) {
			for _, test := range tests {
				// setup the fiber endpoint
				// note that if UnescapePath: false (the default)
				// escaped characters (such as `\"`) will not be handled correctly in the tests
				app := fiber.New(fiber.Config{UnescapePath: true})

				authMiddleware := New(Config{
					KeyLookup: authSource + ":" + test.authTokenName,
					Validator: func(c *fiber.Ctx, key string) (bool, error) {
						if key == CorrectKey {
							return true, nil
						}
						return false, ErrMissingOrMalformedAPIKey
					},
				})

				var route string
				if authSource == param {
					route = test.route + ":" + test.authTokenName
					app.Use(route, authMiddleware)
				} else {
					route = test.route
					app.Use(authMiddleware)
				}

				app.Get(route, func(c *fiber.Ctx) error {
					return c.SendString("Success!")
				})

				// construct the test HTTP request
				var req *http.Request
				req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, test.route, nil)
				utils.AssertEqual(t, err, nil)

				// setup the apikey for the different auth schemes
				if authSource == "header" {
					req.Header.Set(test.authTokenName, test.APIKey)
				} else if authSource == "cookie" {
					req.Header.Set("Cookie", test.authTokenName+"="+test.APIKey)
				} else if authSource == "query" || authSource == "form" {
					q := req.URL.Query()
					q.Add(test.authTokenName, test.APIKey)
					req.URL.RawQuery = q.Encode()
				} else if authSource == "param" {
					r := req.URL.Path
					r += url.PathEscape(test.APIKey)
					req.URL.Path = r
				}

				res, err := app.Test(req, -1)

				utils.AssertEqual(t, nil, err, test.description)

				// test the body of the request
				body, err := io.ReadAll(res.Body)
				// for param authentication, the route would be /:access_token
				// when the access_token is empty, it leads to a 404 (not found)
				// not a 401 (auth error)
				if authSource == "param" && test.APIKey == "" {
					test.expectedCode = 404
					test.expectedBody = "Cannot GET /"
				}
				utils.AssertEqual(t, test.expectedCode, res.StatusCode, test.description)

				// body
				utils.AssertEqual(t, nil, err, test.description)
				utils.AssertEqual(t, test.expectedBody, string(body), test.description)

				err = res.Body.Close()
				utils.AssertEqual(t, err, nil)
			}
		})
	}
}

func TestMultipleKeyAuth(t *testing.T) {
	// setup the fiber endpoint
	app := fiber.New()

	// setup keyauth for /auth1
	app.Use(New(Config{
		Next: func(c *fiber.Ctx) bool {
			return c.OriginalURL() != "/auth1"
		},
		KeyLookup: "header:key",
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			if key == "password1" {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))

	// setup keyauth for /auth2
	app.Use(New(Config{
		Next: func(c *fiber.Ctx) bool {
			return c.OriginalURL() != "/auth2"
		},
		KeyLookup: "header:key",
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			if key == "password2" {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("No auth needed!")
	})

	app.Get("/auth1", func(c *fiber.Ctx) error {
		return c.SendString("Successfully authenticated for auth1!")
	})

	app.Get("/auth2", func(c *fiber.Ctx) error {
		return c.SendString("Successfully authenticated for auth2!")
	})

	// define test cases
	tests := []struct {
		route        string
		description  string
		APIKey       string
		expectedCode int
		expectedBody string
	}{
		// No auth needed for /
		{
			route:        "/",
			description:  "No password needed",
			APIKey:       "",
			expectedCode: 200,
			expectedBody: "No auth needed!",
		},

		// auth needed for auth1
		{
			route:        "/auth1",
			description:  "Normal Authentication Case",
			APIKey:       "password1",
			expectedCode: 200,
			expectedBody: "Successfully authenticated for auth1!",
		},
		{
			route:        "/auth1",
			description:  "Wrong API Key",
			APIKey:       "WRONG KEY",
			expectedCode: 401,
			expectedBody: "missing or malformed API Key",
		},
		{
			route:        "/auth1",
			description:  "Wrong API Key",
			APIKey:       "", // NO KEY
			expectedCode: 401,
			expectedBody: "missing or malformed API Key",
		},

		// Auth 2 has a different password
		{
			route:        "/auth2",
			description:  "Normal Authentication Case for auth2",
			APIKey:       "password2",
			expectedCode: 200,
			expectedBody: "Successfully authenticated for auth2!",
		},
		{
			route:        "/auth2",
			description:  "Wrong API Key",
			APIKey:       "WRONG KEY",
			expectedCode: 401,
			expectedBody: "missing or malformed API Key",
		},
		{
			route:        "/auth2",
			description:  "Wrong API Key",
			APIKey:       "", // NO KEY
			expectedCode: 401,
			expectedBody: "missing or malformed API Key",
		},
	}

	// run the tests
	for _, test := range tests {
		var req *http.Request
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, test.route, nil)
		utils.AssertEqual(t, err, nil)
		if test.APIKey != "" {
			req.Header.Set("key", test.APIKey)
		}

		res, err := app.Test(req, -1)

		utils.AssertEqual(t, nil, err, test.description)

		// test the body of the request
		body, err := io.ReadAll(res.Body)
		utils.AssertEqual(t, test.expectedCode, res.StatusCode, test.description)

		// body
		utils.AssertEqual(t, nil, err, test.description)
		utils.AssertEqual(t, test.expectedBody, string(body), test.description)
	}
}

func TestCustomSuccessAndFailureHandlers(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		SuccessHandler: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusOK).SendString("API key is valid and request was handled by custom success handler")
		},
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusUnauthorized).SendString("API key is invalid and request was handled by custom error handler")
		},
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))

	// Define a test handler that should not be called
	app.Get("/", func(c *fiber.Ctx) error {
		t.Error("Test handler should not be called")
		return nil
	})

	// Create a request without an API key and send it to the app
	res, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, err, nil)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	utils.AssertEqual(t, err, nil)

	// Check that the response has the expected status code and body
	utils.AssertEqual(t, res.StatusCode, http.StatusUnauthorized)
	utils.AssertEqual(t, string(body), "API key is invalid and request was handled by custom error handler")

	// Create a request with a valid API key in the Authorization header
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", CorrectKey))

	// Send the request to the app
	res, err = app.Test(req)
	utils.AssertEqual(t, err, nil)

	// Read the response body into a string
	body, err = io.ReadAll(res.Body)
	utils.AssertEqual(t, err, nil)

	// Check that the response has the expected status code and body
	utils.AssertEqual(t, res.StatusCode, http.StatusOK)
	utils.AssertEqual(t, string(body), "API key is valid and request was handled by custom success handler")
}

func TestCustomNextFunc(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/allowed"
		},
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))

	// Define a test handler
	app.Get("/allowed", func(c *fiber.Ctx) error {
		return c.SendString("API key is valid and request was allowed by custom filter")
	})

	// Create a request with the "/allowed" path and send it to the app
	req := httptest.NewRequest(fiber.MethodGet, "/allowed", nil)
	res, err := app.Test(req)
	utils.AssertEqual(t, err, nil)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	utils.AssertEqual(t, err, nil)

	// Check that the response has the expected status code and body
	utils.AssertEqual(t, res.StatusCode, http.StatusOK)
	utils.AssertEqual(t, string(body), "API key is valid and request was allowed by custom filter")

	// Create a request with a different path and send it to the app without correct key
	req = httptest.NewRequest(fiber.MethodGet, "/not-allowed", nil)
	res, err = app.Test(req)
	utils.AssertEqual(t, err, nil)

	// Read the response body into a string
	body, err = io.ReadAll(res.Body)
	utils.AssertEqual(t, err, nil)

	// Check that the response has the expected status code and body
	utils.AssertEqual(t, res.StatusCode, http.StatusUnauthorized)
	utils.AssertEqual(t, string(body), ErrMissingOrMalformedAPIKey.Error())

	// Create a request with a different path and send it to the app with correct key
	req = httptest.NewRequest(fiber.MethodGet, "/not-allowed", nil)
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", CorrectKey))

	res, err = app.Test(req)
	utils.AssertEqual(t, err, nil)

	// Read the response body into a string
	body, err = io.ReadAll(res.Body)
	utils.AssertEqual(t, err, nil)

	// Check that the response has the expected status code and body
	utils.AssertEqual(t, res.StatusCode, http.StatusUnauthorized)
	utils.AssertEqual(t, string(body), ErrMissingOrMalformedAPIKey.Error())
}

func TestAuthSchemeToken(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		AuthScheme: "Token",
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))

	// Define a test handler
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("API key is valid")
	})

	// Create a request with a valid API key in the "Token" Authorization header
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Add("Authorization", fmt.Sprintf("Token %s", CorrectKey))

	// Send the request to the app
	res, err := app.Test(req)
	utils.AssertEqual(t, err, nil)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	utils.AssertEqual(t, err, nil)

	// Check that the response has the expected status code and body
	utils.AssertEqual(t, res.StatusCode, http.StatusOK)
	utils.AssertEqual(t, string(body), "API key is valid")
}

func TestAuthSchemeBasic(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		KeyLookup:  "header:Authorization",
		AuthScheme: "Basic",
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))

	// Define a test handler
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("API key is valid")
	})

	// Create a request without an API key and  Send the request to the app
	res, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, err, nil)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	utils.AssertEqual(t, err, nil)

	// Check that the response has the expected status code and body
	utils.AssertEqual(t, res.StatusCode, http.StatusUnauthorized)
	utils.AssertEqual(t, string(body), ErrMissingOrMalformedAPIKey.Error())

	// Create a request with a valid API key in the "Authorization" header using the "Basic" scheme
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", CorrectKey))

	// Send the request to the app
	res, err = app.Test(req)
	utils.AssertEqual(t, err, nil)

	// Read the response body into a string
	body, err = io.ReadAll(res.Body)
	utils.AssertEqual(t, err, nil)

	// Check that the response has the expected status code and body
	utils.AssertEqual(t, res.StatusCode, http.StatusOK)
	utils.AssertEqual(t, string(body), "API key is valid")
}
