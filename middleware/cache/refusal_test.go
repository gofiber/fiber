package cache

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// credentialPartition is a way the cache tells sessions apart, and how the fixture sends one.
type credentialPartition struct {
	configure    func(*Config)               // makes the cache tell sessions apart
	send         func(*http.Request, string) // puts a session on a request
	read         func(fiber.Ctx) string      // takes it off again
	vary         string                      // the Vary the app adds, if any
	cacheControl string                      // the Cache-Control the app adds, if any
}

func sendCookie(req *http.Request, session string) {
	req.Header.Set(fiber.HeaderCookie, "session="+session)
}

func readCookie(c fiber.Ctx) string { return c.Cookies("session") }

func sendBearer(req *http.Request, session string) {
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+session)
}

func readBearer(c fiber.Ctx) string {
	return strings.TrimPrefix(c.Get(fiber.HeaderAuthorization), "Bearer ")
}

func headerPartition(name string, configure func(*Config)) credentialPartition {
	return credentialPartition{
		configure: configure,
		send:      func(req *http.Request, session string) { req.Header.Set(name, session) },
		read:      func(c fiber.Ctx) string { return c.Get(name) },
	}
}

var credentialPartitions = map[string]credentialPartition{
	"keyCookies": {
		configure: func(cfg *Config) { cfg.KeyCookies = []string{"session"} },
		send:      sendCookie,
		read:      readCookie,
	},
	"varyCookie": {send: sendCookie, read: readCookie, vary: fiber.HeaderCookie},
	"authorization": {
		send: sendBearer,
		read: readBearer,
		// Stored only when the response says a shared cache may.
		cacheControl: "public, max-age=60",
	},
	// An API key, which the cache takes for a credential by name.
	"apiKey":           headerPartition("X-Api-Key", func(cfg *Config) { cfg.KeyHeaders = []string{"X-Api-Key"} }),
	"credentialHeader": headerPartition("X-Company-Token", func(cfg *Config) { cfg.KeyCredentialHeaders = []string{"X-Company-Token"} }),
	// Varies on a header the cache recognizes, whatever the key is built from.
	"varyApiKey": func() credentialPartition {
		p := headerPartition("X-Api-Key", nil)
		p.vary = "X-Api-Key"
		return p
	}(),
	"customKey": headerPartition("X-Company-Token", func(cfg *Config) {
		cfg.KeyCredentialHeaders = []string{"X-Company-Token"}
		cfg.KeyGenerator = func(c fiber.Ctx) string { return c.Path() + "|" + c.Get("X-Company-Token") }
	}),
	// Headers nothing marks as a credential.
	"tenant":       headerPartition("X-Tenant", func(cfg *Config) { cfg.KeyHeaders = []string{"X-Tenant"} }),
	"customHeader": headerPartition("X-Session", func(cfg *Config) { cfg.KeyHeaders = []string{"X-Session"} }),
	"customVary": func() credentialPartition {
		p := headerPartition("X-Session", nil)
		p.vary = "X-Session"
		return p
	}(),
	// A custom key, which holds Authorization as every key does.
	"customAuthorization": {
		configure:    func(cfg *Config) { cfg.KeyGenerator = pathKey },
		send:         sendBearer,
		read:         readBearer,
		cacheControl: "public, max-age=60",
	},
}

const fixturePath = "/account"

// credentialFixture is an app behind the cache that answers per session and hands any other to refuse.
type credentialFixture struct {
	app           *fiber.App
	t             *testing.T
	probe         *accountingProbe
	ended         sync.Map
	variantHeader string
	partition     credentialPartition
	calls         atomic.Int32
	noStore       atomic.Bool // the answer to a valid session says not to keep it
}

type credentialOptions struct {
	custom fiber.Storage         // used instead of storage when set
	refuse func(fiber.Ctx) error // the answer to a session that is not valid, a bare 401 when nil
	// clock and expiration push an entry past its lifetime while storage still holds it.
	clock *testClock
	// beforeRefusal runs in the origin before it refuses a session, as a concurrent request would.
	beforeRefusal func(*credentialFixture)
	partition     string // a key of credentialPartitions
	storage       string // memory or external
	vary          string // a Vary the served answer carries besides the partition's
	variantHeader string // the header a variant is sent in, X-Variant when empty
	// invalidateOnly limits invalidateEnded to the requests for this variant.
	invalidateOnly string
	valid          []string // the sessions the app answers, "a" and "b" when nil
	expiration     time.Duration
	recordCap      int  // how many refusal records an in-memory cache keeps, derived from MaxBytes when zero
	allowStale     bool // AllowStaleCredentialResponses
	// invalidateEnded has CacheInvalidator report the sessions the app has ended.
	invalidateEnded bool
}

var (
	reloadForms     = []string{"max-age=0", "no-cache"}
	refusalStatuses = []int{fiber.StatusUnauthorized, fiber.StatusForbidden, fiber.StatusProxyAuthRequired}
)

func newCredentialFixture(t *testing.T, opts *credentialOptions) *credentialFixture {
	t.Helper()

	partition, ok := credentialPartitions[opts.partition]
	require.True(t, ok, "unknown partition %q", opts.partition)

	valid := opts.valid
	if valid == nil {
		valid = []string{"a", "b"}
	}
	refuse := opts.refuse
	if refuse == nil {
		refuse = refusal(fiber.StatusUnauthorized, "bare")
	}

	f := &credentialFixture{t: t, probe: &accountingProbe{}, partition: partition, variantHeader: opts.variantHeader}
	if f.variantHeader == "" {
		f.variantHeader = "X-Variant"
	}
	// A byte limit turns on the heap accounting a purge has to keep honest.
	cfg := Config{MaxBytes: 1 << 20, accounting: f.probe.record, Expiration: opts.expiration, AllowStaleCredentialResponses: opts.allowStale, refusalRecordLimit: opts.recordCap}
	if opts.clock != nil {
		cfg.clock = opts.clock.Now
	}
	if partition.configure != nil {
		partition.configure(&cfg)
	}
	cfg.Storage = opts.custom
	if cfg.Storage == nil {
		cfg.Storage = storageOf(opts.storage)
	}
	if opts.invalidateEnded {
		cfg.CacheInvalidator = func(c fiber.Ctx) bool {
			_, ended := f.ended.Load(partition.read(c))
			return ended && (opts.invalidateOnly == "" || c.Get(f.variantHeader) == opts.invalidateOnly)
		}
	}

	f.app = fiber.New()
	f.app.Use(New(cfg))
	f.app.Get(fixturePath, func(c fiber.Ctx) error {
		f.calls.Add(1)
		session := partition.read(c)
		if _, ended := f.ended.Load(session); ended || !slices.Contains(valid, session) {
			if opts.beforeRefusal != nil {
				opts.beforeRefusal(f)
			}
			return refuse(c)
		}

		vary := slices.DeleteFunc([]string{partition.vary, opts.vary}, func(name string) bool { return name == "" })
		if len(vary) > 0 {
			c.Set(fiber.HeaderVary, strings.Join(vary, ", "))
		}
		if partition.cacheControl != "" {
			c.Set(fiber.HeaderCacheControl, partition.cacheControl)
		}
		if f.noStore.Load() {
			c.Set(fiber.HeaderCacheControl, "no-store")
		}
		return c.SendString(accountBody(session, c.Get(f.variantHeader)))
	})
	return f
}

// get requests as session, as the partition carries it; an empty session sends nothing.
func (f *credentialFixture) get(session, variant, cacheControl string) reply {
	f.t.Helper()

	return f.send(fiber.MethodGet, session, variant, cacheControl)
}

// send is get with another method.
func (f *credentialFixture) send(method, session, variant, cacheControl string) reply {
	f.t.Helper()

	req := httptest.NewRequest(method, fixturePath, http.NoBody)
	if session != "" {
		f.partition.send(req, session)
	}
	if variant != "" {
		req.Header.Set(f.variantHeader, variant)
	}
	if cacheControl != "" {
		req.Header.Set(fiber.HeaderCacheControl, cacheControl)
	}
	return doRequest(f.t, f.app, req)
}

func (f *credentialFixture) requireHit(session, variant string) {
	f.t.Helper()

	got := f.get(session, variant, "")
	require.Equal(f.t, fiber.StatusOK, got.status)
	require.Equal(f.t, accountBody(session, variant), got.body)
	require.Equal(f.t, cacheHit, got.cache)
}

// warm sends each session's request for each variant and asserts every one reached the origin.
func (f *credentialFixture) warm(sessions []string, variants ...string) {
	f.t.Helper()

	if len(variants) == 0 {
		variants = []string{""}
	}
	for _, session := range sessions {
		for _, variant := range variants {
			got := f.get(session, variant, "")
			require.Equal(f.t, fiber.StatusOK, got.status)
			require.Equal(f.t, accountBody(session, variant), got.body)
			require.Equal(f.t, cacheMiss, got.cache)
		}
	}
}

// requireRefused asserts that session "a", which the app has ended, was refused with status at the origin.
func (f *credentialFixture) requireRefused(status int, variant, cacheControl string) reply {
	f.t.Helper()

	return f.requireRefusedAs(status, "a", variant, cacheControl)
}

func (f *credentialFixture) requireRefusedAs(status int, session, variant, cacheControl string) reply {
	f.t.Helper()

	calls := f.calls.Load()
	got := f.get(session, variant, cacheControl)
	require.Equal(f.t, status, got.status, "a refused credential was answered from the cache")
	require.NotContains(f.t, got.body, "ACCOUNT")
	require.Equal(f.t, calls+1, f.calls.Load(), "a refused credential must reach the origin")
	return got
}

func accountBody(session, variant string) string {
	return "ACCOUNT: data of " + session + "/" + variant
}

// refusal answers with status, naming directive on the response: "bare" for
// none, "VaryStar" for Vary: *, anything else as a Cache-Control.
func refusal(status int, directive string) func(fiber.Ctx) error {
	return func(c fiber.Ctx) error {
		switch directive {
		case "bare":
		case "VaryStar":
			c.Set(fiber.HeaderVary, "*")
		default:
			c.Set(fiber.HeaderCacheControl, directive)
		}
		return c.SendStatus(status)
	}
}

// refusingAfter refuses with a 401 once strip has removed the credential, like a handler that clears a rejected session.
func refusingAfter(strip func(*fasthttp.RequestHeader)) func(fiber.Ctx) error {
	return func(c fiber.Ctx) error {
		strip(&c.Request().Header)
		return c.SendStatus(fiber.StatusUnauthorized)
	}
}

// A reload answered 401, 403 or 407 ends its own credential's entry only, whatever directive it carries.
func Test_Cache_RejectedCredentialDropsEntry(t *testing.T) {
	t.Parallel()

	for _, partition := range []string{"keyCookies", "varyCookie", "varyApiKey", "authorization"} {
		for _, storage := range storageKinds {
			for _, reload := range reloadForms {
				for _, status := range refusalStatuses {
					for _, directive := range []string{"bare", "private", "no-store", "no-cache", "VaryStar"} {
						t.Run(fmt.Sprintf("%s/%s/%s/%d/%s", partition, storage, reload, status, directive), func(t *testing.T) {
							t.Parallel()

							f := newCredentialFixture(t, &credentialOptions{
								partition: partition,
								storage:   storage,
								refuse:    refusal(status, directive),
							})

							f.warm([]string{"a"})
							entrySize := f.probe.counted()
							f.warm([]string{"b"})
							f.requireHit("a", "")
							require.Equal(t, 2*entrySize, f.probe.counted())
							require.Equal(t, 2, f.probe.nodes())

							// A refusal that found no entry of its own purges nothing.
							for _, cacheControl := range []string{"", reload} {
								got := f.get("stranger", "", cacheControl)
								require.Equal(t, status, got.status)
								require.Equal(t, cacheUnreachable, got.cache)
							}
							f.requireHit("b", "")
							f.requireHit("a", "")
							require.Equal(t, 2*entrySize, f.probe.counted())
							require.Equal(t, 2, f.probe.nodes())

							f.ended.Store("a", true)
							got := f.get("a", "", reload)
							require.Equal(t, status, got.status)
							require.Equal(t, cacheUnreachable, got.cache)

							got = f.requireRefused(status, "", "")
							require.Equal(t, cacheUnreachable, got.cache)

							// The purge gave back exactly the refused entry's accounting.
							require.Equal(t, entrySize, f.probe.counted())
							require.Equal(t, 1, f.probe.nodes())

							// Only the refused credential's entry went, whatever directive the refusal carried.
							f.requireHit("b", "")
						})
					}
				}
			}
		}
	}
}

// A refusal returned as fiber.ErrUnauthorized ends the entry; an error that says nothing about the credential does not.
func Test_Cache_RejectedCredentialReturnedAsError(t *testing.T) {
	t.Parallel()

	answers := []struct {
		err    error
		name   string
		status int
		drops  bool
	}{
		{name: "401", err: fiber.ErrUnauthorized, status: fiber.StatusUnauthorized, drops: true},
		{name: "403", err: fiber.ErrForbidden, status: fiber.StatusForbidden, drops: true},
		{name: "407", err: fiber.ErrProxyAuthRequired, status: fiber.StatusProxyAuthRequired, drops: true},
		{name: "wrapped 401", err: fmt.Errorf("session expired: %w", fiber.ErrUnauthorized), status: fiber.StatusUnauthorized, drops: true},
		{name: "503", err: fiber.ErrServiceUnavailable, status: fiber.StatusServiceUnavailable},
		{name: "429", err: fiber.ErrTooManyRequests, status: fiber.StatusTooManyRequests},
		{name: "plain error", err: errors.New("session store unreachable"), status: fiber.StatusInternalServerError},
	}

	for _, partition := range []string{"keyCookies", "varyCookie", "authorization"} {
		for _, storage := range storageKinds {
			for _, reload := range reloadForms {
				for _, answer := range answers {
					t.Run(fmt.Sprintf("%s/%s/%s/%s", partition, storage, reload, answer.name), func(t *testing.T) {
						t.Parallel()

						f := newCredentialFixture(t, &credentialOptions{
							partition: partition,
							storage:   storage,
							refuse:    func(fiber.Ctx) error { return answer.err },
						})
						f.warm([]string{"a", "b"})

						f.ended.Store("a", true)
						require.Equal(t, answer.status, f.get("a", "", reload).status)

						if answer.drops {
							f.requireRefused(answer.status, "", "")
						} else {
							calls := f.calls.Load()
							f.requireHit("a", "")
							require.Equal(t, calls, f.calls.Load())
						}

						f.requireHit("b", "")
					})
				}
			}
		}
	}
}

// A refusal OnTimeout returns ends the entry, but the response stays the timed-out handler's.
func Test_Cache_RejectedCredentialFromOnTimeout(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	answered := make(chan struct{})
	finished := make(chan struct{})
	app := fiber.New()
	app.Use(New(Config{KeyCookies: []string{"session"}}))
	app.Get("/", timeout.New(func(c fiber.Ctx) error {
		if calls.Add(1) != 2 {
			return c.SendString("ok")
		}
		defer close(finished)
		// Nothing orders this write against the cache's after c.Next().
		<-answered
		c.Set("X-Handler", "1")
		return nil
	}, timeout.Config{
		Timeout: 100 * time.Millisecond,
		OnTimeout: func(fiber.Ctx) error {
			close(answered)
			return fiber.ErrUnauthorized
		},
	}))

	get := func(cacheControl string) *fasthttp.RequestCtx {
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.SetRequestURI("/")
		fctx.Request.Header.SetCookie("session", "a")
		if cacheControl != "" {
			fctx.Request.Header.Set(fiber.HeaderCacheControl, cacheControl)
		}
		app.Handler()(fctx)
		return fctx
	}

	require.Equal(t, cacheMiss, string(get("").Response.Header.Peek("X-Cache")))

	fctx := get("no-cache")
	resp := fctx.LastTimeoutErrorResponse()
	require.NotNil(t, resp)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode())
	<-finished
	require.Equal(t, "1", string(fctx.Response.Header.Peek("X-Handler")))
	require.Empty(t, fctx.Response.Header.Peek("X-Cache"), "the cache wrote to the timed-out handler's response")

	require.Equal(t, cacheMiss, string(get("").Response.Header.Peek("X-Cache")))
	require.Equal(t, int32(3), calls.Load())
}

// A refusal ends the credential's entry and its other variants, through the manifest when that is its own, else a record.
func Test_Cache_RejectedCredentialDropsItsVariants(t *testing.T) {
	t.Parallel()

	noStore := refusal(fiber.StatusUnauthorized, "no-store")
	private := refusal(fiber.StatusUnauthorized, "private")
	// Returned as an error, with the directive set before: the headers stand, the status is the error's.
	noStoreError := func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return fiber.ErrUnauthorized
	}
	scenarios := []struct {
		refuse      func(fiber.Ctx) error // the refusal, a bare 401 when nil
		name        string
		partition   string
		anonymous   bool
		kept        bool // the refused entry stays: the refusal is no verdict on a credential
		siblingKept bool // the credential's other variant stays
	}{
		{name: "KeyCookies", partition: "keyCookies"},
		{name: "Authorization", partition: "authorization"},
		// The shared manifest stays; a record revalidates the session's other variant.
		{name: "Vary Cookie", partition: "varyCookie"},
		{name: "Vary X-API-Key", partition: "varyApiKey"},
		{name: "API key header", partition: "apiKey"},
		{name: "header declared in KeyCredentialHeaders", partition: "credentialHeader"},
		// A custom key is opaque, so the declared header is taken to be in it.
		{name: "custom KeyGenerator, declared header", partition: "customKey"},
		// Nothing marks these a credential, so a bare refusal leaves the shared entries alone.
		{name: "tenant header, not a credential", partition: "tenant", kept: true, siblingKept: true},
		{name: "undeclared header, bare refusal", partition: "customHeader", kept: true, siblingKept: true},
		{name: "undeclared Vary, bare refusal", partition: "customVary", kept: true, siblingKept: true},
		{name: "no credential in the key", partition: "keyCookies", anonymous: true, kept: true, siblingKept: true},
		// Its directives are about the refusal: nothing that is no credential's goes with it.
		{name: "undeclared header, private refusal", partition: "customHeader", refuse: private, kept: true, siblingKept: true},
		{name: "undeclared Vary, private refusal", partition: "customVary", refuse: private, kept: true, siblingKept: true},
		{name: "no credential in the key, no-store refusal", partition: "keyCookies", anonymous: true, refuse: noStore, kept: true, siblingKept: true},
		// A manifest the sessions share is theirs as well, so it stays whatever the refusal says about keeping.
		{name: "Vary Cookie, no-store refusal", partition: "varyCookie", refuse: noStore},
		{name: "Vary Cookie, no-store refusal returned as an error", partition: "varyCookie", refuse: noStoreError},
		{name: "Vary X-API-Key, private refusal", partition: "varyApiKey", refuse: private},
		// The key is decided before the handler runs, so clearing the credential cannot change it.
		{
			name: "KeyCookies, cookie cleared by the handler", partition: "keyCookies",
			refuse: refusingAfter(func(h *fasthttp.RequestHeader) { h.DelAllCookies() }),
		},
		{
			name: "Authorization, header cleared by the handler", partition: "authorization",
			refuse: refusingAfter(func(h *fasthttp.RequestHeader) { h.Del(fiber.HeaderAuthorization) }),
		},
	}

	for _, storage := range storageKinds {
		for _, scenario := range scenarios {
			t.Run(storage+"/"+scenario.name, func(t *testing.T) {
				t.Parallel()

				refused, other := "a", "b"
				var valid []string
				if scenario.anonymous {
					// No cookie is sent, so the key is shared by every client without one.
					refused, valid = "", []string{"", "b"}
				}
				f := newCredentialFixture(t, &credentialOptions{
					partition: scenario.partition,
					storage:   storage,
					vary:      "X-Variant",
					valid:     valid,
					refuse:    scenario.refuse,
				})
				f.warm([]string{refused, other}, "one", "two")

				f.ended.Store(refused, true)
				got := f.get(refused, "one", "no-cache")
				require.Equal(t, fiber.StatusUnauthorized, got.status)
				require.Equal(t, cacheUnreachable, got.cache)

				got = f.get(refused, "one", "")
				if scenario.kept {
					require.Equal(t, cacheHit, got.cache, "a refusal that is no verdict on a credential ended the shared entry")
				} else {
					require.Equal(t, fiber.StatusUnauthorized, got.status, "the refused variant was answered from the cache")
				}

				got = f.get(refused, "two", "")
				if scenario.siblingKept {
					require.Equal(t, cacheHit, got.cache)
				} else {
					require.Equal(t, fiber.StatusUnauthorized, got.status, "a variant of the refused credential was answered from the cache")
				}

				for _, variant := range []string{"one", "two"} {
					f.requireHit(other, variant)
				}
			})
		}
	}
}

// The documented recipe: a CacheInvalidator reporting ended sessions cuts one off at once, even with max-stale.
func Test_Cache_InvalidatorCutsOffEndedSession(t *testing.T) {
	t.Parallel()

	for _, partition := range []string{"keyCookies", "varyCookie"} {
		for _, storage := range storageKinds {
			for _, request := range []struct{ name, cacheControl string }{{"plain", ""}, {"max-stale", "max-stale"}} {
				t.Run(partition+"/"+storage+"/"+request.name, func(t *testing.T) {
					t.Parallel()

					f := newCredentialFixture(t, &credentialOptions{
						partition:       partition,
						storage:         storage,
						vary:            "X-Variant",
						invalidateEnded: true,
					})
					variants := []string{"one", "two"}
					f.warm([]string{"a", "b"}, variants...)
					for _, variant := range variants {
						f.requireHit("a", variant)
					}

					f.ended.Store("a", true)
					for _, variant := range variants {
						f.requireRefused(fiber.StatusUnauthorized, variant, request.cacheControl)
					}

					for _, variant := range variants {
						f.requireHit("b", variant)
					}
				})
			}
		}
	}
}

// isMarker tells the markers announcing refusal records from the records, which are keyed under them.
func isMarker(key string) bool {
	return key == refusalMarker || slices.Contains(groupMarkers[:], key)
}

func isRefusalRecord(key string) bool {
	return !isMarker(key) && strings.HasPrefix(key, refusalMarker+"|")
}

// refusalStorage counts refusal record reads and writes and marker reads, and can fail record reads
// and writes.
type refusalStorage struct {
	*failingCacheStorage
	reads       atomic.Int32
	writes      atomic.Int32
	markerReads atomic.Int32 // of the marker for all URLs
	groupReads  atomic.Int32 // of the markers of the groups of URLs
	fail        atomic.Bool
	failWrites  atomic.Bool
}

func newRefusalStorage() *refusalStorage {
	return &refusalStorage{failingCacheStorage: newFailingCacheStorage()}
}

func (s *refusalStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	if isRefusalRecord(key) {
		s.writes.Add(1)
		if s.failWrites.Load() {
			return errors.New("storage unavailable")
		}
	}
	return s.failingCacheStorage.SetWithContext(ctx, key, val, exp)
}

func (s *refusalStorage) records() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for key := range s.data {
		if isRefusalRecord(key) {
			n++
		}
	}
	return n
}

func (s *refusalStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if key == refusalMarker {
		s.markerReads.Add(1)
	} else if isMarker(key) {
		s.groupReads.Add(1)
	}
	// A read with no key is a record read for a request that has no credential to record.
	if isRefusalRecord(key) || key == "" {
		s.reads.Add(1)
		if s.fail.Load() {
			return nil, errors.New("storage unavailable")
		}
	}
	return s.failingCacheStorage.GetWithContext(ctx, key)
}

func Test_Cache_RefusedCredentialStaysOutWhileOthersMiss(t *testing.T) {
	t.Parallel()

	for _, partition := range []string{"varyCookie", "varyApiKey"} {
		for _, storage := range storageKinds {
			for _, directive := range []string{"bare"} {
				t.Run(partition+"/"+storage+"/"+directive, func(t *testing.T) {
					t.Parallel()

					f := newCredentialFixture(t, &credentialOptions{
						partition: partition,
						storage:   storage,
						vary:      "X-Variant",
						refuse:    refusal(fiber.StatusUnauthorized, directive),
					})
					f.warm([]string{"a", "b"}, "one", "two")

					f.ended.Store("a", true)
					require.Equal(t, fiber.StatusUnauthorized, f.get("a", "one", "no-cache").status)

					require.Equal(t, cacheMiss, f.get("b", "three", "").cache)

					f.requireRefused(fiber.StatusUnauthorized, "two", "")

					for _, variant := range []string{"one", "two", "three"} {
						f.requireHit("b", variant)
					}
				})
			}
		}
	}
}

// A hit looks for a record only once one is announced for its URL group, and only if a credential selected the entry.
func Test_Cache_RefusalRecordIsReadOnlyWhereOneCanApply(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		partition string
		vary      string
		reads     int32 // per hit, once a record is announced
		anonymous bool
		polls     bool // the key or the Vary could hold a credential, so the markers are read
	}{
		{name: "no Vary", partition: "tenant"},
		{name: "Vary: X-Variant", partition: "tenant", vary: "X-Variant"},
		{name: "Vary: Cookie", partition: "varyCookie", reads: 1, polls: true},
		{name: "Vary: Cookie, X-Variant", partition: "varyCookie", vary: "X-Variant", reads: 1, polls: true},
		{name: "Vary: X-API-Key", partition: "varyApiKey", reads: 1, polls: true},
		{name: "Vary: Cookie, no cookie sent", partition: "varyCookie", anonymous: true, polls: true},
		{name: "Vary: X-API-Key, no key sent", partition: "varyApiKey", anonymous: true, polls: true},
		{name: "the key holds the cookie", partition: "keyCookies", reads: 1, polls: true},
		{name: "the key holds the credential header", partition: "apiKey", reads: 1, polls: true},
		{name: "the key holds Authorization", partition: "authorization", reads: 1, polls: true},
	} {
		for _, announced := range []string{"nowhere", "another group", "the group"} {
			t.Run(fmt.Sprintf("%s/announced=%s", tc.name, announced), func(t *testing.T) {
				t.Parallel()

				session := "a"
				if tc.anonymous {
					session = ""
				}
				storage := newRefusalStorage()
				if announced != "nowhere" {
					group := groupOf(fixturePath)
					if announced == "another group" {
						group = (group + 1) % refusalGroups
					}
					storage.data[refusalMarker] = []byte{1}
					storage.data[groupMarkers[group]] = []byte{1}
				}
				f := newCredentialFixture(t, &credentialOptions{
					partition: tc.partition,
					custom:    storage,
					vary:      tc.vary,
					valid:     []string{session},
				})
				require.Equal(t, cacheMiss, f.get(session, "", "").cache)
				require.Zero(t, storage.reads.Load(), "a miss has nothing to check")

				for range 3 {
					require.Equal(t, cacheHit, f.get(session, "", "").cache)
				}
				want := int32(0)
				if announced == "the group" {
					want = 3 * tc.reads
				}
				require.Equal(t, want, storage.reads.Load())
				// A marker is read once per poll, and not at all for a hit that can hold no credential.
				if !tc.polls {
					require.Zero(t, storage.markerReads.Load(), "a hit that can hold no credential read a marker")
				}
				require.LessOrEqual(t, storage.markerReads.Load(), int32(2))
				require.LessOrEqual(t, storage.groupReads.Load(), int32(2))
			})
		}
	}
}

func Test_Cache_UnreadableRefusalRecordRevalidates(t *testing.T) {
	t.Parallel()

	storage := newRefusalStorage()
	storage.data[refusalMarker] = []byte{1}
	storage.data[groupMarkers[groupOf(fixturePath)]] = []byte{1}
	f := newCredentialFixture(t, &credentialOptions{
		partition: "varyCookie",
		custom:    storage,
		valid:     []string{"a"},
	})
	require.Equal(t, cacheMiss, f.get("a", "", "").cache)
	require.Equal(t, cacheHit, f.get("a", "", "").cache)

	storage.fail.Store(true)
	calls := f.calls.Load()
	got := f.get("a", "", "")
	require.Equal(t, cacheMiss, got.cache)
	require.Equal(t, calls+1, f.calls.Load(), "the entry must reach the origin")
}

// A refused client that sends no credential leaves the shared entry alone, and no record is made or read.
func Test_Cache_RefusedAnonymousClientLeavesTheSharedEntry(t *testing.T) {
	t.Parallel()

	for _, partition := range []string{"varyCookie", "varyApiKey", "tenant", "customHeader", "customVary"} {
		for _, storage := range storageKinds {
			for _, reload := range reloadForms {
				for _, status := range refusalStatuses {
					for _, directive := range []string{"bare", "no-store"} {
						t.Run(fmt.Sprintf("%s/%s/%s/%d/%s", partition, storage, reload, status, directive), func(t *testing.T) {
							t.Parallel()

							backing := newRefusalStorage()
							opts := &credentialOptions{partition: partition, valid: []string{"", "a"}, refuse: refusal(status, directive)}
							if storage == "external" {
								opts.custom = backing
							}
							f := newCredentialFixture(t, opts)
							f.warm([]string{"", "a"})
							f.requireHit("", "")

							f.requireRefusedAs(status, "stranger", "", reload)
							f.requireHit("", "")

							f.ended.Store("", true)
							f.requireRefusedAs(status, "", "", reload)
							require.Zero(t, backing.writes.Load(), "an anonymous client has no credential to record a refusal of")
							require.Zero(t, backing.reads.Load(), "an anonymous client has no record to be checked against")

							f.requireHit("", "")
							f.requireHit("a", "")
						})
					}
				}
			}
		}
	}
}

func Test_Cache_RefusedCredentialRecoversWhenAccepted(t *testing.T) {
	t.Parallel()

	for _, partition := range []string{"varyCookie", "varyApiKey"} {
		t.Run(partition, func(t *testing.T) {
			t.Parallel()

			storage := newRefusalStorage()
			f := newCredentialFixture(t, &credentialOptions{
				partition: partition,
				custom:    storage,
				vary:      "X-Variant",
			})
			f.warm([]string{"a", "b"}, "one", "two")

			f.ended.Store("a", true)
			f.requireRefused(fiber.StatusUnauthorized, "one", "no-cache")
			require.Equal(t, 1, storage.records(), "the refusal is recorded")

			// Granted again: the other variant, still stored, goes to the origin once, which ends the record.
			f.ended.Delete("a")
			calls := f.calls.Load()
			require.Equal(t, cacheMiss, f.get("a", "two", "").cache)
			require.Equal(t, calls+1, f.calls.Load(), "the record must send the entry to the origin")
			require.Zero(t, storage.records(), "the record of a refusal that no longer stands goes")

			f.requireHit("a", "two")
			require.Equal(t, cacheMiss, f.get("a", "one", "").cache, "the refused entry was dropped")
			f.requireHit("a", "one")
			f.requireHit("b", "one")
		})
	}
}

// A redirect answering a refused credential, as a login page's does, is no acceptance: the record stays.
func Test_Cache_RedirectDoesNotForgetTheRefusal(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		partition     string
		vary          string
		variantHeader string
	}{
		// Variants on a manifest the sessions share, which the record reaches.
		{name: "Vary Cookie", partition: "varyCookie", vary: "X-Variant"},
		// Variants the key tells apart by Accept, each with a manifest of its own, which the record reaches too.
		{name: "KeyCookies", partition: "keyCookies", variantHeader: fiber.HeaderAccept},
	} {
		for _, storage := range storageKinds {
			t.Run(tc.name+"/"+storage, func(t *testing.T) {
				t.Parallel()

				var redirect atomic.Bool
				f := newCredentialFixture(t, &credentialOptions{
					partition:     tc.partition,
					storage:       storage,
					vary:          tc.vary,
					variantHeader: tc.variantHeader,
					refuse: func(c fiber.Ctx) error {
						if redirect.Load() {
							c.Set(fiber.HeaderLocation, "/login")
							return c.SendStatus(fiber.StatusFound)
						}
						return c.SendStatus(fiber.StatusUnauthorized)
					},
				})
				variants := []string{"one", "two", "three"}
				f.warm([]string{"a", "b"}, variants...)

				f.ended.Store("a", true)
				f.requireRefused(fiber.StatusUnauthorized, "one", "no-cache")

				// Sent to the login page from then on: the refusal stands, so the record does.
				redirect.Store(true)
				f.requireRefused(fiber.StatusFound, "two", "")
				f.requireRefused(fiber.StatusFound, "three", "")
				for _, variant := range variants {
					f.requireHit("b", variant)
				}
			})
		}
	}
}

func Test_Cache_RefusalOutlivesAConcurrentReplacement(t *testing.T) {
	t.Parallel()

	for _, partition := range []string{"keyCookies", "authorization", "varyCookie", "varyApiKey"} {
		for _, storage := range storageKinds {
			t.Run(partition+"/"+storage, func(t *testing.T) {
				t.Parallel()

				var armed atomic.Bool
				f := newCredentialFixture(t, &credentialOptions{
					partition: partition,
					storage:   storage,
					vary:      "X-Variant",
					beforeRefusal: func(f *credentialFixture) {
						if !armed.CompareAndSwap(true, false) {
							return
						}
						// Another request of the session, answered before it ended, stores its response meanwhile.
						f.ended.Delete("a")
						require.Equal(f.t, cacheMiss, f.get("a", "one", "max-age=0").cache)
						f.ended.Store("a", true)
					},
				})
				f.warm([]string{"a"}, "one")
				entrySize := f.probe.counted()
				f.warm([]string{"a"}, "two")
				f.warm([]string{"b"}, "one", "two")

				f.ended.Store("a", true)
				armed.Store(true)
				require.Equal(t, fiber.StatusUnauthorized, f.get("a", "one", "no-cache").status)
				require.False(t, armed.Load(), "the replacement was never stored")

				f.requireRefused(fiber.StatusUnauthorized, "one", "")
				f.requireRefused(fiber.StatusUnauthorized, "two", "")
				f.requireHit("b", "one")

				// The replacement is not left behind. The other session's two entries remain, and the refused one's other
				// variant when its manifest went and no request found it.
				want := 2
				if partition == "keyCookies" || partition == "authorization" {
					want = 3
				}
				require.Equal(t, uint(want)*entrySize, f.probe.counted())
				require.Equal(t, want, f.probe.nodes())
			})
		}
	}
}

// Only a credential that has an entry gets a record, so records cannot outnumber the entries.
func Test_Cache_RefusalsOfUnknownCredentialsWriteNoRecords(t *testing.T) {
	t.Parallel()

	storage := newRefusalStorage()
	f := newCredentialFixture(t, &credentialOptions{
		partition: "varyCookie",
		custom:    storage,
		valid:     []string{"a"},
	})
	require.Equal(t, cacheMiss, f.get("a", "", "").cache)

	for i := range 50 {
		got := f.get(fmt.Sprintf("stranger-%d", i), "", "no-cache")
		require.Equal(t, fiber.StatusUnauthorized, got.status)
	}
	require.Zero(t, storage.writes.Load(), "a credential with no entry got a record")
	require.Zero(t, storage.records())

	// The one credential that has an entry gets one, refused again and again.
	f.ended.Store("a", true)
	for range 3 {
		require.Equal(t, fiber.StatusUnauthorized, f.get("a", "", "no-cache").status)
	}
	require.Equal(t, int32(1), storage.writes.Load())
	require.Equal(t, 1, storage.records())
}

func Test_Cache_HandlerErrorKeepsTheEntry(t *testing.T) {
	t.Parallel()

	for _, storage := range storageKinds {
		t.Run(storage, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Storage: storageOf(storage)}))
			app.Get("/", func(c fiber.Ctx) error {
				if c.Get("X-Fail") == "1" {
					return fiber.ErrServiceUnavailable
				}
				return c.SendString("PUBLIC")
			})

			require.Equal(t, cacheMiss, getRoot(t, app, nil).cache)
			for _, reload := range []string{"no-cache", "max-age=0"} {
				got := getRoot(t, app, map[string]string{"X-Fail": "1", fiber.HeaderCacheControl: reload})
				require.Equal(t, fiber.StatusServiceUnavailable, got.status)
			}
			got := getRoot(t, app, nil)
			require.Equal(t, cacheHit, got.cache)
			require.Equal(t, "PUBLIC", got.body)
		})
	}
}

// An entry holding a credential's response is not served stale, unless AllowStaleCredentialResponses says it may be.
func Test_Cache_CredentialKeyedEntryIgnoresMaxStale(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		name      string
		partition string
		stale     bool // the expired entry is still served: nothing ties it to a credential
	}{
		{name: "KeyCookies", partition: "keyCookies"},
		{name: "Vary Cookie", partition: "varyCookie"},
		{name: "Vary X-API-Key", partition: "varyApiKey"},
		{name: "Authorization", partition: "authorization"},
		{name: "API key header named in KeyHeaders", partition: "apiKey"},
		{name: "header declared in KeyCredentialHeaders", partition: "credentialHeader"},
		{name: "tenant header, not a credential", partition: "tenant", stale: true},
	}

	for _, storage := range storageKinds {
		for _, maxStale := range []string{"max-stale", "max-stale=3600"} {
			for _, allowed := range []bool{false, true} {
				for _, scenario := range scenarios {
					t.Run(fmt.Sprintf("%s/%s/allowed=%t/%s", storage, maxStale, allowed, scenario.name), func(t *testing.T) {
						t.Parallel()

						clock := newTestClock(time.Unix(1_700_000_000, 0))
						f := newCredentialFixture(t, &credentialOptions{
							partition:  scenario.partition,
							storage:    storage,
							clock:      clock,
							expiration: time.Minute,
							allowStale: allowed,
						})
						f.warm([]string{"a", "b"})

						// Past the entry's lifetime, well inside what the storage keeps.
						clock.Add(2 * time.Minute)
						f.ended.Store("a", true)
						if scenario.stale || allowed {
							calls := f.calls.Load()
							got := f.get("a", "", maxStale)
							require.Equal(t, fiber.StatusOK, got.status)
							require.Equal(t, accountBody("a", ""), got.body)
							require.Equal(t, cacheHit, got.cache)
							require.Contains(t, got.header.Get(fiber.HeaderWarning), "110")
							require.Equal(t, calls, f.calls.Load())
							return
						}
						f.requireRefused(fiber.StatusUnauthorized, "", maxStale)
					})
				}
			}
		}
	}
}

func Test_Cache_AllowStaleCredentialResponsesKeepsInvalidatedEntriesOut(t *testing.T) {
	t.Parallel()

	for _, partition := range []string{"keyCookies", "varyCookie", "authorization"} {
		for _, storage := range storageKinds {
			t.Run(partition+"/"+storage, func(t *testing.T) {
				t.Parallel()

				clock := newTestClock(time.Unix(1_700_000_000, 0))
				f := newCredentialFixture(t, &credentialOptions{
					partition:       partition,
					storage:         storage,
					clock:           clock,
					expiration:      time.Minute,
					allowStale:      true,
					invalidateEnded: true,
				})
				f.warm([]string{"a", "b"})
				clock.Add(2 * time.Minute)

				// The session that has not ended is served stale; the one that has is not.
				got := f.get("b", "", "max-stale")
				require.Equal(t, cacheHit, got.cache)
				f.ended.Store("a", true)
				f.requireRefused(fiber.StatusUnauthorized, "", "max-stale")
			})
		}
	}
}

// A refusal ends the shared entry only as a verdict on a credential: an IP or geo rule must not end what others share.
func Test_Cache_RefusalEndsTheSharedEntryOnlyForACredential(t *testing.T) {
	t.Parallel()

	custom := Config{KeyGenerator: pathKey}
	tests := []struct {
		blocked   map[string]string // what the client the app refuses sends besides X-Blocked
		name      string
		directive string // the Cache-Control the refusal carries, if any
		cfg       Config
		kept      bool
	}{
		{name: "default key", kept: true},
		{name: "default key, a cookie the key does not hold", blocked: map[string]string{"Cookie": "sid=1"}, kept: true},
		{name: "default key, a credential header the key does not hold", blocked: map[string]string{"X-API-Key": "k"}, kept: true},
		{name: "KeyHeaders naming a tenant", cfg: Config{KeyHeaders: []string{"X-Tenant"}}, kept: true},
		{name: "KeyCookies the request does not carry", cfg: Config{KeyCookies: []string{"session"}}, kept: true},
		{name: "custom KeyGenerator, client with an undeclared cookie", cfg: custom, blocked: map[string]string{"Cookie": "sid=1"}, kept: true},
		{name: "custom KeyGenerator, anonymous client", cfg: custom, kept: true},
		{
			name: "custom KeyGenerator, client with a header that is no credential",
			cfg:  custom, blocked: map[string]string{"X-Tenant": "acme"}, kept: true,
		},
		{
			name:    "custom KeyGenerator, client with a declared cookie",
			cfg:     Config{KeyGenerator: pathKey, KeyCredentialHeaders: []string{"Cookie"}},
			blocked: map[string]string{"Cookie": "sid=1"},
		},
		{
			name:    "custom KeyGenerator, client with a declared header",
			cfg:     Config{KeyGenerator: pathKey, KeyCredentialHeaders: []string{"X-Company-Token"}},
			blocked: map[string]string{"X-Company-Token": "t"},
		},
		{name: "custom KeyGenerator, client with a built-in credential header it does not declare", cfg: custom, blocked: map[string]string{"X-API-Key": "junk"}, kept: true},
		{
			name:    "custom KeyGenerator, client with a built-in credential header it declares",
			cfg:     Config{KeyGenerator: pathKey, KeyCredentialHeaders: []string{"X-API-Key"}},
			blocked: map[string]string{"X-API-Key": "k"},
		},
		// What the refusal says about keeping its own response is no verdict on the entry others share.
		{name: "default key, anonymous client, refusal says no-store", directive: "no-store", kept: true},
		{name: "default key, anonymous client, refusal says private", directive: "private", kept: true},
		{name: "default key, anonymous client, refusal says no-cache", directive: "no-cache", kept: true},
		{name: "custom KeyGenerator, anonymous client, refusal says no-store", cfg: custom, directive: "no-store", kept: true},
		{
			name:      "custom KeyGenerator, client with a built-in credential header, refusal says no-store",
			cfg:       custom,
			blocked:   map[string]string{"X-API-Key": "junk"},
			directive: "no-store",
			kept:      true,
		},
	}

	for _, storage := range storageKinds {
		for _, delivery := range []string{"status", "error"} {
			for _, reload := range reloadForms {
				for _, status := range refusalStatuses {
					for _, tc := range tests {
						t.Run(fmt.Sprintf("%s/%s/%s/%d/%s", storage, delivery, reload, status, tc.name), func(t *testing.T) {
							t.Parallel()

							cfg := tc.cfg
							cfg.Storage = storageOf(storage)
							app := fiber.New()
							app.Use(New(cfg))
							app.Get("/", func(c fiber.Ctx) error {
								if c.Get("X-Blocked") != "1" {
									return c.SendString("PUBLIC")
								}
								if tc.directive != "" {
									c.Set(fiber.HeaderCacheControl, tc.directive)
								}
								if delivery == "error" {
									return fiber.NewError(status)
								}
								return c.SendStatus(status)
							})

							require.Equal(t, cacheMiss, getRoot(t, app, nil).cache)
							require.Equal(t, cacheHit, getRoot(t, app, nil).cache)

							headers := map[string]string{"X-Blocked": "1", fiber.HeaderCacheControl: reload}
							maps.Copy(headers, tc.blocked)
							require.Equal(t, status, getRoot(t, app, headers).status)

							want := cacheMiss
							if tc.kept {
								want = cacheHit
							}
							require.Equal(t, want, getRoot(t, app, nil).cache)
						})
					}
				}
			}
		}
	}
}

func Test_Cache_RefusalReachesTheVariantsTheKeyTellsApart(t *testing.T) {
	t.Parallel()

	for _, partition := range []string{"keyCookies", "credentialHeader", "authorization", "varyCookie", "varyApiKey"} {
		for _, storage := range storageKinds {
			t.Run(partition+"/"+storage, func(t *testing.T) {
				t.Parallel()

				f := newCredentialFixture(t, &credentialOptions{
					partition:     partition,
					storage:       storage,
					variantHeader: fiber.HeaderAccept,
				})
				variants := []string{"text/html", "application/json"}
				f.warm([]string{"a", "b"}, variants...)

				f.ended.Store("a", true)
				f.requireRefused(fiber.StatusUnauthorized, variants[0], "no-cache")

				f.requireRefused(fiber.StatusUnauthorized, variants[1], "")
				for _, variant := range variants {
					f.requireHit("b", variant)
				}
			})
		}
	}
}

// GET and HEAD answer for the same resource and credential, so a refusal answering either reaches both.
func Test_Cache_RefusalOfGetOrHeadReachesTheOther(t *testing.T) {
	t.Parallel()

	methods := []string{fiber.MethodGet, fiber.MethodHead}
	for _, partition := range []string{"keyCookies", "credentialHeader", "authorization", "varyCookie", "varyApiKey", "customKey", "customAuthorization"} {
		for _, storage := range storageKinds {
			for _, refusedBy := range methods {
				t.Run(partition+"/"+storage+"/"+refusedBy, func(t *testing.T) {
					t.Parallel()

					other := methods[0]
					if refusedBy == other {
						other = methods[1]
					}
					f := newCredentialFixture(t, &credentialOptions{partition: partition, storage: storage})
					for _, method := range methods {
						for _, session := range []string{"a", "b"} {
							require.Equal(t, cacheMiss, f.send(method, session, "", "").cache)
						}
					}

					f.ended.Store("a", true)
					require.Equal(t, fiber.StatusUnauthorized, f.send(refusedBy, "a", "", "no-cache").status)

					calls := f.calls.Load()
					require.Equal(t, fiber.StatusUnauthorized, f.send(other, "a", "", "").status, "answered from the cache")
					require.Equal(t, calls+1, f.calls.Load(), "a refused credential must reach the origin")

					for _, method := range methods {
						require.Equal(t, cacheHit, f.send(method, "b", "", "").cache)
					}
				})
			}
		}
	}
}

func Test_Cache_RefusalReachesTheVariantsWhateverEachVariesOn(t *testing.T) {
	t.Parallel()

	for _, storage := range storageKinds {
		for _, refused := range []string{"text/html", "application/json"} {
			t.Run(storage+"/"+refused, func(t *testing.T) {
				t.Parallel()

				var ended atomic.Bool
				app := fiber.New()
				app.Use(New(Config{Storage: storageOf(storage), KeyCookies: []string{"session"}}))
				app.Get("/", func(c fiber.Ctx) error {
					if ended.Load() {
						return c.SendStatus(fiber.StatusUnauthorized)
					}
					// Only the HTML varies on the cookie: it has a manifest, the JSON has none.
					if c.Get(fiber.HeaderAccept) == "text/html" {
						c.Set(fiber.HeaderVary, fiber.HeaderCookie)
					}
					return c.SendString("page as " + c.Get(fiber.HeaderAccept))
				})
				get := func(accept, cacheControl string) reply {
					req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
					req.Header.Set(fiber.HeaderCookie, "session=a")
					req.Header.Set(fiber.HeaderAccept, accept)
					if cacheControl != "" {
						req.Header.Set(fiber.HeaderCacheControl, cacheControl)
					}
					return doRequest(t, app, req)
				}
				other := "application/json"
				if refused == other {
					other = "text/html"
				}
				for _, accept := range []string{refused, other} {
					require.Equal(t, cacheMiss, get(accept, "").cache)
					require.Equal(t, cacheHit, get(accept, "").cache)
				}

				ended.Store(true)
				require.Equal(t, fiber.StatusUnauthorized, get(refused, "no-cache").status)
				require.Equal(t, fiber.StatusUnauthorized, get(other, "").status, "the other variant was served from the cache")
			})
		}
	}
}

func Test_Cache_RefusalIsForgottenWhenTheAnswerIsNotStored(t *testing.T) {
	t.Parallel()

	storage := newRefusalStorage()
	f := newCredentialFixture(t, &credentialOptions{
		partition: "varyCookie",
		custom:    storage,
		vary:      "X-Variant",
	})
	f.warm([]string{"a", "b"}, "one", "two")

	f.ended.Store("a", true)
	f.requireRefused(fiber.StatusUnauthorized, "one", "no-cache")
	require.Equal(t, 1, storage.records())

	f.ended.Delete("a")
	f.noStore.Store(true)
	require.Equal(t, cacheUnreachable, f.get("a", "two", "").cache)
	require.Zero(t, storage.records(), "the record of a refusal that no longer stands is still there")

	f.noStore.Store(false)
	require.Equal(t, cacheMiss, f.get("a", "two", "").cache)
	f.requireHit("a", "two")
}

func Test_Cache_ExpiredDeleteKeepsAConcurrentReplacement(t *testing.T) {
	t.Parallel()

	for _, storage := range storageKinds {
		t.Run(storage, func(t *testing.T) {
			t.Parallel()

			clock := newTestClock(time.Unix(1_700_000_000, 0))
			gate := make(chan struct{})
			entered := make(chan struct{}, 1)
			inHandler := make(chan struct{}, 1)
			release := make(chan struct{})

			app := fiber.New()
			app.Use(New(Config{
				Storage:    storageOf(storage),
				Expiration: time.Minute,
				clock:      clock.Now,
				CacheInvalidator: func(c fiber.Ctx) bool {
					if c.Get("X-Slow") == "1" {
						entered <- struct{}{}
						<-gate
					}
					return false
				},
			}))
			app.Get("/", func(c fiber.Ctx) error {
				if c.Get("X-Slow") == "1" {
					inHandler <- struct{}{}
					<-release
					return c.SendString("slow")
				}
				return c.SendString("fresh")
			})

			require.Equal(t, cacheMiss, getRoot(t, app, nil).cache)
			clock.Add(2 * time.Minute)

			// The slow request finds the expired entry and waits in the invalidator.
			type result struct {
				resp *http.Response
				err  error
			}
			done := make(chan result, 1)
			go func() {
				req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
				req.Header.Set("X-Slow", "1")
				// Held by gate and release, which the test controls, so no timeout of Test's own.
				resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
				done <- result{resp, err}
			}()
			<-entered

			// Another request replaces the entry it found expired, then the slow one resumes.
			require.Equal(t, cacheMiss, getRoot(t, app, nil).cache)
			close(gate)
			<-inHandler

			require.Equal(t, cacheHit, getRoot(t, app, nil).cache, "the delete of the expired entry took the replacement with it")
			close(release)
			res := <-done
			require.NoError(t, res.err)
			require.NoError(t, res.resp.Body.Close())
		})
	}
}

// Freshness is judged after CacheInvalidator returns: an entry a slow call ages past max-age is revalidated.
func Test_Cache_InvalidatorRunsBeforeFreshnessIsJudged(t *testing.T) {
	t.Parallel()

	for _, storage := range storageKinds {
		t.Run(storage, func(t *testing.T) {
			t.Parallel()

			clock := newTestClock(time.Unix(1_700_000_000, 0))
			var calls atomic.Int32
			app := fiber.New()
			app.Use(New(Config{
				Storage:    storageOf(storage),
				Expiration: time.Hour,
				clock:      clock.Now,
				CacheInvalidator: func(c fiber.Ctx) bool {
					if c.Get("X-Slow") == "1" {
						clock.Add(40 * time.Second)
					}
					return false
				},
			}))
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("response-" + strconv.Itoa(int(calls.Add(1))))
			})

			require.Equal(t, "response-1", getRoot(t, app, nil).body)
			clock.Add(30 * time.Second)

			got := getRoot(t, app, map[string]string{"X-Slow": "1", fiber.HeaderCacheControl: "max-age=60"})
			require.Equal(t, "response-2", got.body, "an entry the call aged past max-age was served")
		})
	}
}

// An in-memory cache keeps no more refusal records than its limit. A record dropped while live is remembered
// by its hash, so its credential is still revalidated; past what is remembered, the oldest is forgotten and
// its variants are served until they expire. No other session's hits are revalidated.
func Test_Cache_RefusalRecordOverflow(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		records   int // the records the cache keeps
		refused   int // the sessions the app ends and refuses
		forgotten int // the sessions refused first, whose record is forgotten
	}{
		{name: "dropped records are remembered", records: 2, refused: 4},
		{name: "past what is remembered the oldest dropped is forgotten", records: 1, refused: 2 + droppedPerRecord, forgotten: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sessions := make([]string, tc.refused)
			for i := range sessions {
				sessions[i] = "s" + strconv.Itoa(i)
			}
			f := newCredentialFixture(t, &credentialOptions{
				partition: "varyCookie",
				storage:   "memory",
				vary:      "X-Variant",
				valid:     append([]string{"b"}, sessions...),
				recordCap: tc.records,
			})
			f.warm(append([]string{"b"}, sessions...), "one", "two")

			for _, session := range sessions {
				f.ended.Store(session, true)
				f.requireRefusedAs(fiber.StatusUnauthorized, session, "one", "no-cache")
			}

			// A remembered record keeps its session's other variant out; a forgotten one does not, until it expires.
			for i, session := range sessions {
				if i < tc.forgotten {
					got := f.get(session, "two", "")
					require.Equal(t, cacheHit, got.cache)
					require.Equal(t, accountBody(session, "two"), got.body)
					continue
				}
				f.requireRefusedAs(fiber.StatusUnauthorized, session, "two", "")
			}

			// The others keep their hits throughout.
			for _, variant := range []string{"one", "two"} {
				f.requireHit("b", variant)
			}
		})
	}
}

func Test_Cache_RefusalRecordCostsOnlyItsGroupOfURLs(t *testing.T) {
	t.Parallel()

	require.NotEqual(t, groupOf("/refused"), groupOf("/elsewhere"))
	storage := newRefusalStorage()
	var ended atomic.Bool
	app := fiber.New()
	app.Use(New(Config{Storage: storage, KeyCookies: []string{"session"}}))
	app.Get("/refused", func(c fiber.Ctx) error {
		if ended.Load() {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.SendString("data")
	})
	app.Get("/elsewhere", func(c fiber.Ctx) error { return c.SendString("data") })
	get := func(uri, accept, cacheControl string) reply {
		t.Helper()
		req := httptest.NewRequest(fiber.MethodGet, uri, http.NoBody)
		req.Header.Set("Cookie", "session=abc")
		req.Header.Set(fiber.HeaderAccept, accept)
		if cacheControl != "" {
			req.Header.Set(fiber.HeaderCacheControl, cacheControl)
		}
		return doRequest(t, app, req)
	}

	for _, uri := range []string{"/refused", "/elsewhere"} {
		for _, accept := range []string{"text/html", "application/json"} {
			require.Equal(t, cacheMiss, get(uri, accept, "").cache, uri)
			require.Equal(t, cacheHit, get(uri, accept, "").cache, uri)
		}
	}
	require.Zero(t, storage.reads.Load())

	ended.Store(true)
	require.Equal(t, fiber.StatusUnauthorized, get("/refused", "text/html", "no-cache").status)
	require.Equal(t, int32(1), storage.writes.Load())
	require.Equal(t, int32(1), storage.reads.Load(), "the refusal reads what is there before it writes")
	storage.reads.Store(0)

	// The group of /refused looks for the record, the other URL does not.
	require.Equal(t, cacheHit, get("/elsewhere", "application/json", "").cache)
	require.Zero(t, storage.reads.Load(), "a hit on a URL outside the group read a record")
	require.Equal(t, fiber.StatusUnauthorized, get("/refused", "application/json", "").status)
	require.NotZero(t, storage.reads.Load())
}

// An instance sharing the Storage notices a record another wrote within markerPoll.
func Test_Cache_RefusalReachesAnotherInstanceSharingTheStorage(t *testing.T) {
	t.Parallel()

	storage := newFailingCacheStorage()
	var ended atomic.Bool
	instance := func() *fiber.App {
		app := fiber.New()
		app.Use(New(Config{Storage: storage, KeyCookies: []string{"session"}}))
		app.Get("/page", func(c fiber.Ctx) error {
			if ended.Load() {
				return c.SendStatus(fiber.StatusUnauthorized)
			}
			return c.SendString("data")
		})
		return app
	}
	first, second := instance(), instance()
	get := func(app *fiber.App, accept, cacheControl string) reply {
		t.Helper()
		req := httptest.NewRequest(fiber.MethodGet, "/page", http.NoBody)
		req.Header.Set("Cookie", "session=abc")
		req.Header.Set(fiber.HeaderAccept, accept)
		if cacheControl != "" {
			req.Header.Set(fiber.HeaderCacheControl, cacheControl)
		}
		return doRequest(t, app, req)
	}

	for _, accept := range []string{"text/html", "application/json"} {
		require.Equal(t, cacheMiss, get(second, accept, "").cache)
	}
	require.Equal(t, cacheHit, get(second, "application/json", "").cache, "the second instance finds no record")

	// The first instance is refused, and records it. The second knows nothing of it until it reads the markers.
	ended.Store(true)
	require.Equal(t, fiber.StatusUnauthorized, get(first, "text/html", "no-cache").status)
	require.Eventually(t, func() bool {
		return get(second, "application/json", "").status == fiber.StatusUnauthorized
	}, 10*time.Second, 5*time.Millisecond, "the second instance still serves the refused credential")
}

func Test_Cache_ExpiredDeleteChecksTheEntry(t *testing.T) {
	t.Parallel()

	for _, withInvalidator := range []bool{false, true} {
		t.Run(fmt.Sprintf("invalidator=%t", withInvalidator), func(t *testing.T) {
			t.Parallel()

			var reads atomic.Int32
			storage := &hookedStorage{failingCacheStorage: newFailingCacheStorage(), onGet: func(key string) {
				if strings.HasPrefix(key, cacheKeyVersion+"|GET|") && !strings.HasPrefix(key, bodyKeyPrefix) {
					reads.Add(1)
				}
			}}
			clock := newTestClock(time.Unix(1_700_000_000, 0))
			cfg := Config{Storage: storage, Expiration: time.Minute, clock: clock.Now, DisableVaryHeaders: true}
			if withInvalidator {
				cfg.CacheInvalidator = func(fiber.Ctx) bool { return false }
			}
			app := fiber.New()
			app.Use(New(cfg))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("body") })

			require.Equal(t, cacheMiss, getRoot(t, app, nil).cache)
			clock.Add(2 * time.Minute)

			reads.Store(0)
			require.Equal(t, cacheMiss, getRoot(t, app, nil).cache)
			require.Equal(t, int32(2), reads.Load(), "the lookup, and the check that the entry is the one found")
		})
	}
}

// An entry that cannot be served is deleted before the request goes on. A Storage that fails that delete,
// or the read that checks it, fails the request rather than leave the entry to be found again.
func Test_Cache_UnservableEntryThatCannotBeDeletedFailsTheRequest(t *testing.T) {
	t.Parallel()

	const key = cacheKeyVersion + "|GET|scheme=http|host=example.com|/|q=|h=accept:0|accept-encoding:0|accept-language:0"
	future := uint64(time.Now().Unix()) + 3600

	for _, tc := range []struct {
		entry       *item
		name        string
		want        string
		invalidator bool
	}{
		{name: "expired, held past its lifetime", entry: &item{exp: 1, ttl: 1}, want: "failed to delete expired key"},
		{name: "private", entry: &item{exp: future, private: true}, want: "failed to delete private response"},
		{name: "expired, and the read that checks it fails", entry: &item{exp: 1, ttl: 1}, invalidator: true, want: "failed to delete expired key"},
		{name: "private, and the read that checks it fails", entry: &item{exp: future, private: true}, invalidator: true, want: "failed to delete private response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			base := newFailingCacheStorage()
			raw, err := tc.entry.MarshalMsg(nil)
			require.NoError(t, err)
			base.data[key] = raw

			cfg := Config{Storage: base, Expiration: time.Hour}
			if tc.invalidator {
				// The entry is read to find it, and again to check that it is still the one found.
				var reads atomic.Int32
				cfg.Storage = &hookedStorage{failingCacheStorage: base, onGet: func(read string) {
					if read == key && reads.Add(1) == 2 {
						base.mu.Lock()
						base.errs["get|"+key] = errors.New("storage unavailable")
						base.mu.Unlock()
					}
				}}
				cfg.CacheInvalidator = func(fiber.Ctx) bool { return false }
			} else {
				base.errs["del|"+key] = errors.New("storage unavailable")
			}

			var captured error
			app := fiber.New(fiber.Config{ErrorHandler: func(c fiber.Ctx, err error) error {
				captured = err
				return c.SendStatus(fiber.StatusInternalServerError)
			}})
			app.Use(New(cfg))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

			require.Equal(t, fiber.StatusInternalServerError, getRoot(t, app, nil).status)
			require.ErrorContains(t, captured, tc.want)
		})
	}
}

// Dropping what a concurrent request stored under the refused key is best effort: a failing Storage is
// logged, and the credential stays refused through its record.
func Test_Cache_RefusalToleratesAStorageThatFailsWhileDroppingTheReplacement(t *testing.T) {
	t.Parallel()

	for _, failing := range []string{"read", "delete"} {
		t.Run(failing, func(t *testing.T) {
			t.Parallel()

			base := newFailingCacheStorage()
			failure := errors.New("storage unavailable")
			var armed atomic.Bool
			var watched atomic.Pointer[string]
			var reads atomic.Int32
			storage := &hookedStorage{failingCacheStorage: base, onGet: func(key string) {
				// The first read of the key after the replacement is stored finds it, and the next removes it.
				if w := watched.Load(); w != nil && key == *w && reads.Add(1) == 2 && failing == "read" {
					base.mu.Lock()
					base.errs["get|"+key] = failure
					base.mu.Unlock()
				}
			}}
			entryOf := func(session string) string {
				base.mu.RLock()
				defer base.mu.RUnlock()
				for key := range base.data {
					if strings.Contains(key, "session:sha256:"+authDigest(session)) && !strings.HasPrefix(key, bodyKeyPrefix) && !isRefusalRecord(key) {
						return key
					}
				}
				return ""
			}
			f := newCredentialFixture(t, &credentialOptions{
				partition: "keyCookies",
				custom:    storage,
				beforeRefusal: func(f *credentialFixture) {
					if !armed.CompareAndSwap(true, false) {
						return
					}
					// Another request of the session, answered before it ended, stores its response meanwhile.
					f.ended.Delete("a")
					require.Equal(f.t, cacheMiss, f.get("a", "", "max-age=0").cache)
					f.ended.Store("a", true)

					key := entryOf("a")
					require.NotEmpty(f.t, key)
					if failing == "delete" {
						base.mu.Lock()
						base.errs["del|"+key] = failure
						base.mu.Unlock()
					}
					reads.Store(0)
					watched.Store(&key)
				},
			})
			f.warm([]string{"a"})
			entrySize := f.probe.counted()
			f.warm([]string{"b"})

			f.ended.Store("a", true)
			armed.Store(true)
			require.Equal(t, fiber.StatusUnauthorized, f.get("a", "", "no-cache").status)
			require.False(t, armed.Load(), "the replacement was never stored")

			base.mu.Lock()
			_, held := base.data[*watched.Load()]
			clear(base.errs)
			base.mu.Unlock()
			require.True(t, held, "the replacement could not be removed")

			// Still stored, so still tracked and counted, beside the other session's entry.
			require.Equal(t, 2, f.probe.nodes())
			require.Equal(t, 2*entrySize, f.probe.counted())

			f.requireRefused(fiber.StatusUnauthorized, "", "")
			f.requireHit("b", "")
		})
	}
}

func Test_Cache_RefusalAfterAnEntryFoundExpiredOrInvalidatedReachesTheOtherVariants(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"expired", "invalidated"} {
		for _, partition := range []string{"varyCookie", "varyApiKey"} {
			for _, storage := range storageKinds {
				t.Run(kind+"/"+partition+"/"+storage, func(t *testing.T) {
					t.Parallel()

					clock := newTestClock(time.Unix(1_700_000_000, 0))
					f := newCredentialFixture(t, &credentialOptions{
						partition:       partition,
						storage:         storage,
						vary:            "X-Variant",
						clock:           clock,
						expiration:      time.Minute,
						invalidateEnded: kind == "invalidated",
						invalidateOnly:  "one",
					})
					f.warm([]string{"a", "b"}, "one")
					clock.Add(40 * time.Second)
					f.warm([]string{"a", "b"}, "two")
					if kind == "expired" {
						clock.Add(30 * time.Second) // "one" has lapsed, "two" has half a minute left
					}

					f.ended.Store("a", true)
					require.Equal(t, fiber.StatusUnauthorized, f.get("a", "one", "").status)

					f.requireRefused(fiber.StatusUnauthorized, "two", "")
					f.requireHit("b", "two")
				})
			}
		}
	}
}

// A record the Storage cannot take is remembered by this instance, so the credential's other variants
// are still revalidated here, while the other sessions keep their hits.
func Test_Cache_RefusalIsRememberedWhenTheRecordCannotBeWritten(t *testing.T) {
	t.Parallel()

	for _, partition := range []string{"varyCookie", "varyApiKey"} {
		t.Run(partition, func(t *testing.T) {
			t.Parallel()

			storage := newRefusalStorage()
			f := newCredentialFixture(t, &credentialOptions{partition: partition, custom: storage, vary: "X-Variant"})
			f.warm([]string{"a", "b"}, "one", "two")

			storage.failWrites.Store(true)
			f.ended.Store("a", true)
			require.Equal(t, fiber.StatusUnauthorized, f.get("a", "one", "no-cache").status)
			require.Zero(t, storage.records())

			f.requireRefused(fiber.StatusUnauthorized, "two", "")
			f.requireHit("b", "two")
		})
	}
}

// With a custom key the manifest is the credential's own, so the refusal drops it, and an answer that was
// already on its way can bring it back. The record keeps the credential's variants revalidated.
func Test_Cache_RefusalOfCustomKeyAuthorizationOutlivesTheManifestComingBack(t *testing.T) {
	t.Parallel()

	storage := newFailingCacheStorage()
	f := newCredentialFixture(t, &credentialOptions{partition: "customAuthorization", custom: storage, vary: "X-Variant"})
	f.warm([]string{"a", "b"}, "one", "two", "three")

	manifests := func() map[string][]byte {
		storage.mu.RLock()
		defer storage.mu.RUnlock()
		found := map[string][]byte{}
		for key, value := range storage.data {
			if strings.HasPrefix(key, manifestKeyPrefix) && strings.Contains(key, "|auth="+authDigest("Bearer a")) {
				found[key] = slices.Clone(value)
			}
		}
		return found
	}
	saved := manifests()
	require.Len(t, saved, 1)

	f.ended.Store("a", true)
	require.Equal(t, fiber.StatusUnauthorized, f.get("a", "one", "no-cache").status)
	require.Empty(t, manifests(), "the refusal drops the manifest")

	storage.mu.Lock()
	maps.Copy(storage.data, saved)
	storage.mu.Unlock()

	f.requireRefused(fiber.StatusUnauthorized, "two", "")
	f.requireRefused(fiber.StatusUnauthorized, "three", "")
	f.requireHit("b", "three")
}

func Test_Cache_RefusalRecordKeepsTheGroupOfTheLookupWhenThePathIsRewritten(t *testing.T) {
	t.Parallel()

	require.NotEqual(t, groupOf("/account"), groupOf("/elsewhere"))

	for _, storage := range storageKinds {
		t.Run(storage, func(t *testing.T) {
			t.Parallel()

			var ended atomic.Bool
			app := fiber.New()
			app.Use(New(Config{Storage: storageOf(storage), KeyCookies: []string{"session"}}))
			app.Use(func(c fiber.Ctx) error {
				if c.Get("X-Rewrite") != "" {
					c.Path("/elsewhere")
					return c.SendStatus(fiber.StatusUnauthorized)
				}
				return c.Next()
			})
			app.Get("/account", func(c fiber.Ctx) error {
				if ended.Load() {
					return c.SendStatus(fiber.StatusUnauthorized)
				}
				return c.SendString("account for " + c.Get(fiber.HeaderAccept))
			})
			get := func(accept string, headers map[string]string) reply {
				req := httptest.NewRequest(fiber.MethodGet, "/account", http.NoBody)
				req.Header.Set(fiber.HeaderCookie, "session=a")
				req.Header.Set(fiber.HeaderAccept, accept)
				for name, value := range headers {
					req.Header.Set(name, value)
				}
				return doRequest(t, app, req)
			}

			require.Equal(t, cacheMiss, get("text/html", nil).cache)
			require.Equal(t, cacheMiss, get("text/plain", nil).cache)

			ended.Store(true)
			reload := map[string]string{"X-Rewrite": "1", fiber.HeaderCacheControl: "no-cache"}
			require.Equal(t, fiber.StatusUnauthorized, get("text/html", reload).status)

			require.Equal(t, fiber.StatusUnauthorized, get("text/plain", nil).status, "the other variant was served from the cache")
		})
	}
}

// The entry lifetime noted for the record counts for the group the lookup saw, even if a middleware rewrites the path.
func Test_Cache_RefusalRecordOutlivesTheGroupsEntriesWhenThePathIsRewritten(t *testing.T) {
	t.Parallel()

	require.NotEqual(t, groupOf("/account"), groupOf("/elsewhere"))

	storage := newFailingCacheStorage()
	var ended atomic.Bool
	app := fiber.New()
	app.Use(New(Config{Storage: storage, KeyCookies: []string{"session"}}))
	app.Use(func(c fiber.Ctx) error {
		if c.Query("v") == "long" {
			c.Path("/elsewhere")
			c.Set(fiber.HeaderCacheControl, "max-age=3600")
			return c.SendString("kept for an hour")
		}
		return c.Next()
	})
	app.Get("/account", func(c fiber.Ctx) error {
		if ended.Load() {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.SendString("account")
	})
	get := func(uri, cacheControl string) reply {
		req := httptest.NewRequest(fiber.MethodGet, uri, http.NoBody)
		req.Header.Set(fiber.HeaderCookie, "session=a")
		if cacheControl != "" {
			req.Header.Set(fiber.HeaderCacheControl, cacheControl)
		}
		return doRequest(t, app, req)
	}

	stored := uint64(time.Now().Unix())
	require.Equal(t, cacheMiss, get("/account?v=long", "").cache)
	require.Equal(t, cacheMiss, get("/account?v=short", "").cache)

	ended.Store(true)
	require.Equal(t, fiber.StatusUnauthorized, get("/account?v=short", "no-cache").status)

	// The record lasts as long as the hour-long entry of the group, not the minute the refused one had left.
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	records := 0
	for key, value := range storage.data {
		if isRefusalRecord(key) {
			records++
			require.GreaterOrEqual(t, lapseOf(value), stored+3598, key)
		}
	}
	require.Equal(t, 1, records)
}

// The app is not asked about an entry the request already rules out: that stays so a moment later.
func Test_Cache_InvalidatorIsNotAskedAboutAnEntryRuledOut(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		cacheControl string
		age          time.Duration // how long ago the entry was stored
		asked        bool
	}{
		{name: "a plain hit", asked: true},
		{name: "max-age=0", cacheControl: "max-age=0"},
		{name: "older than max-age", cacheControl: "max-age=5", age: 10 * time.Second},
		{name: "younger than max-age", cacheControl: "max-age=60", age: 10 * time.Second, asked: true},
		{name: "min-fresh beyond what is left", cacheControl: "min-fresh=3600"},
		{name: "min-fresh within what is left", cacheControl: "min-fresh=5", asked: true},
	} {
		for _, storage := range storageKinds {
			t.Run(tc.name+"/"+storage, func(t *testing.T) {
				t.Parallel()

				var asked atomic.Int32
				clock := newTestClock(time.Unix(1_700_000_000, 0))
				app := fiber.New()
				app.Use(New(Config{
					Storage:    storageOf(storage),
					Expiration: time.Minute,
					clock:      clock.Now,
					CacheInvalidator: func(fiber.Ctx) bool {
						asked.Add(1)
						return false
					},
				}))
				app.Get("/", func(c fiber.Ctx) error { return c.SendString("body") })

				require.Equal(t, cacheMiss, getRoot(t, app, nil).cache)
				require.Zero(t, asked.Load(), "a miss has no entry to ask about")
				clock.Add(tc.age)

				headers := map[string]string{}
				if tc.cacheControl != "" {
					headers[fiber.HeaderCacheControl] = tc.cacheControl
				}
				getRoot(t, app, headers)
				want := int32(0)
				if tc.asked {
					want = 1
				}
				require.Equal(t, want, asked.Load())
			})
		}
	}
}

// Nor is it asked about an entry that lasted under a second, which is found expired whenever it is found.
func Test_Cache_InvalidatorIsNotAskedAboutAnEntryThatHasExpired(t *testing.T) {
	t.Parallel()

	for _, storage := range storageKinds {
		t.Run(storage, func(t *testing.T) {
			t.Parallel()

			var asked atomic.Int32
			app := fiber.New()
			app.Use(New(Config{
				Storage:             storageOf(storage),
				ExpirationGenerator: func(fiber.Ctx, *Config) time.Duration { return 500 * time.Millisecond },
				CacheInvalidator: func(fiber.Ctx) bool {
					asked.Add(1)
					return false
				},
			}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("body") })

			require.Equal(t, cacheMiss, getRoot(t, app, nil).cache)
			require.Equal(t, cacheMiss, getRoot(t, app, nil).cache, "the entry was found expired")
			require.Zero(t, asked.Load())
		})
	}
}

// hookedStorage runs onGet, onSet and onDelete before it reads, writes and deletes a key, when set.
type hookedStorage struct {
	*failingCacheStorage
	onGet    func(key string)
	onSet    func(key string, exp time.Duration)
	onDelete func(key string)
}

func (s *hookedStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	if s.onSet != nil {
		s.onSet(key, exp)
	}
	return s.failingCacheStorage.SetWithContext(ctx, key, val, exp)
}

func (s *hookedStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if s.onGet != nil {
		s.onGet(key)
	}
	return s.failingCacheStorage.GetWithContext(ctx, key)
}

func (s *hookedStorage) DeleteWithContext(ctx context.Context, key string) error {
	if s.onDelete != nil {
		s.onDelete(key)
	}
	return s.failingCacheStorage.DeleteWithContext(ctx, key)
}
