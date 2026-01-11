package fiber_test

import (
	"bytes"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
	"github.com/gofiber/fiber/v3/middleware/cache"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3/middleware/encryptcookie"
	"github.com/gofiber/fiber/v3/middleware/envvar"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/keyauth"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

type integrationCustomCtx struct {
	*fiber.DefaultCtx
}

func newIntegrationCustomCtx(app *fiber.App) fiber.CustomCtx {
	return &integrationCustomCtx{DefaultCtx: fiber.NewDefaultCtx(app)}
}

func performOversizedRequest(t *testing.T, app *fiber.App, configure func(req *fasthttp.Request)) *fasthttp.Response {
	t.Helper()

	ln := fasthttputil.NewInmemoryListener()
	errCh := make(chan error, 1)

	go func() {
		errCh <- app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})
	}()

	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		if err := <-errCh; err != nil && !errors.Is(err, net.ErrClosed) {
			require.NoError(t, err)
		}
	})

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err != nil {
			return false
		}
		if err := conn.Close(); err != nil {
			return false
		}
		return true
	}, time.Second, 10*time.Millisecond)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()

	req.SetRequestURI("http://example.com/")
	req.Header.SetMethod(fiber.MethodPost)
	req.Header.Set(fiber.HeaderOrigin, "https://example.com")
	req.SetBody(bytes.Repeat([]byte{'a'}, 32))
	if configure != nil {
		configure(req)
	}

	client := fasthttp.Client{
		Dial: func(string) (net.Conn, error) {
			return ln.Dial()
		},
	}

	require.NoError(t, client.Do(req, resp))

	respCopy := fasthttp.AcquireResponse()
	resp.CopyTo(respCopy)

	fasthttp.ReleaseRequest(req)
	fasthttp.ReleaseResponse(resp)

	t.Cleanup(func() {
		fasthttp.ReleaseResponse(respCopy)
	})

	return respCopy
}

var integrationEncryptCookieKey = encryptcookie.GenerateKey(32)

// middlewareCombinationTestCase describes a middleware stack that should keep its
// headers intact even when the default error handler runs. Keeping it as a named
// type (instead of an inline struct) makes the massive table below easier to
// scan and extend.
//
//nolint:govet // field alignment is secondary to readability for this test table
type middlewareCombinationTestCase struct { // betteralign:ignore - readability takes priority in tests
	name             string
	setup            func(app *fiber.App)
	configureRequest func(req *fasthttp.Request)
	handler          func(c fiber.Ctx) error
	assertions       func(t *testing.T, resp *fasthttp.Response)
	expectedStatus   int
}

func (tc middlewareCombinationTestCase) statusOrDefault() int {
	if tc.expectedStatus == 0 {
		return fiber.StatusInternalServerError
	}
	return tc.expectedStatus
}

func (tc middlewareCombinationTestCase) handlerOrDefault() func(fiber.Ctx) error {
	if tc.handler != nil {
		return tc.handler
	}

	return func(fiber.Ctx) error {
		return fiber.NewError(fiber.StatusInternalServerError, "middleware combination failure")
	}
}

func Test_Integration_App_ServerErrorHandler_MiddlewareCombinationHeaders(t *testing.T) {
	t.Parallel()

	// This integration suite exercises representative middleware stacks to ensure their
	// response headers survive after Fiber's default error handler emits a failure.

	const (
		// Origins used by the CORS stacks in this suite.
		corsHelmetOrigin    = "https://cors-and-helmet.example"
		corsRequestIDOrigin = "https://cors-and-requestid.example"
		corsCSRForigin      = "https://cors-and-csrf.example"
		corsCacheOrigin     = "https://cors-and-cache.example"
		corsSessionOrigin   = "https://cors-and-session.example"
		corsHelmetRequestID = "https://cors-helmet-requestid.example"

		csrfCookieName      = "combo-csrf"
		generatedRequestID  = "generated-combo-request-id"
		helmetLimiterMax    = 7
		helmetLimiterReset  = 60
		requestIDHeader     = "combo-request-id"
		csrfTokenValue      = "csrf-token"
		encryptedCookieName = "combo-encrypted"
		encryptedCookieVal  = "unencrypted"
		envvarAllowHeader   = fiber.MethodGet + ", " + fiber.MethodHead
		basicRealm          = "combo-basic"
		keyAuthRealm        = "combo-key"
		keyAuthErrorDesc    = "missing-key"
	)

	// Each entry wires up a different middleware stack so we can ensure response mutations
	// survive the hop through the default error handler.
	testCases := []middlewareCombinationTestCase{
		// --- CORS-focused stacks keep cross-origin metadata on error responses.
		{
			name: "cors+helmet",
			setup: func(app *fiber.App) {
				app.Use(cors.New(cors.Config{AllowOrigins: []string{corsHelmetOrigin}}))
				app.Use(helmet.New())
			},
			configureRequest: func(req *fasthttp.Request) {
				req.Header.Set(fiber.HeaderOrigin, corsHelmetOrigin)
			},
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, corsHelmetOrigin, string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
				require.Equal(t, "same-origin", string(resp.Header.Peek("Cross-Origin-Opener-Policy")))
				require.Equal(t, "same-origin", string(resp.Header.Peek("Cross-Origin-Resource-Policy")))
				require.Equal(t, "require-corp", string(resp.Header.Peek("Cross-Origin-Embedder-Policy")))
			},
		},
		{
			name: "cors+requestid",
			setup: func(app *fiber.App) {
				app.Use(cors.New(cors.Config{AllowOrigins: []string{corsRequestIDOrigin}}))
				app.Use(requestid.New())
			},
			configureRequest: func(req *fasthttp.Request) {
				req.Header.Set(fiber.HeaderOrigin, corsRequestIDOrigin)
				req.Header.Set("X-Request-ID", requestIDHeader)
			},
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, corsRequestIDOrigin, string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
				require.Equal(t, requestIDHeader, string(resp.Header.Peek("X-Request-ID")))
			},
		},
		{
			name: "cors+helmet+requestid",
			setup: func(app *fiber.App) {
				app.Use(cors.New(cors.Config{AllowOrigins: []string{corsHelmetRequestID}}))
				app.Use(helmet.New())
				app.Use(requestid.New(requestid.Config{
					Generator: func() string {
						return generatedRequestID
					},
				}))
			},
			configureRequest: func(req *fasthttp.Request) {
				req.Header.Set(fiber.HeaderOrigin, corsHelmetRequestID)
			},
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, corsHelmetRequestID, string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
				require.Equal(t, generatedRequestID, string(resp.Header.Peek("X-Request-ID")))
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
			},
		},
		{
			name: "cors+cache",
			setup: func(app *fiber.App) {
				app.Use(cors.New(cors.Config{AllowOrigins: []string{corsCacheOrigin}}))
				app.Use(cache.New())
				// Cache needs the default error handler to execute so it can emit X-Cache on failures.
				app.Use(func(c fiber.Ctx) error {
					if err := c.Next(); err != nil {
						if handlerErr := app.Config().ErrorHandler(c, err); handlerErr != nil {
							return handlerErr
						}
						c.Set(fiber.HeaderCacheControl, "no-store")
						return nil
					}
					c.Set(fiber.HeaderCacheControl, "no-store")
					return nil
				})
			},
			configureRequest: func(req *fasthttp.Request) {
				req.Header.Set(fiber.HeaderOrigin, corsCacheOrigin)
				req.Header.SetMethod(fiber.MethodGet)
			},
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, corsCacheOrigin, string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
				require.Equal(t, "unreachable", string(resp.Header.Peek("X-Cache")))
				require.Equal(t, "no-store", string(resp.Header.Peek(fiber.HeaderCacheControl)))
			},
		},
		{
			name: "cors+session",
			setup: func(app *fiber.App) {
				app.Use(cors.New(cors.Config{
					AllowOrigins:     []string{corsSessionOrigin},
					AllowCredentials: true,
				}))
				app.Use(session.New())
				app.Use(func(c fiber.Ctx) error {
					if sm := session.FromContext(c); sm != nil {
						sm.Set("cors-session", "enabled")
					}
					return c.Next()
				})
			},
			configureRequest: func(req *fasthttp.Request) {
				req.Header.Set(fiber.HeaderOrigin, corsSessionOrigin)
			},
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, corsSessionOrigin, string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
				require.Equal(t, "true", string(resp.Header.Peek(fiber.HeaderAccessControlAllowCredentials)))
				require.Contains(t, string(resp.Header.Peek(fiber.HeaderSetCookie)), "session_id=")
			},
		},
		{
			name: "helmet+encryptcookie",
			setup: func(app *fiber.App) {
				app.Use(helmet.New())
				app.Use(encryptcookie.New(encryptcookie.Config{Key: integrationEncryptCookieKey}))
				app.Use(func(c fiber.Ctx) error {
					c.Cookie(&fiber.Cookie{Name: encryptedCookieName, Value: encryptedCookieVal})
					return c.Next()
				})
			},
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
				cookieHeader := string(resp.Header.Peek(fiber.HeaderSetCookie))
				require.Contains(t, cookieHeader, encryptedCookieName+"=")
				require.NotContains(t, cookieHeader, encryptedCookieVal)
			},
		},
		// --- Helmet anchored stacks validate security headers across other middleware.
		{
			name: "helmet+limiter",
			setup: func(app *fiber.App) {
				app.Use(helmet.New())
				app.Use(limiter.New(limiter.Config{
					Max:        helmetLimiterMax,
					Expiration: time.Duration(helmetLimiterReset) * time.Second,
					KeyGenerator: func(fiber.Ctx) string {
						return "helmet+limiter"
					},
				}))
			},
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
				require.Equal(t, strconv.Itoa(helmetLimiterMax), string(resp.Header.Peek("X-RateLimit-Limit")))
				require.Equal(t, strconv.Itoa(helmetLimiterMax-1), string(resp.Header.Peek("X-RateLimit-Remaining")))
				require.Equal(t, strconv.Itoa(helmetLimiterReset), string(resp.Header.Peek("X-RateLimit-Reset")))
			},
		},
		{
			name: "cors+csrf",
			setup: func(app *fiber.App) {
				app.Use(cors.New(cors.Config{
					AllowOrigins:     []string{corsCSRForigin},
					AllowCredentials: true,
				}))
				app.Use(csrf.New(csrf.Config{
					CookieName:   csrfCookieName,
					KeyGenerator: func() string { return csrfTokenValue },
				}))
			},
			configureRequest: func(req *fasthttp.Request) {
				req.Header.SetMethod(fiber.MethodGet)
				req.Header.Set(fiber.HeaderOrigin, corsCSRForigin)
			},
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, corsCSRForigin, string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
				require.Equal(t, "true", string(resp.Header.Peek(fiber.HeaderAccessControlAllowCredentials)))
				require.Contains(t, string(resp.Header.Peek(fiber.HeaderSetCookie)), csrfCookieName+"="+csrfTokenValue)
			},
		},
		{
			name: "helmet+session",
			setup: func(app *fiber.App) {
				app.Use(helmet.New())
				app.Use(session.New())
				app.Use(func(c fiber.Ctx) error {
					if sm := session.FromContext(c); sm != nil {
						sm.Set("combo-session", "enabled")
					}
					return c.Next()
				})
			},
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
				require.Contains(t, string(resp.Header.Peek(fiber.HeaderSetCookie)), "session_id=")
			},
		},
		{
			name: "helmet+csrf",
			setup: func(app *fiber.App) {
				app.Use(helmet.New())
				app.Use(csrf.New(csrf.Config{
					CookieName:   csrfCookieName,
					KeyGenerator: func() string { return csrfTokenValue },
				}))
			},
			configureRequest: func(req *fasthttp.Request) {
				req.Header.SetMethod(fiber.MethodGet)
			},
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
				require.Contains(t, string(resp.Header.Peek(fiber.HeaderSetCookie)), csrfCookieName+"="+csrfTokenValue)
			},
		},
		{
			name: "helmet+envvar",
			setup: func(app *fiber.App) {
				app.Use(helmet.New())
				app.Use(envvar.New(envvar.Config{ExportVars: map[string]string{"COMBO_ENV": "configured"}}))
			},
			expectedStatus: fiber.StatusMethodNotAllowed,
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
				require.Equal(t, envvarAllowHeader, string(resp.Header.Peek(fiber.HeaderAllow)))
			},
		},
		{
			name: "helmet+basicauth",
			setup: func(app *fiber.App) {
				app.Use(helmet.New())
				app.Use(basicauth.New(basicauth.Config{
					Realm: basicRealm,
					Unauthorized: func(c fiber.Ctx) error {
						c.Set(fiber.HeaderWWWAuthenticate, "Basic realm=\""+basicRealm+"\", charset=\"UTF-8\"")
						c.Set(fiber.HeaderCacheControl, "no-store")
						c.Set(fiber.HeaderVary, fiber.HeaderAuthorization)
						c.Status(fiber.StatusUnauthorized)
						return fiber.ErrUnauthorized
					},
				}))
			},
			expectedStatus: fiber.StatusUnauthorized,
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
				require.Equal(t, "Basic realm=\""+basicRealm+"\", charset=\"UTF-8\"", string(resp.Header.Peek(fiber.HeaderWWWAuthenticate)))
				require.Equal(t, "no-store", string(resp.Header.Peek(fiber.HeaderCacheControl)))
				require.Equal(t, fiber.HeaderAuthorization, string(resp.Header.Peek(fiber.HeaderVary)))
			},
		},
		{
			name: "helmet+keyauth",
			setup: func(app *fiber.App) {
				app.Use(helmet.New())
				app.Use(keyauth.New(keyauth.Config{
					Realm:            keyAuthRealm,
					Error:            keyauth.ErrorInvalidToken,
					ErrorDescription: keyAuthErrorDesc,
					Validator: func(fiber.Ctx, string) (bool, error) {
						return false, nil
					},
					ErrorHandler: func(c fiber.Ctx, _ error) error {
						c.Status(fiber.StatusUnauthorized)
						return fiber.ErrUnauthorized
					},
				}))
			},
			expectedStatus: fiber.StatusUnauthorized,
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
				authenticate := string(resp.Header.Peek(fiber.HeaderWWWAuthenticate))
				require.Contains(t, authenticate, "Bearer realm=\""+keyAuthRealm+"\"")
				require.Contains(t, authenticate, "error=\""+keyauth.ErrorInvalidToken+"\"")
				require.Contains(t, authenticate, "error_description=\""+keyAuthErrorDesc+"\"")
			},
		},
		{
			name: "helmet+compress",
			setup: func(app *fiber.App) {
				app.Use(helmet.New())
				app.Use(compress.New())
				app.Use(func(c fiber.Ctx) error {
					if err := c.Next(); err != nil {
						if handlerErr := app.Config().ErrorHandler(c, err); handlerErr != nil {
							return handlerErr
						}
						// Inflate the error body so the compress middleware has something to work with.
						if body := c.Response().Body(); len(body) > 0 {
							c.Response().SetBodyString(strings.Repeat(string(body), 32))
						}
					}
					return nil
				})
			},
			configureRequest: func(req *fasthttp.Request) {
				req.Header.Set(fiber.HeaderAcceptEncoding, "gzip")
			},
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
				require.Equal(t, "gzip", string(resp.Header.Peek(fiber.HeaderContentEncoding)))
				require.Equal(t, fiber.HeaderAcceptEncoding, string(resp.Header.Peek(fiber.HeaderVary)))
			},
		},
		{
			name: "helmet+recover",
			setup: func(app *fiber.App) {
				app.Use(helmet.New())
				app.Use(recover.New())
			},
			handler: func(fiber.Ctx) error {
				panic("panic for recover middleware")
			},
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
				// Recover writes a plain-text body; ensure we still return content to clients while
				// keeping Helmet's security headers intact.
				require.Contains(t, string(resp.Body()), "panic for recover middleware")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			tc.setup(app)
			// Every stack shares the same route that always hits the default error handler so we
			// can verify which headers survive the error response. A few cases override the
			// handler to exercise panic recovery or other routes that still flow through the
			// default error path.
			app.All("/", tc.handlerOrDefault())

			resp := performOversizedRequest(t, app, tc.configureRequest)

			require.Equal(t, tc.statusOrDefault(), resp.StatusCode())
			tc.assertions(t, resp)
		})
	}
}

func Test_Integration_App_ServerErrorHandler_PreservesCORSHeadersOnBodyLimit(t *testing.T) {
	app := fiber.New(fiber.Config{BodyLimit: 16})
	app.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"https://example.com"},
		AllowCredentials: true,
		ExposeHeaders:    []string{"X-Request-ID"},
	}))
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp := performOversizedRequest(t, app, nil)

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Equal(t, "https://example.com", string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
	require.Equal(t, "true", string(resp.Header.Peek(fiber.HeaderAccessControlAllowCredentials)))
	require.Equal(t, "X-Request-ID", string(resp.Header.Peek(fiber.HeaderAccessControlExposeHeaders)))
	require.Equal(t, "Origin", string(resp.Header.Peek(fiber.HeaderVary)))
}

func Test_Integration_App_ServerErrorHandler_PreservesHelmetHeadersOnBodyLimit(t *testing.T) {
	app := fiber.New(fiber.Config{BodyLimit: 16})
	app.Use(helmet.New())
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp := performOversizedRequest(t, app, nil)

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
	require.Equal(t, "same-origin", string(resp.Header.Peek("Cross-Origin-Opener-Policy")))
	require.Equal(t, "same-origin", string(resp.Header.Peek("Cross-Origin-Resource-Policy")))
	require.Equal(t, "require-corp", string(resp.Header.Peek("Cross-Origin-Embedder-Policy")))
}

func Test_Integration_App_ServerErrorHandler_PreservesRequestID(t *testing.T) {
	const expectedRequestID = "integration-request-id"

	app := fiber.New(fiber.Config{BodyLimit: 16})
	app.Use(requestid.New())
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp := performOversizedRequest(t, app, func(req *fasthttp.Request) {
		req.Header.Set("X-Request-ID", expectedRequestID)
	})

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Equal(t, expectedRequestID, string(resp.Header.Peek("X-Request-ID")))
}

func Test_Integration_App_ServerErrorHandler_GroupMiddlewareChain(t *testing.T) {
	app := fiber.New(fiber.Config{BodyLimit: 16})
	app.Use(helmet.New())

	api := app.Group("/api")
	api.Use(requestid.New())
	api.Use(func(c fiber.Ctx) error {
		c.Set("X-Group-Middleware", "active")
		return c.Next()
	})
	api.Post("/resource", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp := performOversizedRequest(t, app, func(req *fasthttp.Request) {
		req.SetRequestURI("http://example.com/api/resource")
	})

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
	require.NotEmpty(t, resp.Header.Peek("X-Request-ID"))
	require.Equal(t, "active", string(resp.Header.Peek("X-Group-Middleware")))
}

func Test_Integration_App_ServerErrorHandler_RetainsHeadersFromSubsequentMiddleware(t *testing.T) {
	app := fiber.New(fiber.Config{BodyLimit: 8})
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Custom-Middleware", "ran")
		return c.Next()
	})
	app.Use(cors.New())
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp := performOversizedRequest(t, app, nil)

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Equal(t, "ran", string(resp.Header.Peek("X-Custom-Middleware")))
	require.Equal(t, "*", string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
}

func Test_Integration_App_ServerErrorHandler_WithCustomCtx(t *testing.T) {
	app := fiber.NewWithCustomCtx(newIntegrationCustomCtx, fiber.Config{BodyLimit: 16})
	app.Use(func(c fiber.Ctx) error {
		customCtx, ok := c.(*integrationCustomCtx)
		require.True(t, ok)
		customCtx.Set("X-Custom-Ctx", "true")
		return c.Next()
	})
	app.Use(cors.New(cors.Config{AllowOrigins: []string{"https://example.org"}}))
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp := performOversizedRequest(t, app, func(req *fasthttp.Request) {
		req.Header.Set(fiber.HeaderOrigin, "https://example.org")
	})

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Equal(t, "true", string(resp.Header.Peek("X-Custom-Ctx")))
	require.Equal(t, "https://example.org", string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
}
