package basicauth

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"golang.org/x/crypto/bcrypt"
)

func sha256Hash(p string) string {
	sum := sha256.Sum256([]byte(p))
	return "{SHA256}" + base64.StdEncoding.EncodeToString(sum[:])
}

func sha512Hash(p string) string {
	sum := sha512.Sum512([]byte(p))
	return "{SHA512}" + base64.StdEncoding.EncodeToString(sum[:])
}

// go test -run Test_BasicAuth_Next
func Test_BasicAuth_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_Middleware_BasicAuth(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	hashedJohn := sha256Hash("doe")
	hashedAdmin, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.MinCost)
	require.NoError(t, err)

	app.Use(New(Config{
		Users: map[string]string{
			"john":  hashedJohn,
			"admin": string(hashedAdmin),
		},
	}))

	app.Get("/testauth", func(c fiber.Ctx) error {
		username := UsernameFromContext(c)
		return c.SendString(username)
	})

	tests := []struct {
		url        string
		username   string
		password   string
		statusCode int
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
		// Each user's password only works for that user, although every
		// request also runs the other user's hash algorithm.
		{
			url:        "/testauth",
			statusCode: 401,
			username:   "john",
			password:   "123456",
		},
		{
			url:        "/testauth",
			statusCode: 401,
			username:   "admin",
			password:   "doe",
		},
	}

	for _, tt := range tests {
		// Base64 encode credentials for http auth header
		creds := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%s:%s", tt.username, tt.password))

		req := httptest.NewRequest(fiber.MethodGet, "/testauth", http.NoBody)
		req.Header.Add("Authorization", "Basic "+creds)
		resp, err := app.Test(req)
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)

		require.NoError(t, err)
		require.Equal(t, tt.statusCode, resp.StatusCode)

		if tt.statusCode == 200 {
			require.Equal(t, tt.username, string(body))
		}
	}
}

func Test_BasicAuthLoggerTagWritesUsername(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	app := fiber.New()
	app.Use(New(Config{
		Users: map[string]string{
			"john": sha256Hash("doe"),
		},
	}))
	app.Use(logger.New(logger.Config{
		Format: "${username}",
		Stream: &buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.SetBasicAuth("john", "doe")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "john", buf.String())
}

// Test_BasicAuthLogContextTagWritesUsername runs serially because it mutates
// package-global default logger output and context format.
func Test_BasicAuthLogContextTagWritesUsername(t *testing.T) {
	buf := loggertest.CaptureContextLog(t, "username=${username} ")

	app := fiber.New()
	app.Use(New(Config{
		Users: map[string]string{
			"john": sha256Hash("doe"),
		},
	}))
	app.Get("/", func(c fiber.Ctx) error {
		fiberlog.WithContext(c).Info("start")
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.SetBasicAuth("john", "doe")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Contains(t, buf.String(), "[Info] username=john start")
}

func Test_BasicAuth_UsernameFromContext_Types(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{PassLocalsToContext: true})
	app.Use(New(Config{
		Users: map[string]string{
			"john": sha256Hash("doe"),
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		require.Equal(t, "john", UsernameFromContext(c))
		customCtx, ok := c.(fiber.CustomCtx)
		require.True(t, ok)
		require.Equal(t, "john", UsernameFromContext(customCtx))
		require.Equal(t, "john", UsernameFromContext(c.RequestCtx()))
		require.Equal(t, "john", UsernameFromContext(c.Context()))
		return c.SendStatus(fiber.StatusOK)
	})

	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add(fiber.HeaderAuthorization, "Basic "+creds)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_BasicAuth_AuthorizerCtx(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	called := false
	app.Use(New(Config{
		Authorizer: func(user, pass string, c fiber.Ctx) bool {
			called = true
			require.Equal(t, "john", user)
			require.Equal(t, "doe", pass)
			require.Equal(t, "/ctx", c.Path())
			return true
		},
	}))

	app.Get("/ctx", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))
	req := httptest.NewRequest(fiber.MethodGet, "/ctx", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.True(t, called)
}

func Test_BasicAuth_WWWAuthenticateHeader(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, `Basic realm="Restricted", charset="UTF-8"`, resp.Header.Get(fiber.HeaderWWWAuthenticate))
}

func Test_BasicAuth_WWWAuthenticateHeader_UTF8(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}, Charset: "utf-8"}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, `Basic realm="Restricted", charset="UTF-8"`, resp.Header.Get(fiber.HeaderWWWAuthenticate))
}

// Test_BasicAuth_WWWAuthenticateHeader_QuotedString verifies that generated
// challenge parameters use HTTP quoted-string escaping rather than Go syntax.
func Test_BasicAuth_WWWAuthenticateHeader_QuotedString(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Users: map[string]string{"john": sha256Hash("doe")},
		Realm: "area \"A\"\\\x01\t\n\r\x7f café",
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	require.Equal(
		t,
		`Basic realm="area \"A\"\\%01%09\n\r%7F café", charset="UTF-8"`,
		resp.Header.Get(fiber.HeaderWWWAuthenticate),
	)
}

func Test_BasicAuth_InvalidHeader(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Basic notbase64")
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func Test_BasicAuth_MissingScheme(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer token")
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, `Basic realm="Restricted", charset="UTF-8"`, resp.Header.Get(fiber.HeaderWWWAuthenticate))
}

func Test_BasicAuth_MissingColon(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}}))

	creds := base64.StdEncoding.EncodeToString([]byte("john"))
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func Test_BasicAuth_EmptyAuthorization(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}}))

	cases := []string{"", "   "}
	for _, h := range cases {
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, h)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	}
}

func Test_BasicAuth_HeaderWhitespace(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))

	cases := []struct {
		header string
		status int
	}{
		{"Basic " + creds, fiber.StatusTeapot},
		{" Basic " + creds, fiber.StatusTeapot},
		{"Basic  " + creds, fiber.StatusBadRequest},
		{"Basic   " + creds, fiber.StatusBadRequest},
		{"Basic\t" + creds, fiber.StatusBadRequest},
		{"Basic \t" + creds, fiber.StatusBadRequest},
		{"Basic\u00A0" + creds, fiber.StatusBadRequest},
		{"Basic\u3000" + creds, fiber.StatusBadRequest},
		{"\tBasic " + creds + "\t", fiber.StatusTeapot},
		{"Basic " + creds[:4] + " " + creds[4:], fiber.StatusBadRequest},
	}

	for _, tt := range cases {
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, tt.header)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, tt.status, resp.StatusCode)
	}
}

func Test_BasicAuth_ControlChars(t *testing.T) {
	t.Parallel()
	called := false
	app := fiber.New()
	app.Use(New(Config{
		Authorizer: func(_, _ string, _ fiber.Ctx) bool {
			called = true
			return true
		},
	}))

	creds := []string{
		base64.StdEncoding.EncodeToString([]byte("john:\x01doe")),
		base64.StdEncoding.EncodeToString([]byte("jo\x7Fhn:doe")),
		base64.StdEncoding.EncodeToString([]byte{'j', 'o', 'h', 'n', ':', 0x85, 'd', 'o', 'e'}),
		base64.StdEncoding.EncodeToString([]byte{'j', 'o', 'h', 'n', ':', 0x9F, 'd', 'o', 'e'}),
	}

	for _, c := range creds {
		called = false
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, "Basic "+c)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
		require.Empty(t, resp.Header.Get(fiber.HeaderWWWAuthenticate))
		require.False(t, called)
	}
}

func Test_BasicAuth_UnpaddedBase64(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))
	creds = strings.TrimRight(creds, "=")

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
}

func Test_BasicAuth_NonASCIIHeader(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}}))
	handler := app.Handler()
	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/")
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.Header.SetBytesKV([]byte(fiber.HeaderAuthorization), []byte("Basic \x80"+creds))
	handler(fctx)
	require.Equal(t, fiber.StatusBadRequest, fctx.Response.StatusCode())
}

func Test_BasicAuth_InvalidUTF8(t *testing.T) {
	t.Parallel()
	called := false
	app := fiber.New()
	app.Use(New(Config{
		Charset: "UTF-8",
		Authorizer: func(_, _ string, _ fiber.Ctx) bool {
			called = true
			return true
		},
	}))

	creds := base64.StdEncoding.EncodeToString([]byte{'j', 'o', 'h', 'n', ':', 0xff, 'd', 'o', 'e'})
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	require.False(t, called)
}

func Test_BasicAuth_UTF8Normalization(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	decomposed := "e\u0301" // e + combining acute accent
	called := false
	app.Use(New(Config{
		Charset: "UTF-8",
		Authorizer: func(u, p string, _ fiber.Ctx) bool {
			called = true
			require.Equal(t, "é", u)
			require.Equal(t, "doe", p)
			return true
		},
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

	creds := base64.StdEncoding.EncodeToString([]byte(decomposed + ":doe"))
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	require.True(t, called)
}

func Test_BasicAuth_HeaderControlCharEdges(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}}))

	handler := app.Handler()
	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))
	// Note: \r and \n are sanitized to spaces by fasthttp at the protocol level,
	// so we use other control chars (SOH, BEL) that pass through unchanged.
	headers := [][]byte{
		[]byte("\x01Basic " + creds),
		[]byte("\x07Basic " + creds),
		[]byte("Basic " + creds + "\x01"),
		[]byte("Basic " + creds + "\x07"),
	}

	for _, h := range headers {
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.SetRequestURI("/")
		fctx.Request.Header.SetMethod(fiber.MethodGet)
		fctx.Request.Header.SetBytesKV([]byte(fiber.HeaderAuthorization), h)
		handler(fctx)
		require.Equal(t, fiber.StatusBadRequest, fctx.Response.StatusCode())
	}
}

func Test_BasicAuth_Charset(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() { New(Config{Charset: "ISO-8859-1"}) })
	require.NotPanics(t, func() { New(Config{Charset: "utf-8"}) })
	require.NotPanics(t, func() { New(Config{Charset: "UTF-8"}) })
	require.NotPanics(t, func() { New(Config{}) })
}

func Test_BasicAuth_HeaderLimit(t *testing.T) {
	t.Parallel()
	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))
	hashedJohn := sha256Hash("doe")

	t.Run("too large", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Users: map[string]string{"john": hashedJohn}, HeaderLimit: 10}))
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusRequestHeaderFieldsTooLarge, resp.StatusCode)
	})

	t.Run("allowed", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Users: map[string]string{"john": hashedJohn}, HeaderLimit: 100}))
		app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	})
}

// go test -v -run=^$ -bench=Benchmark_Middleware_BasicAuth -benchmem -count=4
func Benchmark_Middleware_BasicAuth(b *testing.B) {
	app := fiber.New()

	hashedJohn := sha256Hash("doe")

	app.Use(New(Config{
		Users: map[string]string{
			"john": hashedJohn,
		},
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")
	fctx.Request.Header.Set(fiber.HeaderAuthorization, "basic am9objpkb2U=") // john:doe

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
}

// go test -v -run=^$ -bench=Benchmark_Middleware_BasicAuth -benchmem -count=4
func Benchmark_Middleware_BasicAuth_Upper(b *testing.B) {
	app := fiber.New()

	hashedJohn := sha256Hash("doe")

	app.Use(New(Config{
		Users: map[string]string{
			"john": hashedJohn,
		},
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")
	fctx.Request.Header.Set(fiber.HeaderAuthorization, "Basic am9objpkb2U=") // john:doe

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
}

func Test_BasicAuth_Immutable(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{Immutable: true})

	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
}

func Test_parseHashedPassword(t *testing.T) {
	t.Parallel()
	pass := "secret"
	sha := sha256.Sum256([]byte(pass))
	b64 := base64.StdEncoding.EncodeToString(sha[:])
	hexDigest := hex.EncodeToString(sha[:])
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.MinCost)
	require.NoError(t, err)

	cases := []struct {
		name   string
		hashed string
	}{
		{"bcrypt", string(bcryptHash)},
		{"sha512", sha512Hash(pass)},
		{"sha256", sha256Hash(pass)},
		{"sha256-hex", hexDigest},
		{"sha256-b64", b64},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			verify, err := parseHashedPassword(tt.hashed)
			require.NoError(t, err)
			require.True(t, verify(pass))
			require.False(t, verify("wrong"))
		})
	}
}

func Test_buildVerifiers(t *testing.T) {
	t.Parallel()

	t.Run("groups hashes by algorithm and bcrypt cost", func(t *testing.T) {
		t.Parallel()

		bcryptA, err := bcrypt.GenerateFromPassword([]byte("bcrypt-a-pass"), bcrypt.MinCost)
		require.NoError(t, err)
		bcryptB, err := bcrypt.GenerateFromPassword([]byte("bcrypt-b-pass"), bcrypt.MinCost)
		require.NoError(t, err)
		bcryptHigher, err := bcrypt.GenerateFromPassword([]byte("bcrypt-higher-pass"), bcrypt.MinCost+1)
		require.NoError(t, err)

		v, err := buildVerifiers(map[string]string{
			"sha256":        sha256Hash("sha256-pass"),
			"sha256-hex":    hex.EncodeToString(sha256Sum("sha256-hex-pass")),
			"sha256-b64":    base64.StdEncoding.EncodeToString(sha256Sum("sha256-b64-pass")),
			"sha512":        sha512Hash("sha512-pass"),
			"bcrypt-a":      string(bcryptA),
			"bcrypt-b":      string(bcryptB),
			"bcrypt-higher": string(bcryptHigher),
		})
		require.NoError(t, err)
		require.Len(t, v.users, 7)

		// SHA-256 in any encoding, SHA-512, and bcrypt once per cost.
		require.Len(t, v.dummies, 4)
		class := func(user string) int { return v.users[user].class }
		require.Equal(t, class("sha256"), class("sha256-hex"))
		require.Equal(t, class("sha256"), class("sha256-b64"))
		require.Equal(t, class("bcrypt-a"), class("bcrypt-b"))
		require.NotEqual(t, class("bcrypt-a"), class("bcrypt-higher"))
		require.NotEqual(t, class("sha256"), class("sha512"))
		require.NotEqual(t, class("sha256"), class("bcrypt-a"))
		require.NotEqual(t, class("sha512"), class("bcrypt-a"))

		// Each class's dummy is the first verifier of that class in sorted
		// username order.
		require.True(t, v.dummies[class("bcrypt-a")]("bcrypt-a-pass"))
		require.True(t, v.dummies[class("bcrypt-higher")]("bcrypt-higher-pass"))
		require.True(t, v.dummies[class("sha256")]("sha256-pass"))
		require.True(t, v.dummies[class("sha512")]("sha512-pass"))
	})

	t.Run("uses a fixed-work fallback when no users are configured", func(t *testing.T) {
		t.Parallel()

		v, err := buildVerifiers(nil)
		require.NoError(t, err)
		require.Empty(t, v.users)
		require.Len(t, v.dummies, 1)
		fallbackInput := "fiber-basicauth-dummy"
		require.True(t, v.dummies[0](fallbackInput))
		require.False(t, v.dummies[0]("wrong"))
		require.False(t, v.verify("john", fallbackInput))
	})
}

func Test_hashClassOf(t *testing.T) {
	t.Parallel()

	bcryptHash, err := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost+1)
	require.NoError(t, err)

	tests := []struct {
		name string
		hash string
		want hashClass
	}{
		{"bcrypt", string(bcryptHash), hashClass{algorithm: hashAlgorithmBcrypt, cost: bcrypt.MinCost + 1}},
		{"bcrypt with unparseable cost", "$2a$99$" + strings.Repeat("a", 53), hashClass{algorithm: hashAlgorithmBcrypt}},
		{"sha512", sha512Hash("pass"), hashClass{algorithm: hashAlgorithmSHA512}},
		{"sha256", sha256Hash("pass"), hashClass{algorithm: hashAlgorithmSHA256}},
		{"sha256 hex", hex.EncodeToString(sha256Sum("pass")), hashClass{algorithm: hashAlgorithmSHA256}},
		{"sha256 base64", base64.StdEncoding.EncodeToString(sha256Sum("pass")), hashClass{algorithm: hashAlgorithmSHA256}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, hashClassOf(tt.hash))
		})
	}
}

// Test_credentialVerifier_EqualWork ensures every request runs exactly one
// verification per hash class whichever user it names, so response timing
// reveals neither whether a user exists nor which hash their password uses.
func Test_credentialVerifier_EqualWork(t *testing.T) {
	t.Parallel()

	passwords := map[string]string{
		"sha256":        "sha256-pass",
		"sha512":        "sha512-pass",
		"bcrypt":        "bcrypt-pass",
		"bcrypt-higher": "bcrypt-higher-pass",
	}
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(passwords["bcrypt"]), bcrypt.MinCost)
	require.NoError(t, err)
	bcryptHigher, err := bcrypt.GenerateFromPassword([]byte(passwords["bcrypt-higher"]), bcrypt.MinCost+1)
	require.NoError(t, err)

	v, err := buildVerifiers(map[string]string{
		"sha256":        sha256Hash(passwords["sha256"]),
		"sha512":        sha512Hash(passwords["sha512"]),
		"bcrypt":        string(bcryptHash),
		"bcrypt-higher": string(bcryptHigher),
	})
	require.NoError(t, err)
	require.Len(t, v.dummies, 4)

	// Count the verifications a request runs in each hash class.
	calls := make([]int, len(v.dummies))
	counting := func(class int, verify passwordVerifier) passwordVerifier {
		return func(p string) bool {
			calls[class]++
			return verify(p)
		}
	}
	for i, dummy := range v.dummies {
		v.dummies[i] = counting(i, dummy)
	}
	for name, u := range v.users {
		u.verify = counting(u.class, u.verify)
		v.users[name] = u
	}

	check := func(user, pass string, want bool) {
		t.Helper()
		clear(calls)
		require.Equal(t, want, v.verify(user, pass), "user %q, password %q", user, pass)
		for class, n := range calls {
			require.Equal(t, 1, n, "user %q: verifications in hash class %d", user, class)
		}
	}

	for user, pass := range passwords {
		check(user, pass, true)
		check(user, "wrong", false)
		// Every user here is their class's dummy, so each other user's
		// password makes a dummy match; that result must be discarded.
		for other, otherPass := range passwords {
			if other != user {
				check(user, otherPass, false)
			}
		}
		check("unknown", pass, false)
	}
	check("unknown", "wrong", false)
}

func Test_BasicAuth_HashVariants(t *testing.T) {
	t.Parallel()
	pass := "doe"
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.MinCost)
	require.NoError(t, err)
	cases := []struct {
		name   string
		hashed string
	}{
		{"bcrypt", string(bcryptHash)},
		{"sha512", sha512Hash(pass)},
		{"sha256", sha256Hash(pass)},
		{"sha256-hex", func() string { h := sha256.Sum256([]byte(pass)); return hex.EncodeToString(h[:]) }()},
	}

	for _, tt := range cases {
		app := fiber.New()
		app.Use(New(Config{Users: map[string]string{"john": tt.hashed}}))
		app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

		creds := base64.StdEncoding.EncodeToString([]byte("john:" + pass))
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	}
}

func Test_BasicAuth_HashVariants_Invalid(t *testing.T) {
	t.Parallel()
	pass := "doe"
	wrong := "wrong"
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.MinCost)
	require.NoError(t, err)
	cases := []struct {
		name   string
		hashed string
	}{
		{"bcrypt", string(bcryptHash)},
		{"sha512", sha512Hash(pass)},
		{"sha256", sha256Hash(pass)},
		{"sha256-hex", func() string { h := sha256.Sum256([]byte(pass)); return hex.EncodeToString(h[:]) }()},
	}

	for _, tt := range cases {
		app := fiber.New()
		app.Use(New(Config{Users: map[string]string{"john": tt.hashed}}))
		app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

		creds := base64.StdEncoding.EncodeToString([]byte("john:" + wrong))
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	}
}

func Test_containsInvalidHeaderChars_WordBoundaries(t *testing.T) {
	t.Parallel()

	// Valid: HTAB and visible ASCII, across scalar, whole-word, and
	// overlapping-tail positions.
	require.False(t, containsInvalidHeaderChars(""))
	require.False(t, containsInvalidHeaderChars("Basic\tQWxhZGRpbjpvcGVuIHNlc2FtZQ=="))
	require.False(t, containsInvalidHeaderChars("Basic Q"))
	require.False(t, containsInvalidHeaderChars(" ~\t"))

	// Invalid bytes at every code path: short input, inside a full word,
	// and inside the final overlapping word.
	require.True(t, containsInvalidHeaderChars("\n"))
	require.True(t, containsInvalidHeaderChars("Basic \x00credentials"))
	require.True(t, containsInvalidHeaderChars("Basic credential\x7f"))
	require.True(t, containsInvalidHeaderChars("Basic credentials\x80"))
	require.True(t, containsInvalidHeaderChars("Basic  credentials"))
	require.True(t, containsInvalidHeaderChars("abcdefg\r"))
}

// go test -v -run=^$ -bench=Benchmark_containsInvalidHeaderChars -benchmem -count=4
func Benchmark_containsInvalidHeaderChars(b *testing.B) {
	inputs := []string{
		"Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==", // typical valid header
		"Basic dXNlcjpwYXNz",
		"Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==\x80", // invalid at the tail
	}
	var got bool
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range inputs {
			got = containsInvalidHeaderChars(in)
		}
	}
	_ = got
}

func Test_containsCTL(t *testing.T) {
	t.Parallel()

	// Clean ASCII across scalar, word, and overlapping-tail paths.
	require.False(t, containsCTL(""))
	require.False(t, containsCTL("user"))
	require.False(t, containsCTL("averylongusername"))
	require.False(t, containsCTL("pass word with spaces and ~"))

	// C0 and DEL at every code path position.
	require.True(t, containsCTL("\x00"))
	require.True(t, containsCTL("user\npass"))
	require.True(t, containsCTL("averylongusername\x7f"))
	require.True(t, containsCTL("abcdefgh\x1fjklmnopq"))

	// Unicode: C1 controls (multi-byte runes) must still be caught, and
	// ordinary non-ASCII letters must still pass, wherever the first
	// non-ASCII byte sits.
	require.True(t, containsCTL("\u0085"))
	require.True(t, containsCTL("abcdefgh\u009d"))
	require.True(t, containsCTL("pässword\u0085tail"))
	require.False(t, containsCTL("pässwörd"))
	require.False(t, containsCTL("abcdefghijklmnoöp"))
}

// go test -v -run=^$ -bench=Benchmark_containsCTL -benchmem -count=4
func Benchmark_containsCTL(b *testing.B) {
	inputs := []string{
		"john",
		"a-much-longer-username",
		"s3cr3t-p4ssw0rd-with-length",
	}
	var got bool
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range inputs {
			got = containsCTL(in)
		}
	}
	_ = got
}

// Test_BasicAuth_RejectsWrongDigestLength ensures a prefixed hash whose
// decoded digest has the wrong size is rejected at construction time instead
// of being accepted as a verifier that can never match.
func Test_BasicAuth_RejectsWrongDigestLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		hash    string
	}{
		{
			name:    "sha256 too short",
			hash:    "{SHA256}" + base64.StdEncoding.EncodeToString([]byte("short")),
			wantErr: ErrInvalidSHA256PasswordLength,
		},
		{
			name:    "sha512 too short",
			hash:    "{SHA512}" + base64.StdEncoding.EncodeToString([]byte("short")),
			wantErr: ErrInvalidSHA512PasswordLength,
		},
		{
			name:    "sha512 holds a sha256 digest",
			hash:    "{SHA512}" + base64.StdEncoding.EncodeToString(sha256Sum("hello")),
			wantErr: ErrInvalidSHA512PasswordLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := buildVerifiers(map[string]string{"john": tt.hash})
			require.ErrorIs(t, err, tt.wantErr)
		})
	}

	// A correctly sized digest still builds and verifies.
	v, err := buildVerifiers(map[string]string{
		"john": "{SHA512}" + base64.StdEncoding.EncodeToString(sha512Sum("doe")),
	})
	require.NoError(t, err)
	require.True(t, v.verify("john", "doe"))
	require.False(t, v.verify("john", "nope"))
}

func sha256Sum(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

func sha512Sum(s string) []byte {
	sum := sha512.Sum512([]byte(s))
	return sum[:]
}
