package keyauth

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	"github.com/gofiber/fiber/v3/internal/redact"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

const CorrectKey = "correct-token_123./~+"

var testConfig = fiber.TestConfig{Timeout: 0}

const (
	paramExtractorName      = "param"
	formExtractorName       = "form"
	queryExtractorName      = "query"
	headerExtractorName     = "header"
	authHeaderExtractorName = "authHeader"
	cookieExtractorName     = "cookie"
)

func Test_AuthSources(t *testing.T) {
	t.Parallel()
	testSources := []string{headerExtractorName, authHeaderExtractorName, cookieExtractorName, queryExtractorName, paramExtractorName, formExtractorName}

	tests := []struct {
		authTokenName string
		description   string
		APIKey        string
		expectedBody  string
		expectedCode  int
	}{
		{"access_token", "auth with correct key", CorrectKey, "Success!", 200},
		// 404 in case of param authentication
		{"access_token", "auth with no key", "", ErrMissingOrMalformedAPIKey.Error(), 401},
		{"access_token", "auth with wrong key", "WRONGKEY", ErrMissingOrMalformedAPIKey.Error(), 401},
	}

	for _, authSource := range testSources {
		t.Run(authSource, func(t *testing.T) {
			t.Parallel()
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

				handler := func(c fiber.Ctx) error { return c.SendString("Success!") }

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

				res := apptest.Do(t, app, req, testConfig)
				body := apptest.Body(t, res)

				expectedCode := test.expectedCode
				expectedBody := test.expectedBody
				if authSource == paramExtractorName && testKey == "" {
					expectedCode = 404
					expectedBody = "Not Found"
				}
				require.Equal(t, expectedCode, res.StatusCode, test.description)
				require.Equal(t, expectedBody, body, test.description)
			}
		})
	}
}

// validateCorrectKey accepts CorrectKey and refuses every other key with
// ErrMissingOrMalformedAPIKey.
func validateCorrectKey(_ fiber.Ctx, key string) (bool, error) {
	if key == CorrectKey {
		return true, nil
	}
	return false, ErrMissingOrMalformedAPIKey
}

// rejectAllKeys refuses every key with ErrMissingOrMalformedAPIKey.
func rejectAllKeys(fiber.Ctx, string) (bool, error) {
	return false, ErrMissingOrMalformedAPIKey
}

// bearerCorrectKey authenticates a request with CorrectKey in the Bearer scheme.
var bearerCorrectKey = apptest.WithHeader(fiber.HeaderAuthorization, "Bearer "+CorrectKey)

func TestMultipleKeyLookup(t *testing.T) {
	t.Parallel()
	const success = "Success!"

	app := fiber.New()
	app.Use(New(Config{
		Extractor: extractors.Chain(
			extractors.FromAuthHeader("Bearer"),
			extractors.FromHeader("key"),
			extractors.FromCookie("key"),
			extractors.FromQuery("key"),
		),
		Validator: func(_ fiber.Ctx, key string) (bool, error) {
			if key == CorrectKey {
				return true, nil
			}
			return false, errors.New("invalid key")
		},
	}))
	app.Get("/foo", func(c fiber.Ctx) error { return c.SendString(success) })

	apptest.ExpectBody(t, app, "/foo?key="+url.QueryEscape(CorrectKey), fiber.StatusOK, success)
	// A second request without the key is refused.
	apptest.ExpectBody(t, app, "/foo", fiber.StatusUnauthorized, ErrMissingOrMalformedAPIKey.Error())
}

func Test_MultipleKeyAuth(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// set up keyauth for /auth1 and /auth2, each with a different password
	for _, auth := range []struct{ route, password string }{{"/auth1", "password1"}, {"/auth2", "password2"}} {
		app.Use(New(Config{
			Next:      func(c fiber.Ctx) bool { return c.Path() != auth.route },
			Extractor: extractors.FromAuthHeader("Bearer"),
			Validator: func(_ fiber.Ctx, key string) (bool, error) {
				if key == auth.password {
					return true, nil
				}
				return false, errors.New("invalid key")
			},
		}))
	}

	app.Get("/", func(c fiber.Ctx) error { return c.SendString("No auth needed!") })
	app.Get("/auth1", func(c fiber.Ctx) error { return c.SendString("Successfully authenticated for auth1!") })
	app.Get("/auth2", func(c fiber.Ctx) error { return c.SendString("Successfully authenticated for auth2!") })

	tests := []struct {
		route        string
		description  string
		APIKey       string
		expectedBody string
		expectedCode int
	}{
		// No auth needed for /
		{"/", "No password needed", "", "No auth needed!", 200},
		// auth needed for auth1
		{"/auth1", "Normal Authentication Case", "password1", "Successfully authenticated for auth1!", 200},
		{"/auth1", "Wrong API Key", "WRONG KEY", ErrMissingOrMalformedAPIKey.Error(), 401},
		{"/auth1", "Wrong API Key", "", ErrMissingOrMalformedAPIKey.Error(), 401}, // NO KEY
		// Auth 2 has a different password
		{"/auth2", "Normal Authentication Case for auth2", "password2", "Successfully authenticated for auth2!", 200},
		{"/auth2", "Wrong API Key", "WRONG KEY", ErrMissingOrMalformedAPIKey.Error(), 401},
		{"/auth2", "Wrong API Key", "", ErrMissingOrMalformedAPIKey.Error(), 401}, // NO KEY
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()
			authorization := ""
			if test.APIKey != "" {
				authorization = "Bearer " + test.APIKey
			}
			req := apptest.Req(fiber.MethodGet, test.route, apptest.WithHeaderIf(fiber.HeaderAuthorization, authorization))
			apptest.ExpectBody(t, app, req, test.expectedCode, test.expectedBody)
		})
	}
}

func Test_CustomSuccessAndFailureHandlers(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		SuccessHandler: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusOK).SendString("API key is valid and request was handled by custom success handler")
		},
		ErrorHandler: func(c fiber.Ctx, _ error) error {
			return c.Status(fiber.StatusUnauthorized).SendString("API key is invalid and request was handled by custom error handler")
		},
		Validator: validateCorrectKey,
	}))

	app.Get("/", func(_ fiber.Ctx) error {
		t.Error("Test handler should not be called")
		return nil
	})

	apptest.ExpectBody(t, app, "/", http.StatusUnauthorized, "API key is invalid and request was handled by custom error handler")
	apptest.ExpectBody(t, app, apptest.Req(fiber.MethodGet, "/", bearerCorrectKey), http.StatusOK, "API key is valid and request was handled by custom success handler")
}

func Test_CustomNextFunc(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Next:      func(c fiber.Ctx) bool { return c.Path() == "/allowed" },
		Validator: validateCorrectKey,
	}))

	app.Get("/allowed", func(c fiber.Ctx) error {
		return c.SendString("API key is valid and request was allowed by custom filter")
	})
	app.Get("/not-allowed", func(c fiber.Ctx) error { return c.SendString("Should be protected") })

	apptest.ExpectBody(t, app, "/allowed", http.StatusOK, "API key is valid and request was allowed by custom filter")
	apptest.ExpectBody(t, app, "/not-allowed", http.StatusUnauthorized, ErrMissingOrMalformedAPIKey.Error())
	apptest.ExpectBody(t, app, apptest.Req(fiber.MethodGet, "/not-allowed", bearerCorrectKey), http.StatusOK, "Should be protected")
}

func Test_TokenFromContext_None(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error { return c.SendString(TokenFromContext(c)) })

	// Verify a "" is sent back if nothing sets the token on the context.
	apptest.ExpectBody(t, app, "/", http.StatusOK, "")
}

func Test_TokenFromContext(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Extractor: extractors.FromAuthHeader("Basic"), Validator: validateCorrectKey}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString(TokenFromContext(c)) })

	apptest.ExpectBody(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderAuthorization, "Basic "+CorrectKey)), http.StatusOK, CorrectKey)
}

func Test_LogTagsRedactToken(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	app := fiber.New()
	app.Use(New(Config{
		Extractor: extractors.FromAuthHeader("Bearer"),
		Validator: func(_ fiber.Ctx, key string) (bool, error) { return key == CorrectKey, nil },
	}))
	app.Use(logger.New(logger.Config{Format: "${api-key}", Stream: &buf}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", bearerCorrectKey), fiber.StatusOK)
	require.Equal(t, redact.Prefix(CorrectKey), buf.String())
}

// Test_LogContextTagRedactsToken runs serially because it mutates package-global
// default logger output and context format.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_LogContextTagRedactsToken(t *testing.T) {
	buf := loggertest.CaptureContextLog(t, "api-key=${api-key} ")

	app := fiber.New()
	app.Use(New(Config{
		Extractor: extractors.FromAuthHeader("Bearer"),
		Validator: func(_ fiber.Ctx, key string) (bool, error) { return key == CorrectKey, nil },
	}))
	app.Get("/", func(c fiber.Ctx) error {
		fiberlog.WithContext(c).Info("start")
		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", bearerCorrectKey), fiber.StatusOK)
	require.Contains(t, buf.String(), "[Info] api-key="+redact.Prefix(CorrectKey)+" start")
}

func Test_TokenFromContext_Types(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{PassLocalsToContext: true})
	app.Use(New(Config{Extractor: extractors.FromAuthHeader("Basic"), Validator: validateCorrectKey}))

	app.Get("/", func(c fiber.Ctx) error {
		require.Equal(t, CorrectKey, TokenFromContext(c))
		customCtx, ok := c.(fiber.CustomCtx)
		require.True(t, ok)
		require.Equal(t, CorrectKey, TokenFromContext(customCtx))
		require.Equal(t, CorrectKey, TokenFromContext(c.RequestCtx()))
		require.Equal(t, CorrectKey, TokenFromContext(c.Context()))
		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderAuthorization, "Basic "+CorrectKey)), fiber.StatusOK)
}

// Test_HeaderScheme covers how the Authorization header is split into a scheme
// and the key, with the default Bearer extractor unless a row names another.
func Test_HeaderScheme(t *testing.T) {
	t.Parallel()
	tests := []struct {
		validator     func(fiber.Ctx, string) (bool, error)
		name          string
		authorization string
		body          string
		challenge     string
		extractor     extractors.Extractor
		status        int
	}{
		{name: "Token scheme with the key", extractor: extractors.FromAuthHeader("Token"), validator: validateCorrectKey, authorization: "Token " + CorrectKey, status: http.StatusOK, body: "OK"},
		{name: "Basic scheme without a key", extractor: extractors.FromAuthHeader("Basic"), validator: validateCorrectKey, status: http.StatusUnauthorized, body: ErrMissingOrMalformedAPIKey.Error()},
		{name: "Basic scheme with the key", extractor: extractors.FromAuthHeader("Basic"), validator: validateCorrectKey, authorization: "Basic " + CorrectKey, status: http.StatusOK, body: "OK"},
		{name: "scheme is case insensitive", validator: validateCorrectKey, authorization: "bearer " + CorrectKey, status: http.StatusOK, body: "OK"},
		{name: "multiple spaces", validator: validateCorrectKey, authorization: "Bearer    " + CorrectKey, status: http.StatusUnauthorized, body: ErrMissingOrMalformedAPIKey.Error()},
		{name: "missing space", validator: rejectAllKeys, authorization: "Bearer" + CorrectKey, status: http.StatusUnauthorized, body: ErrMissingOrMalformedAPIKey.Error()},
		{name: "no token", validator: rejectAllKeys, authorization: "Bearer ", status: http.StatusUnauthorized, body: ErrMissingOrMalformedAPIKey.Error()},
		// No space between "Bearer" and token
		{name: "no separator", validator: rejectAllKeys, authorization: "BearerTokenWithoutSpace", status: http.StatusUnauthorized, body: ErrMissingOrMalformedAPIKey.Error()},
		// Authorization header with scheme followed by only spaces/tabs (no actual token)
		{name: "empty token after trim", validator: rejectAllKeys, authorization: "Bearer \t  \t ", status: http.StatusUnauthorized, body: ErrMissingOrMalformedAPIKey.Error()},
		{
			name:          "invalid key",
			validator:     func(fiber.Ctx, string) (bool, error) { return false, errors.New("invalid") },
			authorization: "Bearer " + CorrectKey,
			status:        http.StatusUnauthorized,
			body:          ErrMissingOrMalformedAPIKey.Error(),
			challenge:     `Bearer realm="Restricted"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{Extractor: tc.extractor, Validator: tc.validator}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

			req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeaderIf(fiber.HeaderAuthorization, tc.authorization))
			resp := apptest.ExpectBody(t, app, req, tc.status, tc.body)
			if tc.challenge != "" {
				require.Equal(t, tc.challenge, resp.Header.Get(fiber.HeaderWWWAuthenticate))
			}
		})
	}
}

// Test_Challenge covers the WWW-Authenticate and Proxy-Authenticate headers a
// refused request is answered with.
func Test_Challenge(t *testing.T) {
	t.Parallel()
	failing := func(fiber.Ctx, string) (bool, error) { return false, errors.New("validation failed") }
	invalid := func(fiber.Ctx, string) (bool, error) { return false, errors.New("invalid") }
	valid := func(fiber.Ctx, string) (bool, error) { return true, nil }
	// answerWith refuses every request with the given error.
	answerWith := func(err error) fiber.ErrorHandler { return func(fiber.Ctx, error) error { return err } }
	// answerStatus refuses every request with the given status and text.
	answerStatus := func(status int, text string) fiber.ErrorHandler {
		return func(c fiber.Ctx, _ error) error { return c.Status(status).SendString(text) }
	}
	const bearer, apiKey = `Bearer realm="Restricted"`, `ApiKey realm="Restricted"`
	queryThenScheme := extractors.Chain(extractors.FromQuery("q"), extractors.FromAuthHeader("MyScheme"))
	queryThenCookie := extractors.Chain(extractors.FromQuery("q"), extractors.FromCookie("c"))
	twoSchemes := extractors.Chain(extractors.FromAuthHeader("Bearer"), extractors.FromAuthHeader("ApiKey"))

	tests := []struct {
		name          string
		authorization string
		www           string // the WWW-Authenticate header of the answer, "" for none
		proxy         string // the Proxy-Authenticate header of the answer, "" for none
		config        Config
		status        int
	}{
		{
			name: "default config on failure", config: Config{Validator: failing}, authorization: "Bearer somekey",
			status: fiber.StatusUnauthorized, www: bearer,
		},
		{
			name: "custom realm on failure", config: Config{Validator: failing, Realm: "My Custom Realm"},
			authorization: "Bearer somekey", status: fiber.StatusUnauthorized, www: `Bearer realm="My Custom Realm"`,
		},
		{
			name: "default header for non-auth-header extractor", status: fiber.StatusUnauthorized, www: apiKey,
			config: Config{Validator: failing, Extractor: extractors.FromQuery("api_key")},
		},
		{
			name: "no header on success", config: Config{Validator: valid}, authorization: "Bearer somekey",
			status: fiber.StatusOK,
		},
		{
			name: "chained extractor with auth header", status: fiber.StatusUnauthorized,
			config: Config{Validator: failing, Extractor: queryThenScheme}, www: `MyScheme realm="Restricted"`,
		},
		{
			name: "chained extractor without auth header", status: fiber.StatusUnauthorized, www: apiKey,
			config: Config{Validator: failing, Extractor: queryThenCookie},
		},
		{
			name: "default error handler without a key", config: Config{Validator: rejectAllKeys},
			status: http.StatusUnauthorized, www: bearer,
		},
		{
			name: "custom challenge", status: http.StatusUnauthorized, www: apiKey,
			config: Config{
				Validator: invalid, Extractor: extractors.FromQuery("api_key"), Challenge: `ApiKey realm="Restricted"`,
			},
		},
		{
			name: "bearer error fields", authorization: "Bearer something", status: http.StatusUnauthorized,
			config: Config{
				Validator: invalid, Error: "invalid_token",
				ErrorDescription: "token expired", ErrorURI: "https://example.com",
			},
			www: bearer + `, error="invalid_token", error_description="token expired", error_uri="https://example.com"`,
		},
		{
			name: "bearer error URI only", authorization: "Bearer something", status: http.StatusUnauthorized,
			config: Config{Validator: invalid, Error: "invalid_token", ErrorURI: "https://example.com/docs"},
			www:    bearer + `, error="invalid_token", error_uri="https://example.com/docs"`,
		},
		{
			name: "bearer insufficient scope", authorization: "Bearer something", status: http.StatusUnauthorized,
			config: Config{Validator: invalid, Error: ErrorInsufficientScope, Scope: "read"},
			www:    bearer + `, error="insufficient_scope", scope="read"`,
		},
		{
			name: "no challenge unless the answer is a 401", authorization: "Bearer bad", status: http.StatusForbidden,
			config: Config{Validator: invalid, ErrorHandler: answerStatus(fiber.StatusForbidden, "forbidden")},
		},
		{
			name: "default challenge for a non-auth extractor", status: http.StatusUnauthorized, www: apiKey,
			config: Config{Validator: rejectAllKeys, Extractor: extractors.FromQuery("api_key")},
		},
		{
			name: "one challenge per auth scheme", status: http.StatusUnauthorized, www: bearer + ", " + apiKey,
			config: Config{Validator: invalid, Extractor: twoSchemes},
		},
		{
			name: "proxy authentication required", authorization: "Bearer bad",
			status: http.StatusProxyAuthRequired, proxy: bearer,
			config: Config{Validator: invalid, ErrorHandler: answerStatus(fiber.StatusProxyAuthRequired, "proxy auth")},
		},
		// The challenge must be set when the ErrorHandler returns the *fiber.Error instead of writing it.
		{
			name: "returned unauthorized error", status: fiber.StatusUnauthorized, www: bearer,
			config: Config{Validator: rejectAllKeys, ErrorHandler: answerWith(fiber.ErrUnauthorized)},
		},
		{
			name: "returned proxy auth required error", status: fiber.StatusProxyAuthRequired, proxy: bearer,
			config: Config{Validator: rejectAllKeys, ErrorHandler: answerWith(fiber.ErrProxyAuthRequired)},
		},
		// Both default challenges use HTTP quoted-string escaping rather than Go syntax.
		{
			name: "default ApiKey challenge escaping", status: http.StatusUnauthorized,
			config: Config{Extractor: extractors.FromQuery("key"), Realm: "area \"A\"\\\x01\n", Validator: invalid},
			www:    `ApiKey realm="area \"A\"\\%01\n"`,
		},
		{
			name: "Bearer parameters escaping", authorization: "Bearer bad", status: http.StatusUnauthorized,
			config: Config{
				Realm:            "area \"A\"\\\x01",
				Error:            ErrorInvalidToken,
				ErrorDescription: "bad \"token\"\\\x02\n",
				ErrorURI:         `https://example.com/docs?q="x"`,
				Validator:        invalid,
			},
			www: `Bearer realm="area \"A\"\\%01", error="invalid_token", ` +
				`error_description="bad \"token\"\\%02\n", error_uri="https://example.com/docs?q=\"x\""`,
		},
	}

	for i := range tests {
		tc := &tests[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(tc.config))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })

			resp := apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHeaderIf(fiber.HeaderAuthorization, tc.authorization)), tc.status)
			require.Equal(t, tc.www, resp.Header.Get(fiber.HeaderWWWAuthenticate), fiber.HeaderWWWAuthenticate)
			require.Equal(t, tc.proxy, resp.Header.Get(fiber.HeaderProxyAuthenticate), fiber.HeaderProxyAuthenticate)
		})
	}
}

// Test_New_InvalidConfig covers the configurations New refuses with a panic.
func Test_New_InvalidConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		wantPanic string
		config    Config
	}{
		{name: "scope without insufficient_scope", config: Config{Scope: "foo"}, wantPanic: "fiber: keyauth scope requires insufficient_scope error"},
		{name: "insufficient_scope without scope", config: Config{Error: ErrorInsufficientScope}, wantPanic: "fiber: keyauth insufficient_scope requires scope"},
		{name: "scope with an invalid token", config: Config{Error: ErrorInsufficientScope, Scope: "read \"write\""}, wantPanic: "fiber: keyauth scope contains invalid token"},
		{name: "valid scope", config: Config{Error: ErrorInsufficientScope, Scope: "read write:all"}},
		{name: "unsupported error token", config: Config{Error: "unsupported"}, wantPanic: "fiber: keyauth unsupported error token"},
		{name: "error description without error", config: Config{ErrorDescription: "desc"}, wantPanic: "fiber: keyauth error_description requires error"},
		{name: "error URI without error", config: Config{ErrorURI: "https://example.com/docs"}, wantPanic: "fiber: keyauth error_uri requires error"},
		{name: "relative error URI", config: Config{Error: "invalid_token", ErrorURI: "/docs"}, wantPanic: "fiber: keyauth error_uri must be absolute"},
	}

	for i := range tests {
		tc := &tests[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := tc.config
			cfg.Validator = func(fiber.Ctx, string) (bool, error) { return true, nil }
			if tc.wantPanic == "" {
				require.NotPanics(t, func() { New(cfg) })
				return
			}
			require.PanicsWithValue(t, tc.wantPanic, func() { New(cfg) })
		})
	}
}

// benchKey is a JWT-sized token68 credential, the shape a Bearer token has in
// practice.
const benchKey = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9P"

// benchValidator compares in constant time, as a real deployment must, but
// without hashing, so the numbers are the middleware and not a digest.
func benchValidator(_ fiber.Ctx, key string) (bool, error) {
	return subtle.ConstantTimeCompare([]byte(key), []byte(benchKey)) == 1, nil
}

// benchApp registers the middleware with cfg on a route that answers 418. cfg
// is a pointer because Config is 200 bytes and gocritic refuses the copy.
func benchApp(cfg *Config) fasthttp.RequestHandler {
	app := fiber.New()
	app.Use(New(*cfg))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })
	return app.Handler()
}

func Benchmark_KeyAuth_Bearer(b *testing.B) {
	h := benchApp(&Config{Validator: benchValidator})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.Set(fiber.HeaderAuthorization, "Bearer "+benchKey)

	b.ReportAllocs()

	for b.Loop() {
		// What fasthttp does between requests. Without it the extractor's
		// request-local state is installed once and reused, and the per-request
		// cost of installing and recycling it is not in the number.
		ctx.ResetUserValues()
		h(ctx)
	}

	require.Equal(b, fiber.StatusTeapot, ctx.Response.Header.StatusCode())
}

// The credential is in the last source, so the walk pays for both misses.
func Benchmark_KeyAuth_Chain(b *testing.B) {
	h := benchApp(&Config{
		Validator: benchValidator,
		Extractor: extractors.Chain(
			extractors.FromAuthHeader("Bearer"),
			extractors.FromHeader("X-API-Key"),
			extractors.FromQuery("api_key"),
		),
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/?api_key=" + benchKey)

	b.ReportAllocs()

	for b.Loop() {
		ctx.ResetUserValues()
		h(ctx)
	}

	require.Equal(b, fiber.StatusTeapot, ctx.Response.Header.StatusCode())
}
