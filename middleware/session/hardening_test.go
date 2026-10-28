package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
)

// lifetimeApp builds an app over a recording storage. /new stores a value;
// /deadline moves the session's absolute deadline d from now, which is how a
// test gets a session that is close to, or past, its end; /read only reads.
func lifetimeApp(cfg *Config) (*fiber.App, *recordingStorage) {
	rec := newRecordingStorage()
	cfg.Storage = rec

	app := fiber.New()
	app.Use(New(*cfg))
	app.Get("/new", func(c fiber.Ctx) error {
		FromContext(c).Set("k", "v")
		return nil
	})
	app.Get("/deadline", func(c fiber.Ctx) error {
		d, err := time.ParseDuration(c.Query("d"))
		if err != nil {
			return fiber.ErrBadRequest
		}
		m := FromContext(c)
		m.Set("k", "v")
		m.Session.setAbsExpiration(time.Now().Add(d))
		return nil
	})
	app.Get("/read", func(c fiber.Ctx) error {
		_ = FromContext(c).Get("k")
		return nil
	})
	return app, rec
}

func lastWrite(t *testing.T, rec *recordingStorage) storageWrite {
	t.Helper()
	writes := rec.writes()
	require.NotEmpty(t, writes)
	return writes[len(writes)-1]
}

// The record and the cookie must not outlive the absolute deadline: a session
// that is about to end is stored, and its cookie set, for the time it has left.
func Test_Session_Lifetime_CappedByAbsoluteDeadline(t *testing.T) {
	t.Parallel()

	t.Run("without an absolute timeout it is the idle timeout, as before", func(t *testing.T) {
		t.Parallel()
		app, rec := lifetimeApp(&Config{IdleTimeout: 10 * time.Minute})

		resp, _ := sendRequest(t, app, "/new", nil)
		require.Equal(t, 10*time.Minute, lastWrite(t, rec).exp)
		require.Equal(t, 600, cookiesByName(resp)["session_id"].MaxAge)
	})

	t.Run("a new session is cut to the time left", func(t *testing.T) {
		t.Parallel()
		app, rec := lifetimeApp(&Config{IdleTimeout: time.Hour, AbsoluteTimeout: time.Hour})

		resp, _ := sendRequest(t, app, "/new", nil)
		exp := lastWrite(t, rec).exp
		require.LessOrEqual(t, exp, time.Hour)
		require.Greater(t, exp, time.Hour-10*time.Second)
		maxAge := cookiesByName(resp)["session_id"].MaxAge
		require.LessOrEqual(t, maxAge, 3600)
		require.Greater(t, maxAge, 3590)
	})

	t.Run("an idle timeout shorter than what is left wins", func(t *testing.T) {
		t.Parallel()
		app, rec := lifetimeApp(&Config{IdleTimeout: 10 * time.Minute, AbsoluteTimeout: 2 * time.Hour})

		resp, _ := sendRequest(t, app, "/new", nil)
		require.Equal(t, 10*time.Minute, lastWrite(t, rec).exp)
		require.Equal(t, 600, cookiesByName(resp)["session_id"].MaxAge)
	})

	t.Run("close to the deadline", func(t *testing.T) {
		t.Parallel()
		app, rec := lifetimeApp(&Config{IdleTimeout: 30 * time.Minute, AbsoluteTimeout: time.Hour})

		resp, _ := sendRequest(t, app, "/deadline?d=5m", nil)
		exp := lastWrite(t, rec).exp
		require.LessOrEqual(t, exp, 5*time.Minute)
		require.Greater(t, exp, 5*time.Minute-10*time.Second)
		maxAge := cookiesByName(resp)["session_id"].MaxAge
		require.LessOrEqual(t, maxAge, 300)
		require.Greater(t, maxAge, 290)
	})

	t.Run("a deadline within a second or already behind is one second, never none", func(t *testing.T) {
		t.Parallel()
		for _, d := range []string{"200ms", "0s", "-1m", "-720h"} {
			app, rec := lifetimeApp(&Config{IdleTimeout: 30 * time.Minute, AbsoluteTimeout: time.Hour})

			resp, _ := sendRequest(t, app, "/deadline?d="+d, nil)
			require.Equal(t, time.Second, lastWrite(t, rec).exp,
				"a lifetime of zero or less would read as 'never expires' in many storages (deadline %s)", d)
			require.Equal(t, 1, cookiesByName(resp)["session_id"].MaxAge)
		}
	})

	t.Run("a session-only cookie has no lifetime but the record is still cut", func(t *testing.T) {
		t.Parallel()
		app, rec := lifetimeApp(&Config{IdleTimeout: 30 * time.Minute, AbsoluteTimeout: time.Hour, CookieSessionOnly: true})

		resp, _ := sendRequest(t, app, "/deadline?d=5m", nil)
		require.LessOrEqual(t, lastWrite(t, rec).exp, 5*time.Minute)
		cookie := cookiesByName(resp)["session_id"]
		require.Zero(t, cookie.MaxAge)
		require.True(t, cookie.Expires.IsZero())
	})

	t.Run("a session that was only read is cut as well", func(t *testing.T) {
		t.Parallel()
		app, rec := lifetimeApp(&Config{IdleTimeout: 30 * time.Minute, AbsoluteTimeout: time.Hour})

		resp, _ := sendRequest(t, app, "/deadline?d=5m", nil)
		id := cookiesByName(resp)["session_id"].Value

		resp, _ = sendRequest(t, app, "/read", map[string]string{"session_id": id})
		exp := lastWrite(t, rec).exp
		require.LessOrEqual(t, exp, 5*time.Minute, "the snapshot is written back with the capped lifetime")
		require.Greater(t, exp, 4*time.Minute)
		require.LessOrEqual(t, cookiesByName(resp)["session_id"].MaxAge, 300)
	})
}

func Test_Session_Lifetime_Unit(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		idle     time.Duration
		absolute time.Duration
		deadline time.Duration // from now; ignored when noMarker
		noMarker bool
		want     time.Duration
		slack    time.Duration
	}{
		{name: "no absolute timeout", idle: 10 * time.Minute, want: 10 * time.Minute},
		{name: "deadline far away", idle: 10 * time.Minute, absolute: time.Hour, deadline: 2 * time.Hour, want: 10 * time.Minute},
		{name: "deadline nearer than idle", idle: 10 * time.Minute, absolute: time.Hour, deadline: 3 * time.Minute, want: 3 * time.Minute, slack: 2 * time.Second},
		{name: "deadline passed", idle: 10 * time.Minute, absolute: time.Hour, deadline: -time.Hour, want: time.Second},
		{name: "deadline within the minimum", idle: 10 * time.Minute, absolute: time.Hour, deadline: 500 * time.Millisecond, want: time.Second},
		{name: "an idle timeout under the minimum is left alone", idle: 500 * time.Millisecond, absolute: time.Hour, deadline: time.Hour, want: 500 * time.Millisecond},
		{name: "no marker stored", idle: 10 * time.Minute, absolute: time.Hour, noMarker: true, want: 10 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sess := newSessionWithStorage(t, memory.New())
			sess.config.AbsoluteTimeout = tc.absolute
			sess.idleTimeout = tc.idle
			if tc.absolute > 0 && !tc.noMarker {
				sess.setAbsExpiration(time.Now().Add(tc.deadline))
			}
			require.InDelta(t, float64(tc.want), float64(sess.lifetime()), float64(tc.slack))
		})
	}
}

// mapStorage is a storage of a nilable kind that is not a pointer.
type mapStorage map[string][]byte

func (mapStorage) GetWithContext(context.Context, string) ([]byte, error) { return nil, nil }

func (mapStorage) Get(string) ([]byte, error) { return nil, nil }

func (mapStorage) SetWithContext(context.Context, string, []byte, time.Duration) error { return nil }

func (mapStorage) Set(string, []byte, time.Duration) error { return nil }

func (mapStorage) DeleteWithContext(context.Context, string) error { return nil }

func (mapStorage) Delete(string) error { return nil }

func (mapStorage) ResetWithContext(context.Context) error { return nil }

func (mapStorage) Reset() error { return nil }

func (mapStorage) Close() error { return nil }

func Test_Config_TypedNilStorage(t *testing.T) {
	t.Parallel()

	const message = "[session] Storage must not be a typed nil"

	var nilPointer *memory.Storage
	var nilMap mapStorage

	t.Run("a nil pointer is refused at construction", func(t *testing.T) {
		t.Parallel()
		require.PanicsWithValue(t, message, func() { NewStore(Config{Storage: nilPointer}) })
		require.PanicsWithValue(t, message, func() { New(Config{Storage: nilPointer}) })
		require.PanicsWithValue(t, message, func() { NewWithStore(Config{Storage: nilPointer}) })
	})

	t.Run("so is a nil map", func(t *testing.T) {
		t.Parallel()
		require.PanicsWithValue(t, message, func() { NewStore(Config{Storage: nilMap}) })
	})

	t.Run("a plain nil stands for the default", func(t *testing.T) {
		t.Parallel()
		require.NotPanics(t, func() { NewStore(Config{Storage: nil}) })
		require.NotPanics(t, func() { New() })
	})

	t.Run("a storage that is there is fine", func(t *testing.T) {
		t.Parallel()
		require.NotPanics(t, func() { NewStore(Config{Storage: memory.New()}) })
		require.NotPanics(t, func() { NewStore(Config{Storage: mapStorage{}}) }, "an empty map is not nil")
	})
}

// Not parallel: the log output is process-global, so these tests capture it.
func Test_Config_CookiePrefixWarnings(t *testing.T) {
	for _, tc := range []struct {
		name string
		want []string // fragments the warning must contain; none means no warning at all
		cfg  Config
	}{
		{name: "__Host- over plain defaults", cfg: Config{Extractor: extractors.FromCookie("__Host-sid")}, want: []string{"__Host-", "Secure=false"}},
		{name: "__Host- that meets its rules", cfg: Config{Extractor: extractors.FromCookie("__Host-sid"), CookieSecure: true, CookiePath: "/"}},
		{name: "__Host- with a domain", cfg: Config{Extractor: extractors.FromCookie("__Host-sid"), CookieSecure: true, CookiePath: "/", CookieDomain: "example.com"}, want: []string{"__Host-", `Domain="example.com"`}},
		{name: "__Host- without the path", cfg: Config{Extractor: extractors.FromCookie("__Host-sid"), CookieSecure: true}, want: []string{"__Host-", `Path=""`}},
		{name: "__Host- on another path", cfg: Config{Extractor: extractors.FromCookie("__Host-sid"), CookieSecure: true, CookiePath: "/app"}, want: []string{"__Host-", `Path="/app"`}},
		{name: "__Secure- over plain defaults", cfg: Config{Extractor: extractors.FromCookie("__Secure-sid")}, want: []string{"__Secure-"}},
		{name: "__Secure- with Secure", cfg: Config{Extractor: extractors.FromCookie("__Secure-sid"), CookieSecure: true}},
		{name: "__Secure- with SameSite=None, which forces Secure", cfg: Config{Extractor: extractors.FromCookie("__Secure-sid"), CookieSameSite: "None"}},
		{name: "the prefixes are case-insensitive", cfg: Config{Extractor: extractors.FromCookie("__host-sid")}, want: []string{"__host-"}},
		{name: "a header is not a cookie", cfg: Config{Extractor: extractors.FromHeader("__Host-sid")}},
		{name: "an ordinary cookie", cfg: Config{}},
		{name: "a name that merely contains the prefix", cfg: Config{Extractor: extractors.FromCookie("my__Host-sid")}},
		{
			name: "one bad cookie in a chain",
			cfg: Config{Extractor: extractors.Chain(
				extractors.FromCookie("session_id"),
				extractors.FromCookie("__Host-sid"),
				extractors.FromHeader("X-Session"),
			)},
			want: []string{`"__Host-sid"`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buf := loggertest.CaptureContextLog(t, "")
			NewStore(tc.cfg)

			out := buf.String()
			if len(tc.want) == 0 {
				require.Empty(t, out)
				return
			}
			require.Equal(t, 1, strings.Count(out, "[Warn]"), "one warning per offending cookie: %q", out)
			for _, fragment := range tc.want {
				require.Contains(t, out, fragment)
			}
		})
	}

	t.Run("one warning per store, however the store is built", func(t *testing.T) {
		cfg := Config{Extractor: extractors.FromCookie("__Host-sid")}

		buf := loggertest.CaptureContextLog(t, "")
		_, _ = NewWithStore(cfg)
		require.Equal(t, 1, strings.Count(buf.String(), "[Warn]"), "NewWithStore builds its store once")

		buf = loggertest.CaptureContextLog(t, "")
		store := NewStore(cfg)
		require.Equal(t, 1, strings.Count(buf.String(), "[Warn]"))

		buf = loggertest.CaptureContextLog(t, "")
		_ = New(Config{Store: store})
		require.Empty(t, buf.String(), "a store that was built before is not checked again")
	})
}
