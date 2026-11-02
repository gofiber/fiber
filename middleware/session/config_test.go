package session

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
)

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func TestConfigDefault(t *testing.T) {
	cfg := configDefault()
	require.Equal(t, 30*time.Minute, cfg.IdleTimeout)
	require.NotNil(t, cfg.KeyGenerator)
	require.NotNil(t, cfg.Extractor)
	require.Equal(t, "session_id", cfg.Extractor.Key)
	require.Equal(t, "Lax", cfg.CookieSameSite)
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func TestConfigDefaultWithCustomConfig(t *testing.T) {
	customConfig := Config{
		IdleTimeout:  48 * time.Hour,
		Extractor:    extractors.FromHeader("X-Custom-Session"),
		KeyGenerator: func() string { return "custom_key" },
	}
	cfg := configDefault(customConfig)
	require.Equal(t, 48*time.Hour, cfg.IdleTimeout)
	require.NotNil(t, cfg.KeyGenerator)
	require.NotNil(t, cfg.Extractor)
	require.Equal(t, "X-Custom-Session", cfg.Extractor.Key)
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func TestDefaultErrorHandler(t *testing.T) {
	ctx := apptest.Ctx(t, fiber.New())
	DefaultErrorHandler(ctx, fiber.ErrInternalServerError)
	require.Equal(t, fiber.StatusInternalServerError, ctx.Response().StatusCode())
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func TestAbsoluteTimeoutValidation(t *testing.T) {
	require.PanicsWithValue(t, "[session] AbsoluteTimeout must be greater than or equal to IdleTimeout", func() {
		configDefault(Config{
			IdleTimeout:     30 * time.Minute,
			AbsoluteTimeout: 15 * time.Minute, // Less than IdleTimeout
		})
	})
}

// TestCookieSameSiteValidation verifies that initialization accepts every
// supported mode and rejects unsupported non-empty values.
func TestCookieSameSiteValidation(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "disabled", "LAX", "Strict", "none"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			require.NotPanics(t, func() { configDefault(Config{CookieSameSite: value}) })
		})
	}

	require.PanicsWithValue(
		t,
		"[session] CookieSameSite must be one of Disabled, Lax, Strict, or None",
		func() { NewStore(Config{CookieSameSite: "invalid"}) },
	)
}

// mapStorage is a storage of a nilable kind that is not a pointer.
// conventions:ignore storage-fake: the fakes of storagetest are structs, and this one has to be a map
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

	t.Run("a nil pointer or a nil map is refused at construction", func(t *testing.T) {
		t.Parallel()
		require.PanicsWithValue(t, message, func() { NewStore(Config{Storage: nilPointer}) })
		require.PanicsWithValue(t, message, func() { New(Config{Storage: nilPointer}) })
		require.PanicsWithValue(t, message, func() { NewWithStore(Config{Storage: nilPointer}) })
		require.PanicsWithValue(t, message, func() { NewStore(Config{Storage: nilMap}) })
	})

	t.Run("a plain nil, a storage that is there and a storage nobody asks for are fine", func(t *testing.T) {
		t.Parallel()
		store := NewStore()
		require.NotPanics(t, func() { NewStore(Config{Storage: nil}) }, "a plain nil stands for the default")
		require.NotPanics(t, func() { New() })
		require.NotPanics(t, func() { NewStore(Config{Storage: memory.New()}) })
		require.NotPanics(t, func() { NewStore(Config{Storage: mapStorage{}}) }, "an empty map is not nil")
		require.NotPanics(t, func() { New(Config{Store: store, Storage: nilPointer}) },
			"the store was built before, so this Storage is never touched")
		require.NotPanics(t, func() { NewWithStore(Config{Store: store, Storage: nilMap}) })
	})
}

// warningCase is a Config to make a store or a middleware from, and the warning that logs.
type warningCase struct {
	name string
	want []string // fragments the warning must contain; none means no warning at all
	cfg  Config
}

// requireWarnings makes something from the Config of every case with the log captured, and checks
// the log: nothing when want is empty, and otherwise one warning with every fragment of want in it.
//
// conventions:ignore parallel: the log output is process-global, so these tests capture it
func requireWarnings(t *testing.T, build func(Config), cases []warningCase) {
	t.Helper()

	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			buf := loggertest.CaptureContextLog(t, "")
			build(tc.cfg)
			out := buf.String()
			if len(tc.want) == 0 {
				require.Empty(t, out)
				return
			}
			require.Equal(t, 1, strings.Count(out, "[Warn]"), "one warning: %q", out)
			for _, fragment := range tc.want {
				require.Contains(t, out, fragment)
			}
		})
	}
}

// conventions:ignore parallel: the log output is process-global, so this test captures it
func Test_Config_CookiePrefixWarnings(t *testing.T) {
	requireWarnings(t, func(cfg Config) { NewStore(cfg) }, []warningCase{
		{
			name: "__Host- over plain defaults",
			cfg:  Config{Extractor: extractors.FromCookie("__Host-sid")},
			want: []string{"__Host-", "Secure=false"},
		},
		{
			name: "__Host- that meets its rules",
			cfg:  Config{Extractor: extractors.FromCookie("__Host-sid"), CookieSecure: true, CookiePath: "/"},
		},
		{
			name: "__Host- with a domain",
			cfg:  Config{Extractor: extractors.FromCookie("__Host-sid"), CookieSecure: true, CookiePath: "/", CookieDomain: "example.com"},
			want: []string{"__Host-", `Domain="example.com"`},
		},
		{
			name: "__Host- with the default path, which is sent as Path=/",
			cfg:  Config{Extractor: extractors.FromCookie("__Host-sid"), CookieSecure: true},
		},
		{
			name: "__Host- with SameSite=None, which forces Secure",
			cfg:  Config{Extractor: extractors.FromCookie("__Host-sid"), CookieSameSite: "None"},
		},
		{
			name: "__Host- on another path",
			cfg:  Config{Extractor: extractors.FromCookie("__Host-sid"), CookieSecure: true, CookiePath: "/app"},
			want: []string{"__Host-", `Path="/app"`},
		},
		{
			name: "__Secure- over plain defaults",
			cfg:  Config{Extractor: extractors.FromCookie("__Secure-sid")},
			want: []string{"__Secure-"},
		},
		{
			name: "__Secure- with Secure",
			cfg:  Config{Extractor: extractors.FromCookie("__Secure-sid"), CookieSecure: true},
		},
		{
			name: "__Secure- with SameSite=None, which forces Secure",
			cfg:  Config{Extractor: extractors.FromCookie("__Secure-sid"), CookieSameSite: "None"},
		},
		{
			name: "the prefixes are case-insensitive",
			cfg:  Config{Extractor: extractors.FromCookie("__host-sid")},
			want: []string{"__host-"},
		},
		{name: "a header is not a cookie", cfg: Config{Extractor: extractors.FromHeader("__Host-sid")}},
		{name: "an ordinary cookie", cfg: Config{}},
		{name: "a name that merely contains the prefix", cfg: Config{Extractor: extractors.FromCookie("my__Host-sid")}},
		{
			name: "one bad cookie in a chain",
			cfg:  Config{Extractor: extractors.Chain(extractors.FromCookie("session_id"), extractors.FromCookie("__Host-sid"), extractors.FromHeader("X-Session"))},
			want: []string{`"__Host-sid"`},
		},
	})

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

// Without a Config there is nothing the application set on it to warn about, whatever ConfigDefault
// holds, a Store included.
//
// conventions:ignore parallel: it assigns ConfigDefault, which every test reads, and captures the log output
func Test_Config_StoreOptionWarnings_WithoutAConfig(t *testing.T) {
	saved := ConfigDefault
	t.Cleanup(func() { ConfigDefault = saved })
	ConfigDefault.Store = NewStore(Config{CookieSecure: true})

	buf := loggertest.CaptureContextLog(t, "")
	require.NotPanics(t, func() { New() })
	require.Empty(t, buf.String())
}

// conventions:ignore parallel: the log output is process-global, so this test captures it
func Test_Config_StoreOptionWarnings(t *testing.T) {
	hashed := NewStore(Config{HashStorageKeys: true, StorageKeyPrefix: "app:"})
	skipping := NewStore(Config{SkipEmptySessions: true})
	withCodec := NewStore(Config{Codec: JSONCodec{}})
	secure := NewStore(Config{CookieSecure: true})
	timed := NewStore(Config{IdleTimeout: time.Hour})
	configured := NewStore(Config{
		AbsoluteTimeout: 2 * time.Hour, CookieDomain: "example.com", CookiePath: "/app", CookieSameSite: "Strict",
		CookieSecure: true, CookieHTTPOnly: true, CookieSessionOnly: true,
	})
	plain := NewStore()

	requireWarnings(t, func(cfg Config) { New(cfg) }, []warningCase{
		{
			name: "without a store, the options are the middleware's own",
			cfg:  Config{HashStorageKeys: true, StorageKeyPrefix: "app:", SkipEmptySessions: true},
		},
		{name: "options set on the store only", cfg: Config{Store: hashed}},
		{
			name: "hashing asked of the middleware",
			cfg:  Config{Store: plain, HashStorageKeys: true},
			want: []string{"taken from Config.Store", "HashStorageKeys=false"},
		},
		{
			name: "a prefix asked of the middleware",
			cfg:  Config{Store: plain, StorageKeyPrefix: "app:"},
			want: []string{"taken from Config.Store", `StorageKeyPrefix=""`},
		},
		{name: "the same prefix on both", cfg: Config{Store: hashed, StorageKeyPrefix: "app:"}},
		{
			name: "another prefix on the middleware",
			cfg:  Config{Store: hashed, StorageKeyPrefix: "other:"},
			want: []string{"taken from Config.Store", `StorageKeyPrefix="app:"`},
		},
		{
			name: "skipping asked of the store only",
			cfg:  Config{Store: skipping},
			want: []string{"SkipEmptySessions is taken from the Config of the middleware"},
		},
		{name: "skipping asked of the middleware", cfg: Config{Store: plain, SkipEmptySessions: true}},
		{name: "skipping asked of both", cfg: Config{Store: skipping, SkipEmptySessions: true}},
		{name: "the codec is the store's own", cfg: Config{Store: withCodec}},
		{name: "the same codec on both", cfg: Config{Store: withCodec, Codec: JSONCodec{}}},
		{
			name: "a codec asked of the middleware only",
			cfg:  Config{Store: plain, Codec: JSONCodec{}},
			want: []string{"Codec is taken from Config.Store"},
		},
		{
			name: "another codec on the middleware",
			cfg:  Config{Store: withCodec, Codec: NewCodec("other", json.Marshal, json.Unmarshal)},
			want: []string{"Codec is taken from Config.Store"},
		},
		{name: "no store, the codec is the middleware's", cfg: Config{Codec: JSONCodec{}}},
		{
			name: "the cookie options asked of the middleware",
			cfg:  Config{Store: plain, CookiePath: "/app", CookieSecure: true},
			want: []string{"come from the store", "CookiePath, CookieSecure"},
		},
		{name: "the same cookie option on both", cfg: Config{Store: secure, CookieSecure: true}},
		{
			name: "the other cookie options asked of the middleware",
			cfg:  Config{Store: plain, CookieDomain: "example.com", CookieSameSite: "Strict", CookieHTTPOnly: true, CookieSessionOnly: true},
			want: []string{"come from the store", "CookieDomain, CookieSameSite, CookieHTTPOnly, CookieSessionOnly"},
		},
		{
			name: "every option the store has set the same on both",
			cfg: Config{
				Store: configured, AbsoluteTimeout: 2 * time.Hour, CookieDomain: "example.com", CookiePath: "/app",
				CookieSameSite: "strict", CookieSecure: true, CookieHTTPOnly: true, CookieSessionOnly: true,
			},
		},
		{name: "hashing asked of both", cfg: Config{Store: hashed, HashStorageKeys: true}},
		{
			name: "the timeouts asked of the middleware",
			cfg:  Config{Store: plain, IdleTimeout: time.Hour, AbsoluteTimeout: 2 * time.Hour},
			want: []string{"come from the store", "IdleTimeout, AbsoluteTimeout"},
		},
		{name: "the same timeout on both", cfg: Config{Store: timed, IdleTimeout: time.Hour}},
		{
			name: "storage, extractor and key generator asked of the middleware",
			cfg: Config{
				Store: plain, Storage: memory.New(), Extractor: extractors.FromCookie("sid"),
				KeyGenerator: func() string { return "id" },
			},
			want: []string{"come from the store", "Storage, Extractor, KeyGenerator"},
		},
		{
			name: "the defaults are not options",
			cfg:  Config{Store: plain, IdleTimeout: 30 * time.Minute, CookieSameSite: "lax"},
		},
	})
}
