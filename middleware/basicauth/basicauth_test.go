package basicauth

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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
	}

	for _, tt := range tests {
		// Base64 encode credentials for http auth header
		creds := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%s:%s", tt.username, tt.password))

		req := httptest.NewRequest(fiber.MethodGet, "/testauth", nil)
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
	req := httptest.NewRequest(fiber.MethodGet, "/ctx", nil)
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, `Basic realm="Restricted", charset="UTF-8"`, resp.Header.Get(fiber.HeaderWWWAuthenticate))
}

func Test_BasicAuth_InvalidHeader(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}}))

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Basic notbase64")
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func Test_BasicAuth_EmptyAuthorization(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}}))

	cases := []string{"", "   "}
	for _, h := range cases {
		req := httptest.NewRequest(fiber.MethodGet, "/", nil)
		req.Header.Set(fiber.HeaderAuthorization, h)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	}
}

func Test_BasicAuth_WhitespaceHandling(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	hashedJohn := sha256Hash("doe")
	app.Use(New(Config{Users: map[string]string{"john": hashedJohn}}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))

	cases := []string{
		"Basic " + creds,
		" Basic \t" + creds,
		"Basic  " + creds + "   ",
	}

	for _, h := range cases {
		req := httptest.NewRequest(fiber.MethodGet, "/", nil)
		req.Header.Set(fiber.HeaderAuthorization, h)
		resp, err := app.Test(req)

		require.NoError(t, err)
		require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	}
}

func Test_BasicAuth_HeaderLimit(t *testing.T) {
	t.Parallel()
	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))
	hashedJohn := sha256Hash("doe")

	t.Run("too large", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Users: map[string]string{"john": hashedJohn}, HeaderLimit: 10}))
		req := httptest.NewRequest(fiber.MethodGet, "/", nil)
		req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("allowed", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Users: map[string]string{"john": hashedJohn}, HeaderLimit: 100}))
		app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })
		req := httptest.NewRequest(fiber.MethodGet, "/", nil)
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
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
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
		req := httptest.NewRequest(fiber.MethodGet, "/", nil)
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
		req := httptest.NewRequest(fiber.MethodGet, "/", nil)
		req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	}
}
