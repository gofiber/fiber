package fiber_test

import (
	"bytes"
	"cmp"
	"errors"
	"mime/multipart"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
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

	go func() { errCh <- app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()

	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		if err := <-errCh; err != nil && !errors.Is(err, net.ErrClosed) {
			require.NoError(t, err)
		}
	})

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		return err == nil && conn.Close() == nil
	}, time.Second, 10*time.Millisecond)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	resp := fasthttp.AcquireResponse()
	t.Cleanup(func() { fasthttp.ReleaseResponse(resp) })

	req.SetRequestURI("http://example.com/")
	req.Header.SetMethod(fiber.MethodPost)
	req.Header.Set(fiber.HeaderOrigin, "https://example.com")
	req.SetBody(bytes.Repeat([]byte{'a'}, 32))
	if configure != nil {
		configure(req)
	}

	client := fasthttp.Client{Dial: func(string) (net.Conn, error) { return ln.Dial() }}

	require.NoError(t, client.Do(req, resp))
	return resp
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
	config           fiber.Config
	setup            func(app *fiber.App)
	configureRequest func(req *fasthttp.Request)
	handler          func(c fiber.Ctx) error
	assertions       func(t *testing.T, resp *fasthttp.Response)
	expectedStatus   int
}

func Test_Integration_RequestID_ContextPropagationFlag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		config fiber.Config
		name   string
		id     string
		// inContext is the request ID that c.Context() holds: none unless the locals are passed to it.
		inContext string
	}{
		{name: "disabled by default", id: "rid-disabled"},
		{name: "enabled", id: "rid-enabled", inContext: "rid-enabled", config: fiber.Config{PassLocalsToContext: true}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(tc.config)
			app.Use(requestid.New(requestid.Config{Generator: func() string { return tc.id }}))

			app.Get("/", func(c fiber.Ctx) error {
				require.Equal(t, tc.id, requestid.FromContext(c))
				require.Equal(t, tc.inContext, requestid.FromContext(c.Context()))
				return c.SendStatus(fiber.StatusOK)
			})

			apptest.Expect(t, app, "/", fiber.StatusOK)
		})
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
			configureRequest: func(req *fasthttp.Request) { req.Header.Set(fiber.HeaderOrigin, corsHelmetOrigin) },
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
				app.Use(requestid.New(requestid.Config{Generator: func() string { return generatedRequestID }}))
			},
			configureRequest: func(req *fasthttp.Request) { req.Header.Set(fiber.HeaderOrigin, corsHelmetRequestID) },
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
				app.Use(cors.New(cors.Config{AllowOrigins: []string{corsSessionOrigin}, AllowCredentials: true}))
				app.Use(session.New())
				app.Use(func(c fiber.Ctx) error {
					if sm := session.FromContext(c); sm != nil {
						sm.Set("cors-session", "enabled")
					}
					return c.Next()
				})
			},
			configureRequest: func(req *fasthttp.Request) { req.Header.Set(fiber.HeaderOrigin, corsSessionOrigin) },
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
					Max:          helmetLimiterMax,
					Expiration:   time.Duration(helmetLimiterReset) * time.Second,
					KeyGenerator: func(fiber.Ctx) string { return "helmet+limiter" },
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
				app.Use(cors.New(cors.Config{AllowOrigins: []string{corsCSRForigin}, AllowCredentials: true}))
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
			configureRequest: func(req *fasthttp.Request) { req.Header.SetMethod(fiber.MethodGet) },
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
					Validator:        func(fiber.Ctx, string) (bool, error) { return false, nil },
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
			configureRequest: func(req *fasthttp.Request) { req.Header.Set(fiber.HeaderAcceptEncoding, "gzip") },
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
			handler: func(fiber.Ctx) error { panic("panic for recover middleware") },
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
				// Recover writes a plain-text body; ensure we still return content to clients while
				// keeping Helmet's security headers intact.
				require.Contains(t, string(resp.Body()), "panic for recover middleware")
			},
		},
		// --- Requests refused for their size never reach a route, and keep what the middleware set.
		{
			name:   "body limit+cors",
			config: fiber.Config{BodyLimit: 16},
			setup: func(app *fiber.App) {
				app.Use(cors.New(cors.Config{
					AllowOrigins:     []string{"https://example.com"},
					AllowCredentials: true,
					ExposeHeaders:    []string{"X-Request-ID"},
				}))
			},
			expectedStatus: fiber.StatusRequestEntityTooLarge,
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "https://example.com", string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
				require.Equal(t, "true", string(resp.Header.Peek(fiber.HeaderAccessControlAllowCredentials)))
				require.Equal(t, "X-Request-ID", string(resp.Header.Peek(fiber.HeaderAccessControlExposeHeaders)))
				require.Equal(t, "Origin", string(resp.Header.Peek(fiber.HeaderVary)))
			},
		},
		{
			name:           "body limit+helmet",
			config:         fiber.Config{BodyLimit: 16},
			setup:          func(app *fiber.App) { app.Use(helmet.New()) },
			expectedStatus: fiber.StatusRequestEntityTooLarge,
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
				require.Equal(t, "same-origin", string(resp.Header.Peek("Cross-Origin-Opener-Policy")))
				require.Equal(t, "same-origin", string(resp.Header.Peek("Cross-Origin-Resource-Policy")))
				require.Equal(t, "require-corp", string(resp.Header.Peek("Cross-Origin-Embedder-Policy")))
			},
		},
		{
			name:             "body limit+requestid",
			config:           fiber.Config{BodyLimit: 16},
			setup:            func(app *fiber.App) { app.Use(requestid.New()) },
			configureRequest: func(req *fasthttp.Request) { req.Header.Set("X-Request-ID", "integration-request-id") },
			expectedStatus:   fiber.StatusRequestEntityTooLarge,
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "integration-request-id", string(resp.Header.Peek("X-Request-ID")))
			},
		},
		{
			name:   "body limit+group middleware chain",
			config: fiber.Config{BodyLimit: 16},
			setup: func(app *fiber.App) {
				app.Use(helmet.New())

				api := app.Group("/api")
				api.Use(requestid.New())
				api.Use(func(c fiber.Ctx) error {
					c.Set("X-Group-Middleware", "active")
					return c.Next()
				})
				api.Post("/resource", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
			},
			configureRequest: func(req *fasthttp.Request) { req.SetRequestURI("http://example.com/api/resource") },
			expectedStatus:   fiber.StatusRequestEntityTooLarge,
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
				require.NotEmpty(t, resp.Header.Peek("X-Request-ID"))
				require.Equal(t, "active", string(resp.Header.Peek("X-Group-Middleware")))
			},
		},
		{
			name:   "body limit+headers from subsequent middleware",
			config: fiber.Config{BodyLimit: 8},
			setup: func(app *fiber.App) {
				app.Use(func(c fiber.Ctx) error {
					c.Set("X-Custom-Middleware", "ran")
					return c.Next()
				})
				app.Use(cors.New())
			},
			expectedStatus: fiber.StatusRequestEntityTooLarge,
			assertions: func(t *testing.T, resp *fasthttp.Response) {
				t.Helper()
				require.Equal(t, "ran", string(resp.Header.Peek("X-Custom-Middleware")))
				require.Equal(t, "*", string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(tc.config)
			tc.setup(app)
			// Every stack shares the same route that always hits the default error handler so we
			// can verify which headers survive the error response. A few cases override the
			// handler to exercise panic recovery or other routes that still flow through the
			// default error path.
			handler := tc.handler
			if handler == nil {
				handler = func(fiber.Ctx) error {
					return fiber.NewError(fiber.StatusInternalServerError, "middleware combination failure")
				}
			}
			app.All("/", handler)

			resp := performOversizedRequest(t, app, tc.configureRequest)
			require.Equal(t, cmp.Or(tc.expectedStatus, fiber.StatusInternalServerError), resp.StatusCode())
			tc.assertions(t, resp)
		})
	}
}

func Test_Integration_App_ServerErrorHandler_CustomMultipartBodyLimit(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{
		BodyLimit: 128,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) && fiberErr != nil {
				code = fiberErr.Code
			}
			return c.Status(code).JSON(fiber.Map{"code": code, "message": err.Error()})
		},
	})
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", "large.txt")
	require.NoError(t, err)
	_, err = fileWriter.Write(bytes.Repeat([]byte{'a'}, 256))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	resp := performOversizedRequest(t, app, func(req *fasthttp.Request) {
		req.Header.Set(fiber.HeaderContentType, writer.FormDataContentType())
		req.SetBody(body.Bytes())
	})

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Contains(t, string(resp.Header.ContentType()), fiber.MIMEApplicationJSON)
	require.JSONEq(t, `{"code":413,"message":"Request Entity Too Large"}`, string(resp.Body()))
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
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	resp := performOversizedRequest(t, app, func(req *fasthttp.Request) {
		req.Header.Set(fiber.HeaderOrigin, "https://example.org")
	})

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Equal(t, "true", string(resp.Header.Peek("X-Custom-Ctx")))
	require.Equal(t, "https://example.org", string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
}

// integrationDomainGet requests path from app as host, requires the status and returns the response.
func integrationDomainGet(t *testing.T, app *fiber.App, host, path string, status int) *http.Response {
	t.Helper()

	return apptest.Expect(t, app, apptest.Req(http.MethodGet, path, apptest.WithHost(host)), status)
}

// integrationDomainBody is integrationDomainGet for a 200 whose body is body.
func integrationDomainBody(t *testing.T, app *fiber.App, host, path, body string) *http.Response {
	t.Helper()

	return apptest.ExpectBody(t, app, apptest.Req(http.MethodGet, path, apptest.WithHost(host)), fiber.StatusOK, body)
}

func Test_Integration_Domain_WithMiddleware(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(helmet.New())
	app.Use(requestid.New())
	app.Domain("api.example.com").Get("/users", func(c fiber.Ctx) error { return c.SendString("api users") })
	app.Get("/users", func(c fiber.Ctx) error { return c.SendString("default users") })

	resp := integrationDomainBody(t, app, "api.example.com", "/users", "api users")
	require.NotEmpty(t, resp.Header.Get("X-Request-ID"))
	require.Equal(t, "nosniff", resp.Header.Get(fiber.HeaderXContentTypeOptions))

	integrationDomainBody(t, app, "www.example.com", "/users", "default users")
}

func Test_Integration_Domain_WithCORS(t *testing.T) {
	t.Parallel()
	const apiOrigin = "https://api.example.com"

	app := fiber.New()
	app.Use(cors.New(cors.Config{AllowOrigins: []string{apiOrigin}, AllowCredentials: true}))

	app.Domain("api.example.com").Get("/data", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"source": "api"}) })

	req := apptest.Req(http.MethodGet, "/data", apptest.WithHost("api.example.com"), apptest.WithHeader(fiber.HeaderOrigin, apiOrigin))
	resp := apptest.Expect(t, app, req, fiber.StatusOK)
	require.Equal(t, apiOrigin, resp.Header.Get(fiber.HeaderAccessControlAllowOrigin))
	require.Equal(t, "true", resp.Header.Get(fiber.HeaderAccessControlAllowCredentials))
}

func Test_Integration_Domain_WithGroup(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(helmet.New())
	apiDomain := app.Domain("api.example.com")
	apiDomain.Use(requestid.New(requestid.Config{Generator: func() string { return "domain-request-id" }}))
	apiDomain.Group("/v1").Get("/posts", func(c fiber.Ctx) error { return c.SendString("v1 posts") })
	apiDomain.Group("/v2").Get("/posts", func(c fiber.Ctx) error { return c.SendString("v2 posts") })

	resp := integrationDomainBody(t, app, "api.example.com", "/v1/posts", "v1 posts")
	require.Equal(t, "domain-request-id", resp.Header.Get("X-Request-ID"))
	require.Equal(t, "nosniff", resp.Header.Get(fiber.HeaderXContentTypeOptions))

	integrationDomainBody(t, app, "api.example.com", "/v2/posts", "v2 posts")
}

func Test_Integration_Domain_WithSubAppMount(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(helmet.New())
	subApp := fiber.New()
	subApp.Use(requestid.New())
	subApp.Get("/users", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"users": []string{"alice", "bob"}}) })
	subApp.Get("/posts", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"posts": []string{"post1", "post2"}}) })
	app.Domain("api.example.com").Use("/api", subApp)
	app.Get("/api/users", func(c fiber.Ctx) error { return c.SendString("default users") })

	resp := integrationDomainGet(t, app, "api.example.com", "/api/users", fiber.StatusOK)
	require.Contains(t, apptest.Body(t, resp), "alice")
	require.NotEmpty(t, resp.Header.Get("X-Request-ID"))
	require.Equal(t, "nosniff", resp.Header.Get(fiber.HeaderXContentTypeOptions))

	integrationDomainBody(t, app, "www.example.com", "/api/users", "default users")
}

func Test_Integration_Domain_WithParams(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(requestid.New())
	app.Domain(":tenant.app.example.com").Get("/dashboard", func(c fiber.Ctx) error {
		tenant := fiber.DomainParam(c, "tenant")
		return c.JSON(fiber.Map{"tenant": tenant, "page": "dashboard", "requestID": c.Get("X-Request-ID")})
	})

	resp := integrationDomainGet(t, app, "acme.app.example.com", "/dashboard", fiber.StatusOK)
	body := apptest.Body(t, resp)
	require.Contains(t, body, "acme")
	require.Contains(t, body, "dashboard")
	require.NotEmpty(t, resp.Header.Get("X-Request-ID"))
}

func Test_Integration_Domain_MultipleDomains(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Domain("api.example.com").Get("/", func(c fiber.Ctx) error { return c.SendString("api domain") })
	app.Domain("admin.example.com").Get("/", func(c fiber.Ctx) error { return c.SendString("admin domain") })
	app.Domain("app.example.com").Get("/", func(c fiber.Ctx) error { return c.SendString("app domain") })
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("default domain") })

	tests := []struct {
		host     string
		expected string
	}{
		{"api.example.com", "api domain"},
		{"admin.example.com", "admin domain"},
		{"app.example.com", "app domain"},
		{"www.example.com", "default domain"},
		{"unknown.example.com", "default domain"},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			t.Parallel()
			integrationDomainBody(t, app, tt.host, "/", tt.expected)
		})
	}
}

func Test_Integration_Domain_WithSession(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(session.New())
	app.Domain("api.example.com").Get("/session", func(c fiber.Ctx) error {
		sess := session.FromContext(c)
		if sess == nil {
			return c.Status(fiber.StatusInternalServerError).SendString("no session")
		}
		sess.Set("domain", "api")
		return c.SendString("session set")
	})

	resp := integrationDomainGet(t, app, "api.example.com", "/session", fiber.StatusOK)
	require.Contains(t, resp.Header.Get(fiber.HeaderSetCookie), "session_id=")
}

func Test_Integration_Domain_WithCompress(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(compress.New())
	app.Domain("api.example.com").Get("/data", func(c fiber.Ctx) error {
		// Return large enough content for compression
		return c.SendString(strings.Repeat("Hello World! ", 100))
	})

	req := apptest.Req(http.MethodGet, "/data", apptest.WithHost("api.example.com"), apptest.WithHeader(fiber.HeaderAcceptEncoding, "gzip"))
	resp := apptest.Expect(t, app, req, fiber.StatusOK)
	require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
}

func Test_Integration_Domain_ErrorHandling(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			c.Set("X-Error-Handled", "true")
			return c.Status(fiber.StatusInternalServerError).SendString("error: " + err.Error())
		},
	})
	app.Use(helmet.New())
	app.Domain("api.example.com").Get("/error", func(_ fiber.Ctx) error {
		return fiber.NewError(fiber.StatusInternalServerError, "domain error")
	})

	resp := integrationDomainGet(t, app, "api.example.com", "/error", fiber.StatusInternalServerError)
	require.Equal(t, "true", resp.Header.Get("X-Error-Handled"))
	require.Equal(t, "nosniff", resp.Header.Get(fiber.HeaderXContentTypeOptions))
	require.Contains(t, apptest.Body(t, resp), "domain error")
}

func Test_Integration_Domain_WithLimiter(t *testing.T) {
	t.Parallel()
	const maxRequests = 3

	app := fiber.New()
	app.Use(limiter.New(limiter.Config{
		Max:          maxRequests,
		Expiration:   1 * time.Minute,
		KeyGenerator: func(_ fiber.Ctx) string { return "domain-limiter" },
	}))
	app.Domain("api.example.com").Get("/limited", func(c fiber.Ctx) error { return c.SendString("ok") })

	for range maxRequests {
		resp := integrationDomainGet(t, app, "api.example.com", "/limited", fiber.StatusOK)
		require.Equal(t, strconv.Itoa(maxRequests), resp.Header.Get("X-RateLimit-Limit"))
	}
	integrationDomainGet(t, app, "api.example.com", "/limited", fiber.StatusTooManyRequests)
}

func Test_Integration_Domain_WithCSRF(t *testing.T) {
	t.Parallel()
	const csrfCookie = "domain-csrf"

	app := fiber.New()
	app.Use(csrf.New(csrf.Config{CookieName: csrfCookie, KeyGenerator: func() string { return "test-csrf-token" }}))
	app.Domain("api.example.com").Get("/form", func(c fiber.Ctx) error { return c.SendString("form page") })

	resp := integrationDomainGet(t, app, "api.example.com", "/form", fiber.StatusOK)
	require.Contains(t, resp.Header.Get(fiber.HeaderSetCookie), csrfCookie+"=")
}

func Test_Integration_Domain_CaseInsensitiveMatching(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Domain("API.Example.COM").Get("/", func(c fiber.Ctx) error { return c.SendString("matched") })

	hosts := []string{"api.example.com", "API.EXAMPLE.COM", "Api.Example.Com", "aPi.eXaMpLe.CoM"}

	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			t.Parallel()
			integrationDomainBody(t, app, host, "/", "matched")
		})
	}
}

func Test_Integration_Domain_ReusableSubApp(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	apiApp := fiber.New()
	apiApp.Get("/status", func(c fiber.Ctx) error { return c.SendString("ok") })

	// The same sub-app is mounted on two domains.
	app.Domain("api1.example.com").Use("/v1", apiApp)
	app.Domain("api2.example.com").Use("/v2", apiApp)

	integrationDomainBody(t, app, "api1.example.com", "/v1/status", "ok")
	integrationDomainBody(t, app, "api2.example.com", "/v2/status", "ok")
}
