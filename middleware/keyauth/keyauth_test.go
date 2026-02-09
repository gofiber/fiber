package keyauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
)

const CorrectKey = "correct-token_123./~+"

var testConfig = fiber.TestConfig{
	Timeout: 0,
}

const (
	paramExtractorName      = "param"
	formExtractorName       = "form"
	queryExtractorName      = "query"
	headerExtractorName     = "header"
	authHeaderExtractorName = "authHeader"
	cookieExtractorName     = "cookie"
)

func Test_AuthSources(t *testing.T) {
	// define test cases
	testSources := []string{headerExtractorName, authHeaderExtractorName, cookieExtractorName, queryExtractorName, paramExtractorName, formExtractorName}

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
			expectedBody:  ErrMissingOrMalformedAPIKey.Error(),
		},
		{
			route:         "/",
			authTokenName: "access_token",
			description:   "auth with wrong key",
			APIKey:        "WRONGKEY",
			expectedCode:  401,
			expectedBody:  ErrMissingOrMalformedAPIKey.Error(),
		},
	}

	for _, authSource := range testSources {
		t.Run(authSource, func(t *testing.T) {
			for _, test := range tests {
				app := fiber.New(fiber.Config{UnescapePath: true})

				testKey := test.APIKey
				correctKey := CorrectKey

				// Use a simple key for param and cookie to avoid encoding issues in the test setup
				if authSource == paramExtractorName || authSource == cookieExtractorName {
					if test.APIKey != "" && test.APIKey != "WRONGKEY" {
						testKey = "simple-key"
						correctKey = "simple-key"
					}
				}

				authMiddleware := New(Config{
					Extractor: func() extractors.Extractor {
						switch authSource {
						case headerExtractorName:
							return extractors.FromHeader(test.authTokenName)
						case authHeaderExtractorName:
							return extractors.FromAuthHeader("Bearer")
						case cookieExtractorName:
							return extractors.FromCookie(test.authTokenName)
						case queryExtractorName:
							return extractors.FromQuery(test.authTokenName)
						case paramExtractorName:
							return extractors.FromParam(test.authTokenName)
						case formExtractorName:
							return extractors.FromForm(test.authTokenName)
						default:
							panic("unknown source")
						}
					}(),
					Validator: func(_ fiber.Ctx, key string) (bool, error) {
						if key == correctKey {
							return true, nil
						}
						return false, errors.New("invalid key")
					},
				})

				handler := func(c fiber.Ctx) error {
					return c.SendString("Success!")
				}

				method := fiber.MethodGet
				switch authSource {
				case paramExtractorName:
					app.Get("/:"+test.authTokenName, authMiddleware, handler)
				case formExtractorName:
					method = fiber.MethodPost
					app.Post("/", authMiddleware, handler)
				default:
					app.Get("/", authMiddleware, handler)
				}

				targetURL := "/"
				if authSource == paramExtractorName {
					targetURL = "/" + url.PathEscape(testKey)
				}

				var reqBody io.Reader
				if authSource == formExtractorName {
					form := url.Values{}
					form.Add(test.authTokenName, testKey)
					bodyStr := form.Encode()
					reqBody = strings.NewReader(bodyStr)
				}

				req, err := http.NewRequestWithContext(context.Background(), method, targetURL, reqBody)
				require.NoError(t, err)

				switch authSource {
				case headerExtractorName:
					req.Header.Set(test.authTokenName, testKey)
				case authHeaderExtractorName:
					if testKey != "" {
						req.Header.Set("Authorization", "Bearer "+testKey)
					}
				case cookieExtractorName:
					req.Header.Set("Cookie", test.authTokenName+"="+testKey)
				case queryExtractorName:
					q := req.URL.Query()
					q.Add(test.authTokenName, testKey)
					req.URL.RawQuery = q.Encode()
				case formExtractorName:
					req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				default:
					// nothing to do for paramExtractorName
				}

				res, err := app.Test(req, testConfig)
				require.NoError(t, err, test.description)

				body, err := io.ReadAll(res.Body)

				require.NoError(t, err)
				errClose := res.Body.Close()
				require.NoError(t, errClose)

				expectedCode := test.expectedCode
				expectedBody := test.expectedBody
				if test.APIKey == "" || test.APIKey == "WRONGKEY" {
					expectedBody = ErrMissingOrMalformedAPIKey.Error()
				}

				if authSource == paramExtractorName && testKey == "" {
					expectedCode = 404
					expectedBody = "Not Found"
				}
				require.Equal(t, expectedCode, res.StatusCode, test.description)
				require.Equal(t, expectedBody, string(body), test.description)
			}
		})
	}
}

func TestMultipleKeyLookup(t *testing.T) {
	const (
		desc    = "auth with correct key"
		success = "Success!"
		scheme  = "Bearer"
	)

	// set up the fiber endpoint
	app := fiber.New()

	customExtractor := extractors.Chain(
		extractors.FromAuthHeader("Bearer"),
		extractors.FromHeader("key"),
		extractors.FromCookie("key"),
		extractors.FromQuery("key"),
	)

	authMiddleware := New(Config{
		Extractor: customExtractor,
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, errors.New("invalid key")
		},
	})
	app.Use(authMiddleware)
	app.Get("/foo", func(c fiber.Ctx) error {
		return c.SendString(success)
	})

	// construct the test HTTP request
	var (
		req *http.Request
		err error
	)
	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/foo", http.NoBody)
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
	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/foo", http.NoBody)
	require.NoError(t, err)
	res, err = app.Test(req, testConfig)
	require.NoError(t, err)
	errBody, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, ErrMissingOrMalformedAPIKey.Error(), string(errBody))
}

func Test_MultipleKeyAuth(t *testing.T) {
	// set up the fiber endpoint
	app := fiber.New()

	// set up keyauth for /auth1
	app.Use(New(Config{
		Next: func(c fiber.Ctx) bool {
			return c.Path() != "/auth1"
		},
		Extractor: extractors.FromAuthHeader("Bearer"),
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == "password1" {
				return true, nil
			}
			return false, errors.New("invalid key")
		},
	}))

	// setup keyauth for /auth2
	app.Use(New(Config{
		Next: func(c fiber.Ctx) bool {
			return c.Path() != "/auth2"
		},
		Extractor: extractors.FromAuthHeader("Bearer"),
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == "password2" {
				return true, nil
			}
			return false, errors.New("invalid key")
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
			expectedBody: ErrMissingOrMalformedAPIKey.Error(),
		},
		{
			route:        "/auth1",
			description:  "Wrong API Key",
			APIKey:       "", // NO KEY
			expectedCode: 401,
			expectedBody: ErrMissingOrMalformedAPIKey.Error(),
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
			expectedBody: ErrMissingOrMalformedAPIKey.Error(),
		},
		{
			route:        "/auth2",
			description:  "Wrong API Key",
			APIKey:       "", // NO KEY
			expectedCode: 401,
			expectedBody: ErrMissingOrMalformedAPIKey.Error(),
		},
	}

	// run the tests
	for _, test := range tests {
		var req *http.Request
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, test.route, http.NoBody)
		require.NoError(t, err)
		if test.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+test.APIKey)
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
	res, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	// Check that the response has the expected status code and body
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, "API key is invalid and request was handled by custom error handler", string(body))

	// Create a request with a valid API key in the Authorization header
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
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
	app.Get("/not-allowed", func(c fiber.Ctx) error {
		return c.SendString("Should be protected")
	})

	// Create a request with the "/allowed" path and send it to the app
	req := httptest.NewRequest(fiber.MethodGet, "/allowed", http.NoBody)
	res, err := app.Test(req)
	require.NoError(t, err)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	// Check that the response has the expected status code and body
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "API key is valid and request was allowed by custom filter", string(body))

	// Create a request with a different path and send it to the app without correct key
	req = httptest.NewRequest(fiber.MethodGet, "/not-allowed", http.NoBody)
	res, err = app.Test(req)
	require.NoError(t, err)

	// Read the response body into a string
	body, err = io.ReadAll(res.Body)
	require.NoError(t, err)

	// Check that the response has the expected status code and body
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, ErrMissingOrMalformedAPIKey.Error(), string(body))

	// Create a request with a different path and send it to the app with correct key
	req = httptest.NewRequest(fiber.MethodGet, "/not-allowed", http.NoBody)
	req.Header.Add("Authorization", "Bearer "+CorrectKey)

	res, err = app.Test(req)
	require.NoError(t, err)

	// Read the response body into a string
	body, err = io.ReadAll(res.Body)
	require.NoError(t, err)

	// Check that the response has the expected status code and body
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "Should be protected", string(body))
}

func Test_TokenFromContext_None(t *testing.T) {
	app := fiber.New()
	// Define a test handler that checks TokenFromContext
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(TokenFromContext(c))
	})

	// Verify a "" is sent back if nothing sets the token on the context.
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	// Send
	res, err := app.Test(req)
	require.NoError(t, err)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Empty(t, body)
}

func Test_TokenFromContext(t *testing.T) {
	app := fiber.New()
	// Wire up keyauth middleware to set TokenFromContext now
	app.Use(New(Config{
		Extractor: extractors.FromAuthHeader("Basic"),
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))
	// Define a test handler that checks TokenFromContext
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(TokenFromContext(c))
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add("Authorization", "Basic "+CorrectKey)
	// Send
	res, err := app.Test(req)
	require.NoError(t, err)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, CorrectKey, string(body))
}

func Test_TokenFromContext_Types(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Extractor: extractors.FromAuthHeader("Basic"),
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		require.Equal(t, CorrectKey, TokenFromContext(c))
		require.Equal(t, CorrectKey, TokenFromContext(c.RequestCtx()))
		require.Equal(t, CorrectKey, TokenFromContext(c.Context()))
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add("Authorization", "Basic "+CorrectKey)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_AuthSchemeToken(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		Extractor: extractors.FromAuthHeader("Token"),
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
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
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
		Extractor: extractors.FromAuthHeader("Basic"),
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
	res, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	// Read the response body into a string
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	// Check that the response has the expected status code and body
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, ErrMissingOrMalformedAPIKey.Error(), string(body))

	// Create a request with a valid API key in the "Authorization" header using the "Basic" scheme
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
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

func Test_HeaderSchemeCaseInsensitive(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add("Authorization", "bearer "+CorrectKey)
	res, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "OK", string(body))
}

func Test_DefaultErrorHandlerChallenge(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Validator: func(_ fiber.Ctx, _ string) (bool, error) {
			return false, ErrMissingOrMalformedAPIKey
		},
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	res, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, "Bearer realm=\"Restricted\"", res.Header.Get("WWW-Authenticate"))
}

func Test_DefaultErrorHandlerInvalid(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Validator: func(_ fiber.Ctx, _ string) (bool, error) {
			return false, errors.New("invalid")
		},
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add("Authorization", "Bearer "+CorrectKey)
	res, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, ErrMissingOrMalformedAPIKey.Error(), string(body))
	require.Equal(t, "Bearer realm=\"Restricted\"", res.Header.Get("WWW-Authenticate"))
}

func Test_HeaderSchemeMultipleSpaces(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, ErrMissingOrMalformedAPIKey
		},
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add("Authorization", "Bearer    "+CorrectKey)
	res, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, ErrMissingOrMalformedAPIKey.Error(), string(body))
}

func Test_HeaderSchemeMissingSpace(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{Validator: func(_ fiber.Ctx, _ string) (bool, error) {
		return false, ErrMissingOrMalformedAPIKey
	}}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add("Authorization", "Bearer"+CorrectKey)
	res, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, ErrMissingOrMalformedAPIKey.Error(), string(body))
}

func Test_HeaderSchemeNoToken(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{Validator: func(_ fiber.Ctx, _ string) (bool, error) {
		return false, ErrMissingOrMalformedAPIKey
	}}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add("Authorization", "Bearer ")
	res, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, ErrMissingOrMalformedAPIKey.Error(), string(body))
}

func Test_HeaderSchemeNoSeparator(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{Validator: func(_ fiber.Ctx, _ string) (bool, error) {
		return false, ErrMissingOrMalformedAPIKey
	}}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	// No space between "Bearer" and token
	req.Header.Add("Authorization", "BearerTokenWithoutSpace")
	res, err := app.Test(req)
	require.NoError(t, err)

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, ErrMissingOrMalformedAPIKey.Error(), string(body))
}

func Test_HeaderSchemeEmptyTokenAfterTrim(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Validator: func(_ fiber.Ctx, _ string) (bool, error) {
			return false, ErrMissingOrMalformedAPIKey
		},
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	// Authorization header with scheme followed by only spaces/tabs (no actual token)
	req.Header.Add("Authorization", "Bearer \t  \t ")
	res, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, ErrMissingOrMalformedAPIKey.Error(), string(body))
}

func Test_WWWAuthenticateHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		expectedHeader     string
		config             Config
		expectedStatusCode int
	}{
		{
			name: "default config on failure",
			config: Config{
				Validator: func(_ fiber.Ctx, _ string) (bool, error) {
					return false, errors.New("validation failed")
				},
			},
			expectedHeader:     `Bearer realm="Restricted"`,
			expectedStatusCode: fiber.StatusUnauthorized,
		},
		{
			name: "custom realm on failure",
			config: Config{
				Validator: func(_ fiber.Ctx, _ string) (bool, error) {
					return false, errors.New("validation failed")
				},
				Realm: "My Custom Realm",
			},
			expectedHeader:     `Bearer realm="My Custom Realm"`,
			expectedStatusCode: fiber.StatusUnauthorized,
		},
		{
			name: "default header for non-auth-header extractor",
			config: Config{
				Validator: func(_ fiber.Ctx, _ string) (bool, error) {
					return false, errors.New("validation failed")
				},
				Extractor: extractors.FromQuery("api_key"),
			},
			expectedHeader:     `ApiKey realm="Restricted"`,
			expectedStatusCode: fiber.StatusUnauthorized,
		},
		{
			name: "no header on success",
			config: Config{
				Validator: func(_ fiber.Ctx, _ string) (bool, error) {
					return true, nil
				},
			},
			expectedHeader:     "",
			expectedStatusCode: fiber.StatusOK,
		},
		{
			name: "chained extractor with auth header",
			config: Config{
				Validator: func(_ fiber.Ctx, _ string) (bool, error) {
					return false, errors.New("validation failed")
				},
				Extractor: extractors.Chain(extractors.FromQuery("q"), extractors.FromAuthHeader("MyScheme")),
			},
			expectedHeader:     `MyScheme realm="Restricted"`,
			expectedStatusCode: fiber.StatusUnauthorized,
		},
		{
			name: "chained extractor without auth header",
			config: Config{
				Validator: func(_ fiber.Ctx, _ string) (bool, error) {
					return false, errors.New("validation failed")
				},
				Extractor: extractors.Chain(extractors.FromQuery("q"), extractors.FromCookie("c")),
			},
			expectedHeader:     `ApiKey realm="Restricted"`,
			expectedStatusCode: fiber.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(tt.config))
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("OK")
			})

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			// Provide a key for the default extractor to find
			if tt.config.Extractor.Extract == nil {
				req.Header.Set(fiber.HeaderAuthorization, "Bearer somekey")
			}

			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatusCode, resp.StatusCode)
			assert.Equal(t, tt.expectedHeader, resp.Header.Get(fiber.HeaderWWWAuthenticate))
		})
	}
}

func Test_CustomChallenge(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Extractor: extractors.FromQuery("api_key"),
		Validator: func(_ fiber.Ctx, _ string) (bool, error) {
			return false, errors.New("invalid")
		},
		Challenge: `ApiKey realm="Restricted"`,
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	res, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, `ApiKey realm="Restricted"`, res.Header.Get("WWW-Authenticate"))
}

func Test_BearerErrorFields(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Validator: func(_ fiber.Ctx, _ string) (bool, error) {
			return false, errors.New("invalid")
		},
		Error:            "invalid_token",
		ErrorDescription: "token expired",
		ErrorURI:         "https://example.com",
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add("Authorization", "Bearer something")
	res, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, `Bearer realm="Restricted", error="invalid_token", error_description="token expired", error_uri="https://example.com"`, res.Header.Get("WWW-Authenticate"))
}

func Test_BearerErrorURIOnly(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Validator: func(_ fiber.Ctx, _ string) (bool, error) {
			return false, errors.New("invalid")
		},
		Error:    "invalid_token",
		ErrorURI: "https://example.com/docs",
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add("Authorization", "Bearer something")
	res, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, `Bearer realm="Restricted", error="invalid_token", error_uri="https://example.com/docs"`, res.Header.Get("WWW-Authenticate"))
}

func Test_BearerInsufficientScope(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Validator: func(_ fiber.Ctx, _ string) (bool, error) {
			return false, errors.New("invalid")
		},
		Error: ErrorInsufficientScope,
		Scope: "read",
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add("Authorization", "Bearer something")
	res, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, `Bearer realm="Restricted", error="insufficient_scope", scope="read"`, res.Header.Get("WWW-Authenticate"))
}

func Test_ScopeValidation(t *testing.T) {
	require.PanicsWithValue(t, "fiber: keyauth scope requires insufficient_scope error", func() {
		New(Config{
			Validator: func(_ fiber.Ctx, _ string) (bool, error) { return true, nil },
			Scope:     "foo",
		})
	})

	require.PanicsWithValue(t, "fiber: keyauth insufficient_scope requires scope", func() {
		New(Config{
			Validator: func(_ fiber.Ctx, _ string) (bool, error) { return true, nil },
			Error:     ErrorInsufficientScope,
		})
	})

	require.PanicsWithValue(t, "fiber: keyauth scope contains invalid token", func() {
		New(Config{
			Validator: func(_ fiber.Ctx, _ string) (bool, error) { return true, nil },
			Error:     ErrorInsufficientScope,
			Scope:     "read \"write\"",
		})
	})

	require.NotPanics(t, func() {
		New(Config{
			Validator: func(_ fiber.Ctx, _ string) (bool, error) { return true, nil },
			Error:     ErrorInsufficientScope,
			Scope:     "read write:all",
		})
	})
}

func Test_WWWAuthenticateOnlyOn401(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Validator: func(_ fiber.Ctx, _ string) (bool, error) {
			return false, errors.New("invalid")
		},
		ErrorHandler: func(c fiber.Ctx, _ error) error {
			return c.Status(fiber.StatusForbidden).SendString("forbidden")
		},
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add("Authorization", "Bearer bad")
	res, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	require.Empty(t, res.Header.Get("WWW-Authenticate"))
}

func Test_DefaultChallengeForNonAuthExtractor(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Extractor: extractors.FromQuery("api_key"),
		Validator: func(_ fiber.Ctx, _ string) (bool, error) { return false, ErrMissingOrMalformedAPIKey },
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	res, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, `ApiKey realm="Restricted"`, res.Header.Get(fiber.HeaderWWWAuthenticate))
}

func Test_MultipleWWWAuthenticateChallenges(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Extractor: extractors.Chain(
			extractors.FromAuthHeader("Bearer"),
			extractors.FromAuthHeader("ApiKey"),
		),
		Validator: func(_ fiber.Ctx, _ string) (bool, error) { return false, errors.New("invalid") },
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	res, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, `Bearer realm="Restricted", ApiKey realm="Restricted"`, res.Header.Get(fiber.HeaderWWWAuthenticate))
}

func Test_ProxyAuthenticateHeader(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Validator: func(_ fiber.Ctx, _ string) (bool, error) { return false, errors.New("invalid") },
		ErrorHandler: func(c fiber.Ctx, _ error) error {
			return c.Status(fiber.StatusProxyAuthRequired).SendString("proxy auth")
		},
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add("Authorization", "Bearer bad")
	res, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusProxyAuthRequired, res.StatusCode)
	require.Equal(t, `Bearer realm="Restricted"`, res.Header.Get(fiber.HeaderProxyAuthenticate))
	require.Empty(t, res.Header.Get(fiber.HeaderWWWAuthenticate))
}

func Test_New_InvalidErrorToken(t *testing.T) {
	assert.Panics(t, func() {
		New(Config{
			Validator: func(_ fiber.Ctx, _ string) (bool, error) { return true, nil },
			Error:     "unsupported",
		})
	})
}

func Test_New_ErrorDescriptionRequiresError(t *testing.T) {
	assert.Panics(t, func() {
		New(Config{
			Validator:        func(_ fiber.Ctx, _ string) (bool, error) { return true, nil },
			ErrorDescription: "desc",
		})
	})
}

func Test_New_ErrorURIRequiresError(t *testing.T) {
	assert.PanicsWithValue(t, "fiber: keyauth error_uri requires error", func() {
		New(Config{
			Validator: func(_ fiber.Ctx, _ string) (bool, error) { return true, nil },
			ErrorURI:  "https://example.com/docs",
		})
	})
}

func Test_New_ErrorURIAbsolute(t *testing.T) {
	assert.PanicsWithValue(t, "fiber: keyauth error_uri must be absolute", func() {
		New(Config{
			Validator: func(_ fiber.Ctx, _ string) (bool, error) { return true, nil },
			Error:     "invalid_token",
			ErrorURI:  "/docs",
		})
	})
}
