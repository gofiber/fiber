package keyauth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

const CorrectKey = "specials: !$%,.#!?~`<>@$^*(){}[]|/123"

var testConfig = fiber.TestConfig{
	Timeout: 0,
}

func Test_AuthSources(t *testing.T) {
	// define test cases
	testSources := []string{"header", "cookie", "query", "param", "form"}

	tests := []struct {
		route         string
		authTokenName string
		description   string
		APIKey        string
		expectedBody  string
		expectedCode  int
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
					Validator: func(_ fiber.Ctx, key string) (bool, error) {
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

				app.Get(route, func(c fiber.Ctx) error {
					return c.SendString("Success!")
				})

				// construct the test HTTP request
				var req *http.Request
				req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, test.route, nil)
				require.NoError(t, err)

				// setup the apikey for the different auth schemes
				switch authSource {
				case "header":
					req.Header.Set(test.authTokenName, test.APIKey)
				case "cookie":
					req.Header.Set("Cookie", test.authTokenName+"="+test.APIKey)
				case "query", "form":
					q := req.URL.Query()
					q.Add(test.authTokenName, test.APIKey)
					req.URL.RawQuery = q.Encode()
				case "param":
					r := req.URL.Path
					r += url.PathEscape(test.APIKey)
					req.URL.Path = r
				}

				res, err := app.Test(req, testConfig)

				require.NoError(t, err, test.description)

				// test the body of the request
				body, err := io.ReadAll(res.Body)
				// for param authentication, the route would be /:access_token
				// when the access_token is empty, it leads to a 404 (not found)
				// not a 401 (auth error)
				if authSource == "param" && test.APIKey == "" {
					test.expectedCode = 404
					test.expectedBody = "Cannot GET /"
				}
				require.Equal(t, test.expectedCode, res.StatusCode, test.description)

				// body
				require.NoError(t, err, test.description)
				require.Equal(t, test.expectedBody, string(body), test.description)

				err = res.Body.Close()
				require.NoError(t, err)
			}
		})
	}
}

func TestPanicOnInvalidConfiguration(t *testing.T) {
	require.Panics(t, func() {
		authMiddleware := New(Config{
			KeyLookup: "invalid",
		})
		// We shouldn't even make it this far, but these next two lines prevent authMiddleware from being an unused variable.
		app := fiber.New()
		defer func() { // testing panics, defer block to ensure cleanup
			err := app.Shutdown()
			require.NoError(t, err)
		}()
		app.Use(authMiddleware)
	}, "should panic if Validator is missing")

	require.Panics(t, func() {
		authMiddleware := New(Config{
			KeyLookup: "invalid",
			Validator: func(_ fiber.Ctx, _ string) (bool, error) {
				return true, nil
			},
		})
		// We shouldn't even make it this far, but these next two lines prevent authMiddleware from being an unused variable.
		app := fiber.New()
		defer func() { // testing panics, defer block to ensure cleanup
			err := app.Shutdown()
			require.NoError(t, err)
		}()
		app.Use(authMiddleware)
	}, "should panic if CustomKeyLookup is not set AND KeyLookup has an invalid value")
}

func TestCustomKeyUtilityFunctionErrors(t *testing.T) {
	const (
		scheme = "Bearer"
	)

	// Invalid element while parsing
	_, err := DefaultKeyLookup("invalid", scheme)
	require.Error(t, err, "DefaultKeyLookup should fail for 'invalid' keyLookup")

	_, err = MultipleKeySourceLookup([]string{"header:key", "invalid"}, scheme)
	require.Error(t, err, "MultipleKeySourceLookup should fail for 'invalid' keyLookup")
}

func TestMultipleKeyLookup(t *testing.T) {
	const (
		desc    = "auth with correct key"
		success = "Success!"
		scheme  = "Bearer"
	)

	// setup the fiber endpoint
	app := fiber.New()

	customKeyLookup, err := MultipleKeySourceLookup([]string{"header:key", "cookie:key", "query:key"}, scheme)
	require.NoError(t, err)

	authMiddleware := New(Config{
		CustomKeyLookup: customKeyLookup,
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	})
	app.Use(authMiddleware)
	app.Get("/foo", func(c fiber.Ctx) error {
		return c.SendString(success)
	})

	// construct the test HTTP request
	var req *http.Request
	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/foo", nil)
	require.NoError(t, err)
	q := req.URL.Query()
	q.Add("key", CorrectKey)
	req.URL.RawQuery = q.Encode()

	res, err := app.Test(req, testConfig)

	require.NoError(t, err)

	// test the body of the request
	body, err := io.ReadAll(res.Body)
	require.Equal(t, 200, res.StatusCode, desc)
	// body
	require.NoError(t, err)
	require.Equal(t, success, string(body), desc)

	err = res.Body.Close()
	require.NoError(t, err)

	// construct a second request without proper key
	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/foo", nil)
	require.NoError(t, err)
	res, err = app.Test(req, testConfig)
	require.NoError(t, err)
	errBody, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, ErrMissingOrMalformedAPIKey.Error(), string(errBody))
}

func Test_MultipleKeyAuth(t *testing.T) {
	// setup the fiber endpoint
	app := fiber.New()

	// setup keyauth for /auth1
	app.Use(New(Config{
		Next: func(c fiber.Ctx) bool {
			return c.OriginalURL() != "/auth1"
		},
		KeyLookup: "header:key",
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == "password1" {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))

	// setup keyauth for /auth2
	app.Use(New(Config{
		Next: func(c fiber.Ctx) bool {
			return c.OriginalURL() != "/auth2"
		},
		KeyLookup: "header:key",
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == "password2" {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("No auth needed!")
	})

	app.Get("/auth1", func(c fiber.Ctx) error {
		return c.SendString("Successfully authenticated for auth1!")
	})

	app.Get("/auth2", func(c fiber.Ctx) error {
		return c.SendString("Successfully authenticated for auth2!")
	})

	// define test cases
	tests := []struct {
		route        string
		description  string
		APIKey       string
		expectedBody string
		expectedCode int
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
		require.NoError(t, err)
		if test.APIKey != "" {
			req.Header.Set("key", test.APIKey)
		}

		res, err := app.Test(req, testConfig)

		require.NoError(t, err, test.description)

		// test the body of the request
		body, err := io.ReadAll(res.Body)
		require.Equal(t, test.expectedCode, res.StatusCode, test.description)

		// body
		require.NoError(t, err, test.description)
		require.Equal(t, test.expectedBody, string(body), test.description)
	}
}

func Test_CustomSuccessAndFailureHandlers(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		SuccessHandler: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusOK).SendString("API key is valid and request was handled by custom success handler")
		},
		ErrorHandler: func(c fiber.Ctx, _ error) error {
			return c.Status(fiber.StatusUnauthorized).SendString("API key is invalid and request was handled by custom error handler")
		},
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))

	// Define a test handler that should not be called
	app.Get("/", func(_ fiber.Ctx) error {
		t.Error("Test handler should not be called")
		return nil
	})

	// Create a request without an API key and send it to the app
	res, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	// Check that the response has the expected status code and body
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, "API key is invalid and request was handled by custom error handler", string(body))

	// Create a request with a valid API key in the Authorization header
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Add("Authorization", "Bearer "+CorrectKey)

	// Send the request to the app
	res, err = app.Test(req)
	require.NoError(t, err)

	// Read the response body into a string
	body, err = io.ReadAll(res.Body)
	require.NoError(t, err)

	// Check that the response has the expected status code and body
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "API key is valid and request was handled by custom success handler", string(body))
}

func Test_CustomNextFunc(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		Next: func(c fiber.Ctx) bool {
			return c.Path() == "/allowed"
		},
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))

	// Define a test handler
	app.Get("/allowed", func(c fiber.Ctx) error {
		return c.SendString("API key is valid and request was allowed by custom filter")
	})

	// Create a request with the "/allowed" path and send it to the app
	req := httptest.NewRequest(fiber.MethodGet, "/allowed", nil)
	res, err := app.Test(req)
	require.NoError(t, err)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	// Check that the response has the expected status code and body
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "API key is valid and request was allowed by custom filter", string(body))

	// Create a request with a different path and send it to the app without correct key
	req = httptest.NewRequest(fiber.MethodGet, "/not-allowed", nil)
	res, err = app.Test(req)
	require.NoError(t, err)

	// Read the response body into a string
	body, err = io.ReadAll(res.Body)
	require.NoError(t, err)

	// Check that the response has the expected status code and body
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, string(body), ErrMissingOrMalformedAPIKey.Error())

	// Create a request with a different path and send it to the app with correct key
	req = httptest.NewRequest(fiber.MethodGet, "/not-allowed", nil)
	req.Header.Add("Authorization", "Basic "+CorrectKey)

	res, err = app.Test(req)
	require.NoError(t, err)

	// Read the response body into a string
	body, err = io.ReadAll(res.Body)
	require.NoError(t, err)

	// Check that the response has the expected status code and body
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, string(body), ErrMissingOrMalformedAPIKey.Error())
}

func Test_TokenFromContext_None(t *testing.T) {
	app := fiber.New()
	// Define a test handler that checks TokenFromContext
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(TokenFromContext(c))
	})

	// Verify a "" is sent back if nothing sets the token on the context.
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	// Send
	res, err := app.Test(req)
	require.NoError(t, err)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Empty(t, body)
}

func Test_TokenFromContext(t *testing.T) {
	// Test that TokenFromContext returns the correct token
	t.Run("fiber.Ctx", func(t *testing.T) {
		app := fiber.New()
		app.Use(New(Config{
			KeyLookup:  "header:Authorization",
			AuthScheme: "Basic",
			Validator: func(_ fiber.Ctx, key string) (bool, error) {
				if key == CorrectKey {
					return true, nil
				}
				return false, ErrMissingOrMalformedAPIKey
			},
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString(TokenFromContext(c))
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", nil)
		req.Header.Add("Authorization", "Basic "+CorrectKey)
		res, err := app.Test(req)
		require.NoError(t, err)

		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.Equal(t, CorrectKey, string(body))
	})

	t.Run("context.Context", func(t *testing.T) {
		app := fiber.New()
		app.Use(New(Config{
			KeyLookup:  "header:Authorization",
			AuthScheme: "Basic",
			Validator: func(_ fiber.Ctx, key string) (bool, error) {
				if key == CorrectKey {
					return true, nil
				}
				return false, ErrMissingOrMalformedAPIKey
			},
		}))
		// Verify that TokenFromContext works with context.Context
		app.Get("/", func(c fiber.Ctx) error {
			ctx := c.Context()
			token := TokenFromContext(ctx)
			return c.SendString(token)
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", nil)
		req.Header.Add("Authorization", "Basic "+CorrectKey)
		res, err := app.Test(req)
		require.NoError(t, err)

		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.Equal(t, CorrectKey, string(body))
	})

	t.Run("invalid context type", func(t *testing.T) {
		require.Panics(t, func() {
			_ = TokenFromContext("invalid")
		})
	})
}

func Test_AuthSchemeToken(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		AuthScheme: "Token",
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))

	// Define a test handler
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("API key is valid")
	})

	// Create a request with a valid API key in the "Token" Authorization header
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Add("Authorization", "Token "+CorrectKey)

	// Send the request to the app
	res, err := app.Test(req)
	require.NoError(t, err)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	// Check that the response has the expected status code and body
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "API key is valid", string(body))
}

func Test_AuthSchemeBasic(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		KeyLookup:  "header:Authorization",
		AuthScheme: "Basic",
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))

	// Define a test handler
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("API key is valid")
	})

	// Create a request without an API key and  Send the request to the app
	res, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	// Check that the response has the expected status code and body
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, string(body), ErrMissingOrMalformedAPIKey.Error())

	// Create a request with a valid API key in the "Authorization" header using the "Basic" scheme
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Add("Authorization", "Basic "+CorrectKey)

	// Send the request to the app
	res, err = app.Test(req)
	require.NoError(t, err)

	// Read the response body into a string
	body, err = io.ReadAll(res.Body)
	require.NoError(t, err)

	// Check that the response has the expected status code and body
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "API key is valid", string(body))
}
