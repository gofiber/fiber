package cache

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_carriesCredential(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{KeyCredentialHeaders: []string{"X-Company-Token"}})
	for name, want := range map[string]bool{
		"Authorization":       true,
		"authorization":       true,
		"COOKIE":              true,
		"Proxy-Authorization": true,
		"X-API-Key":           true,
		"x-auth-token":        true,
		"X-Access-Token":      true,
		"X-Company-Token":     true,
		"x-company-token":     true,
		"X-Tenant":            false,
		"Accept-Language":     false,
		"Set-Cookie":          false,
		"X-Authorization":     false,
		"":                    false,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, want, cfg.carriesCredential(name))
		})
	}
}

func Test_scopeOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		headers          map[string]string
		name             string
		varyNames        []string
		cfg              Config
		hasAuthorization bool
		wantEntry        bool
		wantKeyed        bool
		wantRecord       bool // the refusal gets a record of its own
	}{
		{name: "nothing in the key", wantEntry: false},
		{name: "Authorization", hasAuthorization: true, wantEntry: true, wantKeyed: true},
		{
			name: "KeyCookies cookie present", cfg: Config{KeyCookies: []string{"session"}},
			headers: map[string]string{"Cookie": "session=abc"}, wantEntry: true, wantKeyed: true,
		},
		{name: "KeyCookies cookie absent", cfg: Config{KeyCookies: []string{"session"}}},
		{
			name: "recognized header in KeyHeaders present", cfg: Config{KeyHeaders: []string{"X-API-Key"}},
			headers: map[string]string{"X-API-Key": "k"}, wantEntry: true, wantKeyed: true,
		},
		{name: "recognized header in KeyHeaders absent", cfg: Config{KeyHeaders: []string{"X-API-Key"}}},
		{
			name: "header in KeyHeaders that is not a credential", cfg: Config{KeyHeaders: []string{"X-Tenant"}},
			headers: map[string]string{"X-Tenant": "acme"},
		},
		{
			name: "declared header present", cfg: Config{KeyCredentialHeaders: []string{"X-Company-Token"}},
			headers: map[string]string{"X-Company-Token": "t"}, wantEntry: true, wantKeyed: true,
		},
		{
			name: "Vary on a credential, key without one", varyNames: []string{"cookie", "x-variant"},
			wantEntry: true, wantRecord: true,
		},
		{
			name: "Vary on a declared header", cfg: Config{KeyCredentialHeaders: []string{"X-Company-Token"}},
			varyNames: []string{"x-company-token"}, wantEntry: true, wantRecord: true,
		},
		{name: "Vary on a header that is not a credential", varyNames: []string{"x-tenant"}},
		{
			name: "Vary on a credential the key holds", cfg: Config{KeyCookies: []string{"session"}},
			headers: map[string]string{"Cookie": "session=abc"}, varyNames: []string{"cookie"},
			wantEntry: true, wantKeyed: true,
		},
		{
			name:      "custom KeyGenerator is assumed to hold one",
			cfg:       Config{KeyGenerator: func(c fiber.Ctx) string { return c.Path() }},
			wantEntry: true,
		},
		{
			name: "custom KeyGenerator and a Vary on a credential",
			cfg:  Config{KeyGenerator: func(c fiber.Ctx) string { return c.Path() }}, varyNames: []string{"cookie"},
			wantEntry: true, wantRecord: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := configDefault(tc.cfg)
			c := fiber.New().AcquireCtx(&fasthttp.RequestCtx{})
			for name, value := range tc.headers {
				c.Request().Header.Set(name, value)
			}

			// The refusal key exists when the responses vary on a credential, as it
			// does for the handler once an entry is found.
			refusal := ""
			if cfg.varyCarriesCredential(tc.varyNames) {
				refusal = cfg.refusalKey(c, "base", tc.varyNames)
			}

			scope := cfg.scopeOf(c, tc.hasAuthorization, refusal)
			require.Equal(t, tc.wantEntry, scope.entry, "entry")
			require.Equal(t, tc.wantKeyed, scope.keyed, "keyed")
			require.Equal(t, tc.wantRecord, scope.record != "", "record")
		})
	}
}

func Test_refusesCredential(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		status int
		want   bool
	}{
		{status: fiber.StatusUnauthorized, want: true},
		{status: fiber.StatusForbidden, want: true},
		{status: fiber.StatusProxyAuthRequired, want: true},
		{status: fiber.StatusOK},
		{status: fiber.StatusFound},
		{status: fiber.StatusNotFound},
		{status: fiber.StatusTooManyRequests},
		{status: fiber.StatusServiceUnavailable},
	} {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, refusesCredential(tc.status))
		})
	}
}

func Test_errorRefusesCredential(t *testing.T) {
	t.Parallel()

	// A nil *fiber.Error held in an error matches errors.As, and must not be read.
	var typedNil *fiber.Error

	for _, tc := range []struct {
		err  error
		name string
		want bool
	}{
		{name: "nil"},
		{name: "401", err: fiber.ErrUnauthorized, want: true},
		{name: "403", err: fiber.ErrForbidden, want: true},
		{name: "407", err: fiber.ErrProxyAuthRequired, want: true},
		{name: "custom message", err: fiber.NewError(fiber.StatusForbidden, "tenant suspended"), want: true},
		{name: "wrapped", err: fmt.Errorf("session: %w", fiber.ErrUnauthorized), want: true},
		{name: "joined", err: errors.Join(errors.New("audit failed"), fiber.ErrForbidden), want: true},
		{name: "503", err: fiber.ErrServiceUnavailable},
		{name: "404", err: fiber.ErrNotFound},
		{name: "not a fiber error", err: errors.New("unauthorized")},
		{name: "typed nil", err: typedNil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, errorRefusesCredential(tc.err))
		})
	}
}

func Test_varyListsCredential(t *testing.T) {
	t.Parallel()

	tests := []struct {
		vary     string
		declared []string
		want     bool
	}{
		{vary: "", want: false},
		{vary: "Accept", want: false},
		{vary: "Accept-Encoding, Accept-Language", want: false},
		{vary: "*", want: false},
		{vary: "Cookie", want: true},
		{vary: "cookie", want: true},
		{vary: "COOKIE", want: true},
		{vary: " Cookie ", want: true},
		{vary: "Accept-Encoding, Cookie", want: true},
		{vary: "Accept-Encoding,Cookie", want: true},
		{vary: "Authorization", want: true},
		{vary: "authorization", want: true},
		// The other headers that carry nothing but a credential.
		{vary: "Proxy-Authorization", want: true},
		{vary: "X-API-Key", want: true},
		{vary: "Accept, x-auth-token", want: true},
		{vary: "X-Access-Token", want: true},
		// Names that merely contain one of them are other fields.
		{vary: "X-Cookie", want: false},
		{vary: "Cookie2", want: false},
		{vary: "Set-Cookie", want: false},
		{vary: "X-Authorization", want: false},
		// A header is a credential only when it is declared.
		{vary: "X-Company-Token", want: false},
		{vary: "X-Company-Token", declared: []string{"x-company-token"}, want: true},
		{vary: "Accept, X-COMPANY-TOKEN", declared: []string{"x-company-token"}, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.vary, func(t *testing.T) {
			t.Parallel()
			cfg := Config{KeyCredentialHeaders: tc.declared}
			require.Equal(t, tc.want, cfg.varyListsCredential([]byte(tc.vary)), "Vary: %q", tc.vary)
		})
	}
}
