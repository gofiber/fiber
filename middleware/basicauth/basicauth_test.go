package basicauth

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"golang.org/x/crypto/bcrypt"
)

func sha256Hash(p string) string { return "{SHA256}" + base64.StdEncoding.EncodeToString(sha256Sum(p)) }

func sha512Hash(p string) string { return "{SHA512}" + base64.StdEncoding.EncodeToString(sha512Sum(p)) }

// teapotApp returns an app that answers GET / with 418 behind New(*cfg). cfg
// is a pointer because Config is 80 bytes and gocritic refuses the copy.
func teapotApp(cfg *Config, appCfg ...fiber.Config) *fiber.App {
	app := fiber.New(appCfg...)
	app.Use(New(*cfg))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })
	return app
}

func Test_BasicAuth_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Next: func(_ fiber.Ctx) bool { return true }}))

	apptest.Expect(t, app, "/", fiber.StatusNotFound)
}

// withBasicCredentials authenticates a request with credentials, a
// "user:password" pair, in the Basic scheme.
func withBasicCredentials(credentials string) apptest.Option {
	return apptest.WithHeader(fiber.HeaderAuthorization, "Basic "+base64.StdEncoding.EncodeToString([]byte(credentials)))
}

func Test_Middleware_BasicAuth(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	hashedJohn := sha256Hash("doe")
	hashedAdmin, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.MinCost)
	require.NoError(t, err)

	app.Use(New(Config{Users: map[string]string{"john": hashedJohn, "admin": string(hashedAdmin)}}))

	app.Get("/testauth", func(c fiber.Ctx) error {
		username := UsernameFromContext(c)
		return c.SendString(username)
	})

	tests := []struct {
		name       string
		username   string
		password   string
		statusCode int
	}{
		{name: "sha256 user", statusCode: 200, username: "john", password: "doe"},
		{name: "bcrypt user", statusCode: 200, username: "admin", password: "123456"},
		{name: "unknown user", statusCode: 401, username: "ee", password: "123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := apptest.Req(fiber.MethodGet, "/testauth", withBasicCredentials(tt.username+":"+tt.password))
			if tt.statusCode == 200 {
				apptest.ExpectBody(t, app, req, tt.statusCode, tt.username)
				return
			}
			apptest.Expect(t, app, req, tt.statusCode)
		})
	}
}

func Test_BasicAuthLoggerTagWritesUsername(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	app := fiber.New()
	app.Use(New(Config{Users: map[string]string{"john": sha256Hash("doe")}}))
	app.Use(logger.New(logger.Config{Format: "${username}", Stream: &buf}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withBasicCredentials("john:doe")), fiber.StatusOK)
	require.Equal(t, "john", buf.String())
}

// Test_BasicAuthLogContextTagWritesUsername runs serially because it mutates
// package-global default logger output and context format.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_BasicAuthLogContextTagWritesUsername(t *testing.T) {
	buf := loggertest.CaptureContextLog(t, "username=${username} ")

	app := fiber.New()
	app.Use(New(Config{Users: map[string]string{"john": sha256Hash("doe")}}))
	app.Get("/", func(c fiber.Ctx) error {
		fiberlog.WithContext(c).Info("start")
		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withBasicCredentials("john:doe")), fiber.StatusOK)
	require.Contains(t, buf.String(), "[Info] username=john start")
}

func Test_BasicAuth_UsernameFromContext_Types(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{PassLocalsToContext: true})
	app.Use(New(Config{Users: map[string]string{"john": sha256Hash("doe")}}))

	app.Get("/", func(c fiber.Ctx) error {
		require.Equal(t, "john", UsernameFromContext(c))
		customCtx, ok := c.(fiber.CustomCtx)
		require.True(t, ok)
		require.Equal(t, "john", UsernameFromContext(customCtx))
		require.Equal(t, "john", UsernameFromContext(c.RequestCtx()))
		require.Equal(t, "john", UsernameFromContext(c.Context()))
		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withBasicCredentials("john:doe")), fiber.StatusOK)
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

	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/ctx", withBasicCredentials("john:doe")), fiber.StatusOK)
	require.True(t, called)
}

// Test_BasicAuth_NoConfig covers New() called without a Config. It used to hand
// back the package default untouched, whose Authorizer, Unauthorized and
// BadRequest are nil, so the first request called a nil function. With no users
// and no Authorizer every request has to be refused, never served or crashed on.
func Test_BasicAuth_NoConfig(t *testing.T) {
	t.Parallel()
	for name, handler := range map[string]fiber.Handler{"New()": New(), "New(Config{})": New(Config{})} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(handler)
			app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

			tests := []struct {
				name          string
				authorization string
				want          int
			}{
				{name: "no credentials", want: fiber.StatusUnauthorized},
				{name: "credentials for a user nobody configured", authorization: "Basic " + base64.StdEncoding.EncodeToString([]byte("john:doe")), want: fiber.StatusUnauthorized},
				{name: "empty username and password", authorization: "Basic " + base64.StdEncoding.EncodeToString([]byte(":")), want: fiber.StatusUnauthorized},
				{name: "another scheme", authorization: "Bearer token", want: fiber.StatusUnauthorized},
				{name: "malformed credentials", authorization: "Basic notbase64", want: fiber.StatusBadRequest},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					authorization := apptest.WithHeaderIf(fiber.HeaderAuthorization, tc.authorization)
					req := apptest.Req(fiber.MethodGet, "/", authorization)

					resp := apptest.Expect(t, app, req, tc.want)
					if tc.want == fiber.StatusUnauthorized {
						require.Equal(t, `Basic realm="Restricted", charset="UTF-8"`, resp.Header.Get(fiber.HeaderWWWAuthenticate))
					}
				})
			}
		})
	}
}

// Test_BasicAuth_ConfigDefault_NoConfig pins that the no-config path runs the same
// defaulting as the config path, so no handler is left nil.
func Test_BasicAuth_ConfigDefault_NoConfig(t *testing.T) {
	t.Parallel()
	cfg := configDefault()
	require.NotNil(t, cfg.Authorizer)
	require.NotNil(t, cfg.Unauthorized)
	require.NotNil(t, cfg.BadRequest)
	require.Equal(t, ConfigDefault.Realm, cfg.Realm)
	require.Equal(t, ConfigDefault.Charset, cfg.Charset)
	require.Equal(t, ConfigDefault.HeaderLimit, cfg.HeaderLimit)
	require.False(t, cfg.Authorizer("john", "doe", nil), "nobody is configured")

	// The package default itself stays what it documents.
	require.Nil(t, ConfigDefault.Authorizer)
	require.Nil(t, ConfigDefault.Unauthorized)
	require.Nil(t, ConfigDefault.BadRequest)
}

// Test_BasicAuth_Unauthorized_KeepsVary covers the default 401 handler, which
// replaced a Vary an earlier middleware had set (Origin from CORS, for one)
// instead of adding Authorization to it.
func Test_BasicAuth_Unauthorized_KeepsVary(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		if earlier := c.Get("X-Earlier-Vary"); earlier != "" {
			c.Set(fiber.HeaderVary, earlier)
		}
		return c.Next()
	})
	app.Use(New(Config{Users: map[string]string{"john": sha256Hash("doe")}}))

	tests := []struct {
		name    string
		earlier string
		want    string
	}{
		{name: "no earlier Vary", want: "Authorization"},
		{name: "an earlier field is kept", earlier: "Origin", want: "Origin, Authorization"},
		{name: "several earlier fields are kept", earlier: "Origin, Accept-Encoding", want: "Origin, Accept-Encoding, Authorization"},
		{name: "a field already listed is not repeated", earlier: "Origin, authorization", want: "Origin, authorization"},
		{name: "a wildcard stays the only member", earlier: "*", want: "*"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeaderIf("X-Earlier-Vary", tc.earlier))

			resp := apptest.Expect(t, app, req, fiber.StatusUnauthorized)
			require.Equal(t, []string{tc.want}, resp.Header.Values(fiber.HeaderVary))
		})
	}
}

func Test_BasicAuth_WWWAuthenticateHeader(t *testing.T) {
	t.Parallel()
	users := map[string]string{"john": sha256Hash("doe")}

	tests := []struct {
		name   string
		want   string
		config Config
	}{
		{name: "default", config: Config{Users: users}, want: `Basic realm="Restricted", charset="UTF-8"`},
		{name: "UTF-8 in lower case", config: Config{Users: users, Charset: "utf-8"}, want: `Basic realm="Restricted", charset="UTF-8"`},
		// The generated challenge parameters use HTTP quoted-string escaping rather than Go syntax.
		{
			name:   "quoted string",
			config: Config{Users: users, Realm: "area \"A\"\\\x01\t\n\r\x7f café"},
			want:   `Basic realm="area \"A\"\\%01%09\n\r%7F café", charset="UTF-8"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(tc.config))

			resp := apptest.Expect(t, app, "/", fiber.StatusUnauthorized)
			require.Equal(t, tc.want, resp.Header.Get(fiber.HeaderWWWAuthenticate))
		})
	}
}

func Test_BasicAuth_AuthorizationHeader(t *testing.T) {
	t.Parallel()
	app := teapotApp(&Config{Users: map[string]string{"john": sha256Hash("doe")}})
	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))

	tests := []struct {
		name          string
		authorization string
		status        int
		challenged    bool // the answer carries the Basic challenge
	}{
		{name: "invalid base64", authorization: "Basic notbase64", status: fiber.StatusBadRequest},
		{name: "missing scheme", authorization: "Bearer token", status: fiber.StatusUnauthorized, challenged: true},
		{name: "missing colon", authorization: "Basic " + base64.StdEncoding.EncodeToString([]byte("john")), status: fiber.StatusBadRequest},
		{name: "empty", authorization: "", status: fiber.StatusUnauthorized},
		{name: "blank", authorization: "   ", status: fiber.StatusUnauthorized},
		{name: "unpadded base64", authorization: "Basic " + strings.TrimRight(creds, "="), status: fiber.StatusTeapot},
		// whitespace around and inside the header
		{name: "valid", authorization: "Basic " + creds, status: fiber.StatusTeapot},
		{name: "leading space", authorization: " Basic " + creds, status: fiber.StatusTeapot},
		{name: "two spaces after the scheme", authorization: "Basic  " + creds, status: fiber.StatusBadRequest},
		{name: "three spaces after the scheme", authorization: "Basic   " + creds, status: fiber.StatusBadRequest},
		{name: "tab after the scheme", authorization: "Basic\t" + creds, status: fiber.StatusBadRequest},
		{name: "space and tab after the scheme", authorization: "Basic \t" + creds, status: fiber.StatusBadRequest},
		{name: "no-break space after the scheme", authorization: "Basic\u00A0" + creds, status: fiber.StatusBadRequest},
		{name: "ideographic space after the scheme", authorization: "Basic\u3000" + creds, status: fiber.StatusBadRequest},
		{name: "surrounding tabs", authorization: "\tBasic " + creds + "\t", status: fiber.StatusTeapot},
		{name: "space inside the credentials", authorization: "Basic " + creds[:4] + " " + creds[4:], status: fiber.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderAuthorization, tc.authorization))
			resp := apptest.Expect(t, app, req, tc.status)
			if tc.challenged {
				require.Equal(t, `Basic realm="Restricted", charset="UTF-8"`, resp.Header.Get(fiber.HeaderWWWAuthenticate))
			}
		})
	}
}

// Test_BasicAuth_RepeatedAuthorizationIsRefused covers a message carrying the
// Authorization field on more than one line, which RFC 9110 Section 5.2 forbids
// for a single-value field. Reading any one of the lines lets whoever wrote the
// other decide what the request means: net/http's Header.Get and fasthttp's
// Peek answer with the first line even when it is empty, so a net/http layer in
// front of the app that refused a revoked credential by that reading saw none,
// while a first-non-empty read behind it would have authenticated with the
// credential on the line after. The message is refused instead, as a malformed
// header is, and without a challenge: the client did not fail to authenticate,
// it sent a request with no single meaning.
func Test_BasicAuth_RepeatedAuthorizationIsRefused(t *testing.T) {
	t.Parallel()
	creds := "Basic " + base64.StdEncoding.EncodeToString([]byte("john:doe"))
	newApp := func(config ...fiber.Config) *fiber.App {
		return teapotApp(&Config{Users: map[string]string{"john": sha256Hash("doe")}}, config...)
	}

	t.Run("lines under the canonical name", func(t *testing.T) {
		t.Parallel()
		app := newApp()

		for _, lines := range [][]string{{"", creds}, {creds, ""}, {creds, creds}} {
			req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			req.Header[fiber.HeaderAuthorization] = lines
			resp := apptest.Do(t, app, req)
			require.Equal(t, fiber.StatusBadRequest, resp.StatusCode, "lines %q", lines)
			require.Empty(t, resp.Header.Get(fiber.HeaderWWWAuthenticate), "a malformed message is not challenged")
		}

		// One line is still read, so the refusals above cannot pass by refusing
		// everything.
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, creds)
		apptest.Expect(t, app, req, fiber.StatusTeapot)
	})

	t.Run("lines under two spellings", func(t *testing.T) {
		t.Parallel()
		// Under DisableHeaderNormalizing the store keeps the spelling the peer
		// sent, so the lines land under two names and a byte-exact read would
		// find only one of them.
		app := newApp(fiber.Config{DisableHeaderNormalizing: true})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header[fiber.HeaderAuthorization] = []string{""}
		req.Header["authorization"] = []string{creds}
		apptest.Expect(t, app, req, fiber.StatusBadRequest)

		req = httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header["authorization"] = []string{creds}
		apptest.Expect(t, app, req, fiber.StatusTeapot)
	})

	t.Run("behind adaptor.FiberApp", func(t *testing.T) {
		t.Parallel()
		// The shape the differential was reported in: a net/http layer in front
		// reads the field as empty and lets the request through to the Fiber
		// app, which must not then find a credential on the second line.
		handler := adaptor.FiberApp(newApp())

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header[fiber.HeaderAuthorization] = []string{"", creds}
		require.Empty(t, req.Header.Get(fiber.HeaderAuthorization), "net/http reads the empty first line")

		rec := httptest.NewRecorder()
		handler(rec, req)
		require.Equal(t, fiber.StatusBadRequest, rec.Code)
	})
}

func Test_BasicAuth_CredentialPolicies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		credentials string
		status      int
		authorized  bool
	}{
		{name: "Unicode password", credentials: "john:pässwörd", status: fiber.StatusTeapot, authorized: true},
		{name: "control character in the password", credentials: "john:\x01doe", status: fiber.StatusBadRequest},
		{name: "DEL in the username", credentials: "jo\x7Fhn:doe", status: fiber.StatusBadRequest},
		{name: "continuation byte 0x85 in the password", credentials: "john:\x85doe", status: fiber.StatusBadRequest},
		{name: "continuation byte 0x9F in the password", credentials: "john:\x9fdoe", status: fiber.StatusBadRequest},
		{name: "invalid UTF-8", credentials: "john:\xffdoe", status: fiber.StatusBadRequest},
		{name: "tab in username", credentials: "jo\thn:doe", status: fiber.StatusBadRequest},
		{name: "C1 after Unicode", credentials: "john:päss\u0085tail", status: fiber.StatusBadRequest},
		{name: "DEL after Unicode", credentials: "john:päss\x7f", status: fiber.StatusBadRequest},
		{name: "C0 after word boundary", credentials: "abcdefghé\x00tail:doe", status: fiber.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			called := false
			app := teapotApp(&Config{
				Charset:    "UTF-8",
				Authorizer: func(_, _ string, _ fiber.Ctx) bool { called = true; return true },
			})

			resp := apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withBasicCredentials(test.credentials)), test.status)
			if test.status == fiber.StatusBadRequest {
				require.Empty(t, resp.Header.Get(fiber.HeaderWWWAuthenticate))
			}
			require.Equal(t, test.authorized, called)
		})
	}
}

func Test_BasicAuth_InvalidHeaderBytes(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Users: map[string]string{"john": sha256Hash("doe")}}))

	handler := app.Handler()
	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))

	tests := []struct {
		name   string
		header string
	}{
		{name: "non-ASCII byte before the credentials", header: "Basic \x80" + creds},
		// Note: \r and \n are sanitized to spaces by fasthttp at the protocol level,
		// so we use other control chars (SOH, BEL) that pass through unchanged.
		{name: "SOH before the scheme", header: "\x01Basic " + creds},
		{name: "BEL before the scheme", header: "\x07Basic " + creds},
		{name: "SOH after the credentials", header: "Basic " + creds + "\x01"},
		{name: "BEL after the credentials", header: "Basic " + creds + "\x07"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fctx := newAuthorizedRequestCtx(tc.header)
			handler(fctx)
			require.Equal(t, fiber.StatusBadRequest, fctx.Response.StatusCode())
		})
	}
}

func Test_BasicAuth_UTF8Normalization(t *testing.T) {
	t.Parallel()
	decomposed := "e\u0301" // e + combining acute accent
	called := false
	app := teapotApp(&Config{
		Charset: "UTF-8",
		Authorizer: func(u, p string, _ fiber.Ctx) bool {
			called = true
			require.Equal(t, "é", u)
			require.Equal(t, "doe", p)
			return true
		},
	})

	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withBasicCredentials(decomposed+":doe")), fiber.StatusTeapot)
	require.True(t, called)
}

func Test_BasicAuth_Charset(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() { New(Config{Charset: "ISO-8859-1"}) })
	require.NotPanics(t, func() { New(Config{Charset: "utf-8"}) })
	require.NotPanics(t, func() { New(Config{Charset: "UTF-8"}) })
	require.NotPanics(t, func() { New(Config{}) })
}

func Test_BasicAuth_Options(t *testing.T) {
	t.Parallel()
	users := map[string]string{"john": sha256Hash("doe")}

	tests := []struct {
		appConfig fiber.Config
		name      string
		config    Config
		status    int
	}{
		{name: "HeaderLimit too large", config: Config{Users: users, HeaderLimit: 10}, status: fiber.StatusRequestHeaderFieldsTooLarge},
		{name: "HeaderLimit allowed", config: Config{Users: users, HeaderLimit: 100}, status: fiber.StatusTeapot},
		{name: "Immutable", config: Config{Users: users}, appConfig: fiber.Config{Immutable: true}, status: fiber.StatusTeapot},
	}

	for i := range tests {
		tc := &tests[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := teapotApp(&tc.config, tc.appConfig)
			apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withBasicCredentials("john:doe")), tc.status)
		})
	}
}

// newAuthorizedRequestCtx returns a GET / request context carrying the given Authorization header.
func newAuthorizedRequestCtx(authorization string) *fasthttp.RequestCtx {
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")
	fctx.Request.Header.Set(fiber.HeaderAuthorization, authorization)
	return fctx
}

func Benchmark_Middleware_BasicAuth(b *testing.B) {
	h := teapotApp(&Config{Users: map[string]string{"john": sha256Hash("doe")}}).Handler()
	fctx := newAuthorizedRequestCtx("basic am9objpkb2U=") // john:doe

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
}

func Benchmark_Middleware_BasicAuth_Upper(b *testing.B) {
	h := teapotApp(&Config{Users: map[string]string{"john": sha256Hash("doe")}}).Handler()
	fctx := newAuthorizedRequestCtx("Basic am9objpkb2U=") // john:doe

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
}

func Test_buildVerifiers(t *testing.T) {
	t.Parallel()
	t.Run("each user accepts only its own password", func(t *testing.T) {
		t.Parallel()
		alphaHash, err := bcrypt.GenerateFromPassword([]byte("alpha-pass"), bcrypt.MinCost+1)
		require.NoError(t, err)
		gammaHash, err := bcrypt.GenerateFromPassword([]byte("gamma-pass"), bcrypt.MinCost)
		require.NoError(t, err)
		verifiers, dummyVerify, err := buildVerifiers(map[string]string{
			"zeta":  sha256Hash("zeta-pass"),
			"alpha": string(alphaHash),
			"beta":  sha512Hash("beta-pass"),
			"gamma": string(gammaHash),
		})
		require.NoError(t, err)
		require.Len(t, verifiers, 4)
		for user, verify := range verifiers {
			for other := range verifiers {
				require.Equal(t, user == other, verify(other+"-pass"), "user %s, password of %s", user, other)
			}
		}
		for user := range verifiers {
			require.False(t, dummyVerify(user+"-pass"))
		}
	})

	t.Run("uses a fixed-work fallback when no users are configured", func(t *testing.T) {
		t.Parallel()
		verifiers, dummyVerify, err := buildVerifiers(nil)
		require.NoError(t, err)
		require.Empty(t, verifiers)
		fallbackInput := "fiber-basicauth-dummy"
		require.True(t, dummyVerify(fallbackInput))
		require.False(t, dummyVerify("wrong"))
	})
}

// Not parallel: other bcrypt tests would skew the timing, and Go pauses
// parallel tests while a sequential one runs.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_buildVerifiers_SameCostAcrossTiers(t *testing.T) {
	adminHash, err := bcrypt.GenerateFromPassword([]byte("admin-pass"), bcrypt.MinCost+3)
	require.NoError(t, err)
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-pass"), bcrypt.MinCost+2)
	require.NoError(t, err)
	verifiers, dummyVerify, err := buildVerifiers(map[string]string{
		"admin": string(adminHash),
		"old":   string(oldHash),
		"john":  sha256Hash("john-pass"),
	})
	require.NoError(t, err)

	// Fastest of interleaved runs, in CPU time so a busy machine cannot stretch one side.
	// What noise is left only adds time: measure until the minima agree, a real difference never does.
	names := []string{"admin", "old", "john", "unknown"}
	attempts := []passwordVerifier{verifiers["admin"], verifiers["old"], verifiers["john"], dummyVerify}
	best := make([]time.Duration, len(attempts))
	for i := range best {
		best[i] = time.Duration(math.MaxInt64)
	}
	const tolerance = 0.2
	agree := func() bool {
		unknown := float64(best[len(best)-1])
		for _, took := range best[:len(best)-1] {
			if math.Abs(float64(took)/unknown-1) > tolerance {
				return false
			}
		}
		return true
	}
	for round := 0; round < 60 && (round < 5 || !agree()); round++ {
		for i, verify := range attempts {
			start := cpuTime(t)
			verify("wrong")
			best[i] = min(best[i], cpuTime(t)-start)
		}
	}
	unknown := best[len(best)-1]
	for i, took := range best[:len(best)-1] {
		require.InDelta(t, 1, float64(took)/float64(unknown), tolerance, "%s took %v, unknown %v", names[i], took, unknown)
	}
}

// Test_BasicAuth_HashVariants runs every supported password hash format through
// parseHashedPassword and through a full request.
func Test_BasicAuth_HashVariants(t *testing.T) {
	t.Parallel()
	const pass = "doe"
	sha := sha256Sum(pass)
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.MinCost)
	require.NoError(t, err)
	cases := []struct {
		name   string
		hashed string
	}{
		{"bcrypt", string(bcryptHash)},
		{"sha512", sha512Hash(pass)},
		{"sha256", sha256Hash(pass)},
		{"sha256-hex", hex.EncodeToString(sha)},
		{"sha256-b64", base64.StdEncoding.EncodeToString(sha)},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Run("parseHashedPassword", func(t *testing.T) {
				t.Parallel()
				verify, err := parseHashedPassword(tt.hashed)
				require.NoError(t, err)
				require.True(t, verify(pass))
				require.False(t, verify("wrong"))
			})
			t.Run("request", func(t *testing.T) {
				t.Parallel()
				app := teapotApp(&Config{Users: map[string]string{"john": tt.hashed}})
				apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withBasicCredentials("john:"+pass)), fiber.StatusTeapot)
				apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withBasicCredentials("john:wrong")), fiber.StatusUnauthorized)
			})
		})
	}
}

func Test_containsInvalidHeaderChars_WordBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want bool
	}{
		// Valid: HTAB and visible ASCII, across scalar, whole-word, and
		// overlapping-tail positions.
		{in: "", want: false},
		{in: "Basic\tQWxhZGRpbjpvcGVuIHNlc2FtZQ==", want: false},
		{in: "Basic Q", want: false},
		{in: " ~\t", want: false},

		// Invalid bytes at every code path: short input, inside a full word,
		// and inside the final overlapping word.
		{in: "\n", want: true},
		{in: "Basic \x00credentials", want: true},
		{in: "Basic credential\x7f", want: true},
		{in: "Basic credentials\x80", want: true},
		{in: "Basic \u00a0credentials", want: true},
		{in: "abcdefg\r", want: true},
	}

	for _, tc := range tests {
		t.Run(strconv.Quote(tc.in), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, containsInvalidHeaderChars(tc.in))
		})
	}
}

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
	tests := []struct {
		in   string
		want bool
	}{
		// Clean ASCII across scalar, word, and overlapping-tail paths.
		{in: "", want: false},
		{in: "user", want: false},
		{in: "averylongusername", want: false},
		{in: "pass word with spaces and ~", want: false},

		// C0 and DEL at every code path position.
		{in: "\x00", want: true},
		{in: "user\npass", want: true},
		{in: "averylongusername\x7f", want: true},
		{in: "abcdefgh\x1fjklmnopq", want: true},

		// Unicode: C1 controls (multi-byte runes) must still be caught, and
		// ordinary non-ASCII letters must still pass, wherever the first
		// non-ASCII byte sits.
		{in: "\u0085", want: true},
		{in: "abcdefgh\u009d", want: true},
		{in: "pässword\u0085tail", want: true},
		{in: "pässwörd", want: false},
		{in: "abcdefghijklmnoöp", want: false},
	}

	for _, tc := range tests {
		t.Run(strconv.Quote(tc.in), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, containsCTL(tc.in))
		})
	}
}

func Benchmark_containsCTL(b *testing.B) {
	inputs := []string{"john", "a-much-longer-username", "s3cr3t-p4ssw0rd-with-length"}
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
	b64 := base64.StdEncoding.EncodeToString
	tests := []struct {
		wantErr error
		name    string
		hash    string
	}{
		{ErrInvalidSHA256PasswordLength, "sha256 too short", "{SHA256}" + b64([]byte("short"))},
		{ErrInvalidSHA512PasswordLength, "sha512 too short", "{SHA512}" + b64([]byte("short"))},
		{ErrInvalidSHA512PasswordLength, "sha512 holds a sha256 digest", "{SHA512}" + b64(sha256Sum("hello"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := buildVerifiers(map[string]string{"john": tt.hash})
			require.ErrorIs(t, err, tt.wantErr)
		})
	}

	// A correctly sized digest still builds and verifies.
	verifiers, _, err := buildVerifiers(map[string]string{"john": "{SHA512}" + b64(sha512Sum("doe"))})
	require.NoError(t, err)
	require.True(t, verifiers["john"]("doe"))
	require.False(t, verifiers["john"]("nope"))
}

func sha256Sum(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

func sha512Sum(s string) []byte {
	sum := sha512.Sum512([]byte(s))
	return sum[:]
}
