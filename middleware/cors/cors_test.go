package cors

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_CORS_Defaults(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	testDefaultOrEmptyConfig(t, app)
}

func Test_CORS_Empty_Config(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{}))

	testDefaultOrEmptyConfig(t, app)
}

func Test_CORS_Negative_MaxAge(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{MaxAge: -1}))

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodOptions)
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://localhost")
	app.Handler()(ctx)

	require.Equal(t, "0", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlMaxAge)))
}

func testDefaultOrEmptyConfig(t *testing.T, app *fiber.App) {
	t.Helper()

	h := app.Handler()

	// Test default GET response headers
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)

	require.Equal(t, "*", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
	require.Equal(t, "", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowCredentials)))
	require.Equal(t, "", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlExposeHeaders)))

	// Test default OPTIONS (preflight) response headers
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodOptions)
	h(ctx)

	require.Equal(t, "GET,POST,HEAD,PUT,DELETE,PATCH", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowMethods)))
	require.Equal(t, "", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowHeaders)))
	require.Equal(t, "", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlMaxAge)))
}

// go test -run -v Test_CORS_Wildcard
func Test_CORS_Wildcard(t *testing.T) {
	t.Parallel()
	// New fiber instance
	app := fiber.New()
	// OPTIONS (preflight) response headers when AllowOrigins is *
	app.Use(New(Config{
		AllowOrigins:  "*",
		MaxAge:        3600,
		ExposeHeaders: "X-Request-ID",
		AllowHeaders:  "Authentication",
	}))
	// Get handler pointer
	handler := app.Handler()

	// Make request
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://localhost")
	ctx.Request.Header.SetMethod(fiber.MethodOptions)

	// Perform request
	handler(ctx)

	// Check result
	require.Equal(t, "*", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin))) // Validates request is not reflecting origin in the response
	require.Equal(t, "", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowCredentials)))
	require.Equal(t, "3600", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlMaxAge)))
	require.Equal(t, "Authentication", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowHeaders)))

	// Test non OPTIONS (preflight) response headers
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	handler(ctx)

	require.Equal(t, "", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowCredentials)))
	require.Equal(t, "X-Request-ID", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlExposeHeaders)))
}

// go test -run -v Test_CORS_Origin_AllowCredentials
func Test_CORS_Origin_AllowCredentials(t *testing.T) {
	t.Parallel()
	// New fiber instance
	app := fiber.New()
	// OPTIONS (preflight) response headers when AllowOrigins is *
	app.Use(New(Config{
		AllowOrigins:     "http://localhost",
		AllowCredentials: true,
		MaxAge:           3600,
		ExposeHeaders:    "X-Request-ID",
		AllowHeaders:     "Authentication",
	}))
	// Get handler pointer
	handler := app.Handler()

	// Make request
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://localhost")
	ctx.Request.Header.SetMethod(fiber.MethodOptions)

	// Perform request
	handler(ctx)

	// Check result
	require.Equal(t, "http://localhost", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
	require.Equal(t, "true", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowCredentials)))
	require.Equal(t, "3600", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlMaxAge)))
	require.Equal(t, "Authentication", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowHeaders)))

	// Test non OPTIONS (preflight) response headers
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	handler(ctx)

	require.Equal(t, "true", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowCredentials)))
	require.Equal(t, "X-Request-ID", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlExposeHeaders)))
}

// go test -run -v Test_CORS_Wildcard_AllowCredentials_Panic
// Test for fiber-ghsa-fmg4-x8pw-hjhg
func Test_CORS_Wildcard_AllowCredentials_Panic(t *testing.T) {
	t.Parallel()
	// New fiber instance
	app := fiber.New()

	didPanic := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
			}
		}()

		app.Use(New(Config{
			AllowOrigins:     "*",
			AllowCredentials: true,
		}))
	}()

	if !didPanic {
		t.Errorf("Expected a panic when AllowOrigins is '*' and AllowCredentials is true")
	}
}

// go test -run -v Test_CORS_Invalid_Origin_Panic
func Test_CORS_Invalid_Origin_Panic(t *testing.T) {
	t.Parallel()
	// New fiber instance
	app := fiber.New()

	didPanic := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
			}
		}()

		app.Use(New(Config{
			AllowOrigins:     "localhost",
			AllowCredentials: true,
		}))
	}()

	if !didPanic {
		t.Errorf("Expected a panic when Origin is missing scheme")
	}
}

// go test -run -v Test_CORS_Subdomain
func Test_CORS_Subdomain(t *testing.T) {
	t.Parallel()
	// New fiber instance
	app := fiber.New()
	// OPTIONS (preflight) response headers when AllowOrigins is set to a subdomain
	app.Use("/", New(Config{AllowOrigins: "http://*.example.com"}))

	// Get handler pointer
	handler := app.Handler()

	// Make request with disallowed origin
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod(fiber.MethodOptions)
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://google.com")

	// Perform request
	handler(ctx)

	// Allow-Origin header should be "" because http://google.com does not satisfy http://*.example.com
	require.Equal(t, "", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))

	ctx.Request.Reset()
	ctx.Response.Reset()

	// Make request with allowed origin
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod(fiber.MethodOptions)
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://test.example.com")

	handler(ctx)

	require.Equal(t, "http://test.example.com", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
}

func Test_CORS_AllowOriginScheme(t *testing.T) {
	t.Parallel()
	tests := []struct {
		reqOrigin, pattern string
		shouldAllowOrigin  bool
	}{
		{
			pattern:           "http://example.com",
			reqOrigin:         "http://example.com",
			shouldAllowOrigin: true,
		},
		{
			pattern:           "https://example.com",
			reqOrigin:         "https://example.com",
			shouldAllowOrigin: true,
		},
		{
			pattern:           "http://example.com",
			reqOrigin:         "https://example.com",
			shouldAllowOrigin: false,
		},
		{
			pattern:           "http://*.example.com",
			reqOrigin:         "http://aaa.example.com",
			shouldAllowOrigin: true,
		},
		{
			pattern:           "http://*.example.com",
			reqOrigin:         "http://bbb.aaa.example.com",
			shouldAllowOrigin: true,
		},
		{
			pattern:           "http://*.aaa.example.com",
			reqOrigin:         "http://bbb.aaa.example.com",
			shouldAllowOrigin: true,
		},
		{
			pattern:           "http://*.example.com:8080",
			reqOrigin:         "http://aaa.example.com:8080",
			shouldAllowOrigin: true,
		},
		{
			pattern:           "http://example.com",
			reqOrigin:         "http://gofiber.com",
			shouldAllowOrigin: false,
		},
		{
			pattern:           "http://*.aaa.example.com",
			reqOrigin:         "http://ccc.bbb.example.com",
			shouldAllowOrigin: false,
		},
		{
			pattern:           "http://*.example.com",
			reqOrigin:         "http://1234567890.1234567890.1234567890.1234567890.1234567890.1234567890.1234567890.1234567890.1234567890.1234567890.1234567890.example.com",
			shouldAllowOrigin: true,
		},
		{
			pattern:           "http://example.com",
			reqOrigin:         "http://ccc.bbb.example.com",
			shouldAllowOrigin: false,
		},
		{
			pattern:           "https://*--aaa.bbb.com",
			reqOrigin:         "https://prod-preview--aaa.bbb.com",
			shouldAllowOrigin: false,
		},
		{
			pattern:           "http://*.example.com",
			reqOrigin:         "http://ccc.bbb.example.com",
			shouldAllowOrigin: true,
		},
		{
			pattern:           "http://foo.[a-z]*.example.com",
			reqOrigin:         "http://ccc.bbb.example.com",
			shouldAllowOrigin: false,
		},
		{
			pattern:           "http://domain-1.com, http://example.com",
			reqOrigin:         "http://example.com",
			shouldAllowOrigin: true,
		},
		{
			pattern:           "http://domain-1.com, http://example.com",
			reqOrigin:         "http://domain-2.com",
			shouldAllowOrigin: false,
		},
		{
			pattern:           "http://domain-1.com,http://example.com",
			reqOrigin:         "http://domain-1.com",
			shouldAllowOrigin: true,
		},
	}

	for _, tt := range tests {
		app := fiber.New()
		app.Use("/", New(Config{AllowOrigins: tt.pattern}))

		handler := app.Handler()

		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/")
		ctx.Request.Header.SetMethod(fiber.MethodOptions)
		ctx.Request.Header.Set(fiber.HeaderOrigin, tt.reqOrigin)

		handler(ctx)

		if tt.shouldAllowOrigin {
			require.Equal(t, tt.reqOrigin, string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
		} else {
			require.Equal(t, "", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
		}
	}
}

// go test -run Test_CORS_Next
func Test_CORS_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_CORS_AllowOriginsAndAllowOriginsFunc(t *testing.T) {
	t.Parallel()
	// New fiber instance
	app := fiber.New()
	app.Use("/", New(Config{
		AllowOrigins: "http://example-1.com",
		AllowOriginsFunc: func(origin string) bool {
			return strings.Contains(origin, "example-2")
		},
	}))

	// Get handler pointer
	handler := app.Handler()

	// Make request with disallowed origin
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod(fiber.MethodOptions)
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://google.com")

	// Perform request
	handler(ctx)

	// Allow-Origin header should be "" because http://google.com does not satisfy http://example-1.com or 'strings.Contains(origin, "example-2")'
	require.Equal(t, "", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))

	ctx.Request.Reset()
	ctx.Response.Reset()

	// Make request with allowed origin
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod(fiber.MethodOptions)
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://example-1.com")

	handler(ctx)

	require.Equal(t, "http://example-1.com", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))

	ctx.Request.Reset()
	ctx.Response.Reset()

	// Make request with allowed origin
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod(fiber.MethodOptions)
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://example-2.com")

	handler(ctx)

	require.Equal(t, "http://example-2.com", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
}

func Test_CORS_AllowOriginsFunc(t *testing.T) {
	t.Parallel()
	// New fiber instance
	app := fiber.New()
	app.Use("/", New(Config{
		AllowOriginsFunc: func(origin string) bool {
			return strings.Contains(origin, "example-2")
		},
	}))

	// Get handler pointer
	handler := app.Handler()

	// Make request with disallowed origin
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod(fiber.MethodOptions)
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://google.com")

	// Perform request
	handler(ctx)

	// Allow-Origin header should be empty because http://google.com does not satisfy 'strings.Contains(origin, "example-2")'
	// and AllowOrigins has not been set
	require.Equal(t, "", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))

	ctx.Request.Reset()
	ctx.Response.Reset()

	// Make request with allowed origin
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod(fiber.MethodOptions)
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://example-2.com")

	handler(ctx)

	// Allow-Origin header should be "http://example-2.com"
	require.Equal(t, "http://example-2.com", string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
}

func Test_CORS_AllowOriginsAndAllowOriginsFunc_AllUseCases(t *testing.T) {
	testCases := []struct {
		Name           string
		Config         Config
		RequestOrigin  string
		ResponseOrigin string
	}{
		{
			Name: "AllowOriginsDefined/AllowOriginsFuncUndefined/OriginAllowed",
			Config: Config{
				AllowOrigins:     "http://aaa.com",
				AllowOriginsFunc: nil,
			},
			RequestOrigin:  "http://aaa.com",
			ResponseOrigin: "http://aaa.com",
		},
		{
			Name: "AllowOriginsDefined/AllowOriginsFuncUndefined/MultipleOrigins/NoWhitespace/OriginAllowed",
			Config: Config{
				AllowOrigins:     "http://aaa.com,http://bbb.com",
				AllowOriginsFunc: nil,
			},
			RequestOrigin:  "http://bbb.com",
			ResponseOrigin: "http://bbb.com",
		},
		{
			Name: "AllowOriginsDefined/AllowOriginsFuncUndefined/MultipleOrigins/NoWhitespace/OriginNotAllowed",
			Config: Config{
				AllowOrigins:     "http://aaa.com,http://bbb.com",
				AllowOriginsFunc: nil,
			},
			RequestOrigin:  "http://ccc.com",
			ResponseOrigin: "",
		},
		{
			Name: "AllowOriginsDefined/AllowOriginsFuncUndefined/MultipleOrigins/Whitespace/OriginAllowed",
			Config: Config{
				AllowOrigins:     "http://aaa.com, http://bbb.com",
				AllowOriginsFunc: nil,
			},
			RequestOrigin:  "http://aaa.com",
			ResponseOrigin: "http://aaa.com",
		},
		{
			Name: "AllowOriginsDefined/AllowOriginsFuncUndefined/OriginNotAllowed",
			Config: Config{
				AllowOrigins:     "http://aaa.com",
				AllowOriginsFunc: nil,
			},
			RequestOrigin:  "http://bbb.com",
			ResponseOrigin: "",
		},
		{
			Name: "AllowOriginsDefined/AllowOriginsFuncReturnsTrue/OriginAllowed",
			Config: Config{
				AllowOrigins: "http://aaa.com",
				AllowOriginsFunc: func(_ string) bool {
					return true
				},
			},
			RequestOrigin:  "http://aaa.com",
			ResponseOrigin: "http://aaa.com",
		},
		{
			Name: "AllowOriginsDefined/AllowOriginsFuncReturnsTrue/OriginNotAllowed",
			Config: Config{
				AllowOrigins: "http://aaa.com",
				AllowOriginsFunc: func(_ string) bool {
					return true
				},
			},
			RequestOrigin:  "http://bbb.com",
			ResponseOrigin: "http://bbb.com",
		},
		{
			Name: "AllowOriginsDefined/AllowOriginsFuncReturnsFalse/OriginAllowed",
			Config: Config{
				AllowOrigins: "http://aaa.com",
				AllowOriginsFunc: func(_ string) bool {
					return false
				},
			},
			RequestOrigin:  "http://aaa.com",
			ResponseOrigin: "http://aaa.com",
		},
		{
			Name: "AllowOriginsDefined/AllowOriginsFuncReturnsFalse/OriginNotAllowed",
			Config: Config{
				AllowOrigins: "http://aaa.com",
				AllowOriginsFunc: func(_ string) bool {
					return false
				},
			},
			RequestOrigin:  "http://bbb.com",
			ResponseOrigin: "",
		},
		{
			Name: "AllowOriginsEmpty/AllowOriginsFuncUndefined/OriginAllowed",
			Config: Config{
				AllowOrigins:     "",
				AllowOriginsFunc: nil,
			},
			RequestOrigin:  "http://aaa.com",
			ResponseOrigin: "*",
		},
		{
			Name: "AllowOriginsEmpty/AllowOriginsFuncReturnsTrue/OriginAllowed",
			Config: Config{
				AllowOrigins: "",
				AllowOriginsFunc: func(_ string) bool {
					return true
				},
			},
			RequestOrigin:  "http://aaa.com",
			ResponseOrigin: "http://aaa.com",
		},
		{
			Name: "AllowOriginsEmpty/AllowOriginsFuncReturnsFalse/OriginNotAllowed",
			Config: Config{
				AllowOrigins: "",
				AllowOriginsFunc: func(_ string) bool {
					return false
				},
			},
			RequestOrigin:  "http://aaa.com",
			ResponseOrigin: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			app := fiber.New()
			app.Use("/", New(tc.Config))

			handler := app.Handler()

			ctx := &fasthttp.RequestCtx{}
			ctx.Request.SetRequestURI("/")
			ctx.Request.Header.SetMethod(fiber.MethodOptions)
			ctx.Request.Header.Set(fiber.HeaderOrigin, tc.RequestOrigin)

			handler(ctx)

			require.Equal(t, tc.ResponseOrigin, string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
		})
	}
}

// The fix for issue #2422
func Test_CORS_AllowCredentials(t *testing.T) {
	testCases := []struct {
		Name                string
		Config              Config
		RequestOrigin       string
		ResponseOrigin      string
		ResponseCredentials string
	}{
		{
			Name: "AllowOriginsFuncDefined",
			Config: Config{
				AllowCredentials: true,
				AllowOriginsFunc: func(_ string) bool {
					return true
				},
			},
			RequestOrigin: "http://aaa.com",
			// The AllowOriginsFunc config was defined, should use the real origin of the function
			ResponseOrigin:      "http://aaa.com",
			ResponseCredentials: "true",
		},
		{
			Name: "fiber-ghsa-fmg4-x8pw-hjhg-wildcard-credentials",
			Config: Config{
				AllowCredentials: true,
				AllowOriginsFunc: func(_ string) bool {
					return true
				},
			},
			RequestOrigin:  "*",
			ResponseOrigin: "*",
			// Middleware will validate that wildcard wont set credentials to true
			ResponseCredentials: "",
		},
		{
			Name: "AllowOriginsFuncNotDefined",
			Config: Config{
				// Setting this to true will cause the middleware to panic since default AllowOrigins is "*"
				AllowCredentials: false,
			},
			RequestOrigin: "http://aaa.com",
			// None of the AllowOrigins or AllowOriginsFunc config was defined, should use the default origin of "*"
			// which will cause the CORS error in the client:
			// The value of the 'Access-Control-Allow-Origin' header in the response must not be the wildcard '*'
			// when the request's credentials mode is 'include'.
			ResponseOrigin:      "*",
			ResponseCredentials: "",
		},
		{
			Name: "AllowOriginsDefined",
			Config: Config{
				AllowCredentials: true,
				AllowOrigins:     "http://aaa.com",
			},
			RequestOrigin:       "http://aaa.com",
			ResponseOrigin:      "http://aaa.com",
			ResponseCredentials: "true",
		},
		{
			Name: "AllowOriginsDefined/UnallowedOrigin",
			Config: Config{
				AllowCredentials: true,
				AllowOrigins:     "http://aaa.com",
			},
			RequestOrigin:       "http://bbb.com",
			ResponseOrigin:      "",
			ResponseCredentials: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			app := fiber.New()
			app.Use("/", New(tc.Config))

			handler := app.Handler()

			ctx := &fasthttp.RequestCtx{}
			ctx.Request.SetRequestURI("/")
			ctx.Request.Header.SetMethod(fiber.MethodOptions)
			ctx.Request.Header.Set(fiber.HeaderOrigin, tc.RequestOrigin)

			handler(ctx)

			require.Equal(t, tc.ResponseCredentials, string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowCredentials)))
			require.Equal(t, tc.ResponseOrigin, string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
		})
	}
}

// The Enhancement for issue #2804
func Test_CORS_AllowPrivateNetwork(t *testing.T) {
	t.Parallel()

	// Test scenario where AllowPrivateNetwork is enabled
	app := fiber.New()
	app.Use(New(Config{
		AllowPrivateNetwork: true,
	}))
	handler := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodOptions)
	ctx.Request.Header.Set(fiber.HeaderOrigin, "https://example.com")
	ctx.Request.Header.Set("Access-Control-Request-Private-Network", "true")
	handler(ctx)

	// Verify the Access-Control-Allow-Private-Network header is set to "true"
	require.Equal(t, "true", string(ctx.Response.Header.Peek("Access-Control-Allow-Private-Network")), "The Access-Control-Allow-Private-Network header should be set to 'true' when AllowPrivateNetwork is enabled")

	// Reset ctx for next test
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodOptions)
	ctx.Request.Header.Set(fiber.HeaderOrigin, "https://example.com")

	// Test scenario where AllowPrivateNetwork is disabled (default)
	app = fiber.New()
	app.Use(New())
	handler = app.Handler()
	handler(ctx)

	// Verify the Access-Control-Allow-Private-Network header is not present
	require.Equal(t, "", string(ctx.Response.Header.Peek("Access-Control-Allow-Private-Network")), "The Access-Control-Allow-Private-Network header should not be present by default")

	// Test scenario where AllowPrivateNetwork is disabled but client sends header
	app = fiber.New()
	app.Use(New())
	handler = app.Handler()

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodOptions)
	ctx.Request.Header.Set(fiber.HeaderOrigin, "https://example.com")
	ctx.Request.Header.Set("Access-Control-Request-Private-Network", "true")
	handler(ctx)

	// Verify the Access-Control-Allow-Private-Network header is not present
	require.Equal(t, "", string(ctx.Response.Header.Peek("Access-Control-Allow-Private-Network")), "The Access-Control-Allow-Private-Network header should not be present by default")

	// Test scenario where AllowPrivateNetwork is enabled and client does NOT send header
	app = fiber.New()
	app.Use(New(Config{
		AllowPrivateNetwork: true,
	}))
	handler = app.Handler()

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodOptions)
	ctx.Request.Header.Set(fiber.HeaderOrigin, "https://example.com")
	handler(ctx)

	// Verify the Access-Control-Allow-Private-Network header is not present
	require.Equal(t, "", string(ctx.Response.Header.Peek("Access-Control-Allow-Private-Network")), "The Access-Control-Allow-Private-Network header should not be present by default")
}

// go test -v -run=^$ -bench=Benchmark_CORS_NewHandler -benchmem -count=4
func Benchmark_CORS_NewHandler(b *testing.B) {
	app := fiber.New()
	c := New(Config{
		AllowOrigins:     "http://localhost,http://example.com",
		AllowMethods:     "GET,POST,PUT,DELETE",
		AllowHeaders:     "Origin,Content-Type,Accept",
		AllowCredentials: true,
		MaxAge:           600,
	})

	app.Use(c)
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	req := &fasthttp.Request{}
	req.Header.SetMethod(fiber.MethodGet)
	req.SetRequestURI("/")
	req.Header.Set(fiber.HeaderOrigin, "http://localhost")
	req.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodGet)
	req.Header.Set(fiber.HeaderAccessControlRequestHeaders, "Origin,Content-Type,Accept")

	ctx.Init(req, nil, nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h(ctx)
	}
}

// go test -v -run=^$ -bench=Benchmark_CORS_NewHandlerParallel -benchmem -count=4
func Benchmark_CORS_NewHandlerParallel(b *testing.B) {
	app := fiber.New()
	c := New(Config{
		AllowOrigins:     "http://localhost,http://example.com",
		AllowMethods:     "GET,POST,PUT,DELETE",
		AllowHeaders:     "Origin,Content-Type,Accept",
		AllowCredentials: true,
		MaxAge:           600,
	})

	app.Use(c)
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		ctx := &fasthttp.RequestCtx{}

		req := &fasthttp.Request{}
		req.Header.SetMethod(fiber.MethodGet)
		req.SetRequestURI("/")
		req.Header.Set(fiber.HeaderOrigin, "http://localhost")
		req.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodGet)
		req.Header.Set(fiber.HeaderAccessControlRequestHeaders, "Origin,Content-Type,Accept")

		ctx.Init(req, nil, nil)

		for pb.Next() {
			h(ctx)
		}
	})
}

// go test -v -run=^$ -bench=Benchmark_CORS_NewHandlerSingleOrigin -benchmem -count=4
func Benchmark_CORS_NewHandlerSingleOrigin(b *testing.B) {
	app := fiber.New()
	c := New(Config{
		AllowOrigins:     "http://example.com",
		AllowMethods:     "GET,POST,PUT,DELETE",
		AllowHeaders:     "Origin,Content-Type,Accept",
		AllowCredentials: true,
		MaxAge:           600,
	})

	app.Use(c)
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	req := &fasthttp.Request{}
	req.Header.SetMethod(fiber.MethodGet)
	req.SetRequestURI("/")
	req.Header.Set(fiber.HeaderOrigin, "http://example.com")
	req.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodGet)
	req.Header.Set(fiber.HeaderAccessControlRequestHeaders, "Origin,Content-Type,Accept")

	ctx.Init(req, nil, nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h(ctx)
	}
}

// go test -v -run=^$ -bench=Benchmark_CORS_NewHandlerSingleOriginParallel -benchmem -count=4
func Benchmark_CORS_NewHandlerSingleOriginParallel(b *testing.B) {
	app := fiber.New()
	c := New(Config{
		AllowOrigins:     "http://example.com",
		AllowMethods:     "GET,POST,PUT,DELETE",
		AllowHeaders:     "Origin,Content-Type,Accept",
		AllowCredentials: true,
		MaxAge:           600,
	})

	app.Use(c)
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		ctx := &fasthttp.RequestCtx{}

		req := &fasthttp.Request{}
		req.Header.SetMethod(fiber.MethodGet)
		req.SetRequestURI("/")
		req.Header.Set(fiber.HeaderOrigin, "http://example.com")
		req.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodGet)
		req.Header.Set(fiber.HeaderAccessControlRequestHeaders, "Origin,Content-Type,Accept")

		ctx.Init(req, nil, nil)

		for pb.Next() {
			h(ctx)
		}
	})
}

// go test -v -run=^$ -bench=Benchmark_CORS_NewHandlerWildcard -benchmem -count=4
func Benchmark_CORS_NewHandlerWildcard(b *testing.B) {
	app := fiber.New()
	c := New(Config{
		AllowOrigins:     "*",
		AllowMethods:     "GET,POST,PUT,DELETE",
		AllowHeaders:     "Origin,Content-Type,Accept",
		AllowCredentials: false,
		MaxAge:           600,
	})

	app.Use(c)
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	req := &fasthttp.Request{}
	req.Header.SetMethod(fiber.MethodGet)
	req.SetRequestURI("/")
	req.Header.Set(fiber.HeaderOrigin, "http://example.com")
	req.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodGet)
	req.Header.Set(fiber.HeaderAccessControlRequestHeaders, "Origin,Content-Type,Accept")

	ctx.Init(req, nil, nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h(ctx)
	}
}

// go test -v -run=^$ -bench=Benchmark_CORS_NewHandlerWildcardParallel -benchmem -count=4
func Benchmark_CORS_NewHandlerWildcardParallel(b *testing.B) {
	app := fiber.New()
	c := New(Config{
		AllowOrigins:     "*",
		AllowMethods:     "GET,POST,PUT,DELETE",
		AllowHeaders:     "Origin,Content-Type,Accept",
		AllowCredentials: false,
		MaxAge:           600,
	})

	app.Use(c)
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		ctx := &fasthttp.RequestCtx{}

		req := &fasthttp.Request{}
		req.Header.SetMethod(fiber.MethodGet)
		req.SetRequestURI("/")
		req.Header.Set(fiber.HeaderOrigin, "http://example.com")
		req.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodGet)
		req.Header.Set(fiber.HeaderAccessControlRequestHeaders, "Origin,Content-Type,Accept")

		ctx.Init(req, nil, nil)

		for pb.Next() {
			h(ctx)
		}
	})
}

// go test -v -run=^$ -bench=Benchmark_CORS_NewHandlerPreflight -benchmem -count=4
func Benchmark_CORS_NewHandlerPreflight(b *testing.B) {
	app := fiber.New()
	c := New(Config{
		AllowOrigins:     "http://localhost,http://example.com",
		AllowMethods:     "GET,POST,PUT,DELETE",
		AllowHeaders:     "Origin,Content-Type,Accept",
		AllowCredentials: true,
		MaxAge:           600,
	})

	app.Use(c)
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	req := &fasthttp.Request{}
	req.Header.SetMethod(fiber.MethodOptions)
	req.SetRequestURI("/")
	req.Header.Set(fiber.HeaderOrigin, "http://example.com")
	req.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodPost)
	req.Header.Set(fiber.HeaderAccessControlRequestHeaders, "Origin,Content-Type,Accept")

	ctx.Init(req, nil, nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h(ctx)
	}
}

// go test -v -run=^$ -bench=Benchmark_CORS_NewHandlerPreflightParallel -benchmem -count=4
func Benchmark_CORS_NewHandlerPreflightParallel(b *testing.B) {
	app := fiber.New()
	c := New(Config{
		AllowOrigins:     "http://localhost,http://example.com",
		AllowMethods:     "GET,POST,PUT,DELETE",
		AllowHeaders:     "Origin,Content-Type,Accept",
		AllowCredentials: true,
		MaxAge:           600,
	})

	app.Use(c)
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		ctx := &fasthttp.RequestCtx{}

		req := &fasthttp.Request{}
		req.Header.SetMethod(fiber.MethodOptions)
		req.SetRequestURI("/")
		req.Header.Set(fiber.HeaderOrigin, "http://example.com")
		req.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodPost)
		req.Header.Set(fiber.HeaderAccessControlRequestHeaders, "Origin,Content-Type,Accept")

		ctx.Init(req, nil, nil)

		for pb.Next() {
			h(ctx)
		}
	})
}

// go test -v -run=^$ -bench=Benchmark_CORS_NewHandlerPreflightSingleOrigin -benchmem -count=4
func Benchmark_CORS_NewHandlerPreflightSingleOrigin(b *testing.B) {
	app := fiber.New()
	c := New(Config{
		AllowOrigins:     "http://example.com",
		AllowMethods:     "GET,POST,PUT,DELETE",
		AllowHeaders:     "Origin,Content-Type,Accept",
		AllowCredentials: true,
		MaxAge:           600,
	})

	app.Use(c)
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	req := &fasthttp.Request{}
	req.Header.SetMethod(fiber.MethodOptions)
	req.SetRequestURI("/")
	req.Header.Set(fiber.HeaderOrigin, "http://example.com")
	req.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodPost)
	req.Header.Set(fiber.HeaderAccessControlRequestHeaders, "Origin,Content-Type,Accept")

	ctx.Init(req, nil, nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h(ctx)
	}
}

// go test -v -run=^$ -bench=Benchmark_CORS_NewHandlerPreflightSingleOriginParallel -benchmem -count=4
func Benchmark_CORS_NewHandlerPreflightSingleOriginParallel(b *testing.B) {
	app := fiber.New()
	c := New(Config{
		AllowOrigins:     "http://example.com",
		AllowMethods:     "GET,POST,PUT,DELETE",
		AllowHeaders:     "Origin,Content-Type,Accept",
		AllowCredentials: true,
		MaxAge:           600,
	})

	app.Use(c)
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		ctx := &fasthttp.RequestCtx{}

		req := &fasthttp.Request{}
		req.Header.SetMethod(fiber.MethodOptions)
		req.SetRequestURI("/")
		req.Header.Set(fiber.HeaderOrigin, "http://example.com")
		req.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodPost)
		req.Header.Set(fiber.HeaderAccessControlRequestHeaders, "Origin,Content-Type,Accept")

		ctx.Init(req, nil, nil)

		for pb.Next() {
			h(ctx)
		}
	})
}

// go test -v -run=^$ -bench=Benchmark_CORS_NewHandlerPreflightWildcard -benchmem -count=4
func Benchmark_CORS_NewHandlerPreflightWildcard(b *testing.B) {
	app := fiber.New()
	c := New(Config{
		AllowOrigins:     "*",
		AllowMethods:     "GET,POST,PUT,DELETE",
		AllowHeaders:     "Origin,Content-Type,Accept",
		AllowCredentials: false,
		MaxAge:           600,
	})

	app.Use(c)
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	req := &fasthttp.Request{}
	req.Header.SetMethod(fiber.MethodOptions)
	req.SetRequestURI("/")
	req.Header.Set(fiber.HeaderOrigin, "http://example.com")
	req.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodPost)
	req.Header.Set(fiber.HeaderAccessControlRequestHeaders, "Origin,Content-Type,Accept")

	ctx.Init(req, nil, nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h(ctx)
	}
}

// go test -v -run=^$ -bench=Benchmark_CORS_NewHandlerPreflightWildcardParallel -benchmem -count=4
func Benchmark_CORS_NewHandlerPreflightWildcardParallel(b *testing.B) {
	app := fiber.New()
	c := New(Config{
		AllowOrigins:     "*",
		AllowMethods:     "GET,POST,PUT,DELETE",
		AllowHeaders:     "Origin,Content-Type,Accept",
		AllowCredentials: false,
		MaxAge:           600,
	})

	app.Use(c)
	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		ctx := &fasthttp.RequestCtx{}

		req := &fasthttp.Request{}
		req.Header.SetMethod(fiber.MethodOptions)
		req.SetRequestURI("/")
		req.Header.Set(fiber.HeaderOrigin, "http://example.com")
		req.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodPost)
		req.Header.Set(fiber.HeaderAccessControlRequestHeaders, "Origin,Content-Type,Accept")

		ctx.Init(req, nil, nil)

		for pb.Next() {
			h(ctx)
		}
	})
}
