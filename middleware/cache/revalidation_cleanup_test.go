package cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Variants share a manifest while their bodies occupy distinct storage keys.
// Mutations happen during the origin call, after the stale entry was read.
func Test_Cache_UncacheableVaryCleanup(t *testing.T) {
	t.Parallel()

	for _, external := range []bool{false, true} {
		for _, directive := range []string{"private", "no-cache", "no-store", "VaryStar"} {
			for _, scenario := range []string{"success", "absent", "readError", "metadataDeleteError", "bodyDeleteError", "replaced", "plainNoStoreMiss"} {
				if scenario == "plainNoStoreMiss" && directive != "no-store" {
					continue
				}
				if !external && (scenario == "readError" || scenario == "metadataDeleteError" || scenario == "bodyDeleteError") {
					continue
				}
				name := "memory/" + directive + "/" + scenario
				if external {
					name = "external/" + directive + "/" + scenario
				}
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					storage := newFailingCacheStorage()
					clock := newTestClock(time.Now())
					cfg := Config{
						Expiration: time.Hour,
						MaxBytes:   10,
						clock:      clock.Now,
						KeyGenerator: func(c fiber.Ctx) string {
							return c.Path()
						},
					}
					if external {
						cfg.Storage = storage
					}
					baseKey := cacheKeyVersion + "|GET|/cached"
					manifestKey := baseKey + "|vary"
					var header fasthttp.RequestHeader
					header.Set("X-Variant", "a")
					key := varyKey(baseKey, []string{"x-variant"}, &header, true)
					app := fiber.New()
					app.Use(New(cfg))
					app.Get("/cached", func(c fiber.Ctx) error {
						c.Set(fiber.HeaderVary, "X-Variant")
						c.Set(fiber.HeaderCacheControl, "public, max-age=60")
						if c.Get("X-Action") == "cleanup" {
							if scenario == "replaced" || (!external && scenario == "absent") {
								variant := "a"
								if scenario == "absent" {
									variant = "c" // Evicts a, whose expiry precedes b's.
								}
								request := httptest.NewRequest(fiber.MethodGet, "/cached", http.NoBody)
								request.Header.Set("X-Variant", variant)
								request.Header.Set(fiber.HeaderCacheControl, "max-age=0")
								response, err := app.Test(request)
								if err != nil {
									return err
								}
								if err := response.Body.Close(); err != nil {
									return fmt.Errorf("close nested response: %w", err)
								}
								if response.StatusCode != fiber.StatusOK {
									return errors.New("nested store failed")
								}
							} else if external {
								storage.mu.Lock()
								switch scenario {
								case "absent":
									delete(storage.data, key)
									delete(storage.data, key+"_body")
								case "readError":
									storage.errs["get|"+key] = errors.New("verification failed")
								case "metadataDeleteError":
									storage.errs["del|"+key] = errors.New("metadata delete failed")
								case "bodyDeleteError":
									storage.errs["del|"+key+"_body"] = errors.New("body delete failed")
								}
								storage.mu.Unlock()
							}
							if directive == "VaryStar" {
								c.Set(fiber.HeaderVary, "*")
							} else {
								c.Set(fiber.HeaderCacheControl, directive)
							}
							return c.SendString("newer")
						}
						return c.SendString("fresh")
					})

					request := func(variant, action string) (*http.Response, string) {
						t.Helper()
						req := httptest.NewRequest(fiber.MethodGet, "/cached", http.NoBody)
						req.Header.Set("X-Variant", variant)
						if action != "" {
							req.Header.Set("X-Action", action)
							req.Header.Set(fiber.HeaderCacheControl, "max-age=0")
						}
						response, err := app.Test(req)
						require.NoError(t, err)
						body, err := io.ReadAll(response.Body)
						require.NoError(t, err)
						require.NoError(t, response.Body.Close())
						return response, string(body)
					}
					for _, variant := range []string{"a", "b"} {
						response, _ := request(variant, "")
						require.Equal(t, cacheMiss, response.Header.Get("X-Cache"))
						clock.Add(time.Second)
					}
					variant := "a"
					if scenario == "plainNoStoreMiss" {
						variant = "c"
					}
					response, body := request(variant, "cleanup")
					require.Equal(t, fiber.StatusOK, response.StatusCode)
					require.Equal(t, "newer", body)
					require.Equal(t, cacheUnreachable, response.Header.Get("X-Cache"))
					preserveManifest := scenario == "replaced" || scenario == "plainNoStoreMiss"
					if external {
						storage.mu.Lock()
						_, manifestPresent := storage.data[manifestKey]
						storage.errs = make(map[string]error)
						storage.mu.Unlock()
						require.Equal(t, preserveManifest, manifestPresent)
					}
					response, body = request("b", "")
					require.Equal(t, fiber.StatusOK, response.StatusCode)
					require.Equal(t, "fresh", body)
					wantStatus := cacheMiss
					if preserveManifest {
						wantStatus = cacheHit
					}
					require.Equal(t, wantStatus, response.Header.Get("X-Cache"))
					if scenario == "replaced" {
						response, _ = request("a", "")
						require.Equal(t, cacheHit, response.Header.Get("X-Cache"))
					}
				})
			}
		}
	}
}

// A reload that lands on an error page must not purge the entry or the manifest
// its sibling variants hang on; a cacheable status still does. A bare 401, 403 or
// 407 keeps them as well, since no credential is in the key.
func Test_Cache_UncacheableStatusKeepsEntryAndManifest(t *testing.T) {
	t.Parallel()

	answers := []struct {
		name      string
		directive string
		status    int
		purged    bool // the entry and the manifest go, so no variant is a hit afterwards
	}{
		{name: "503", status: fiber.StatusServiceUnavailable},
		{name: "503 no-store", status: fiber.StatusServiceUnavailable, directive: "no-store"},
		{name: "503 private", status: fiber.StatusServiceUnavailable, directive: "private"},
		{name: "503 no-cache", status: fiber.StatusServiceUnavailable, directive: "no-cache"},
		{name: "503 VaryStar", status: fiber.StatusServiceUnavailable, directive: "VaryStar"},
		{name: "429 no-store", status: fiber.StatusTooManyRequests, directive: "no-store"},
		{name: "401", status: fiber.StatusUnauthorized},
		{name: "403", status: fiber.StatusForbidden},
		{name: "407", status: fiber.StatusProxyAuthRequired},
		{name: "404 no-store", status: fiber.StatusNotFound, directive: "no-store", purged: true},
	}

	for _, external := range []bool{false, true} {
		for _, reload := range []string{"max-age=0", "no-cache"} {
			for _, answer := range answers {
				name := "memory/" + reload + "/" + answer.name
				if external {
					name = "external/" + reload + "/" + answer.name
				}
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					cfg := Config{}
					if external {
						cfg.Storage = newFailingCacheStorage()
					}
					app := fiber.New()
					app.Use(New(cfg))
					app.Get("/cached", func(c fiber.Ctx) error {
						if c.Get(fiber.HeaderCacheControl) == "" {
							c.Set(fiber.HeaderVary, "X-Variant")
							c.Set(fiber.HeaderCacheControl, "public, max-age=60")
							return c.SendString("fresh")
						}
						switch answer.directive {
						case "":
						case "VaryStar":
							c.Set(fiber.HeaderVary, "*")
						default:
							c.Set(fiber.HeaderCacheControl, answer.directive)
						}
						return c.SendStatus(answer.status)
					})

					request := func(variant, cacheControl string) (int, string) {
						t.Helper()
						req := httptest.NewRequest(fiber.MethodGet, "/cached", http.NoBody)
						req.Header.Set("X-Variant", variant)
						if cacheControl != "" {
							req.Header.Set(fiber.HeaderCacheControl, cacheControl)
						}
						response, err := app.Test(req)
						require.NoError(t, err)
						require.NoError(t, response.Body.Close())
						return response.StatusCode, response.Header.Get("X-Cache")
					}

					for _, variant := range []string{"a", "b"} {
						_, cacheStatus := request(variant, "")
						require.Equal(t, cacheMiss, cacheStatus)
					}
					status, cacheStatus := request("a", reload)
					require.Equal(t, answer.status, status)
					require.Equal(t, cacheUnreachable, cacheStatus)

					want := cacheHit
					if answer.purged {
						want = cacheMiss
					}
					// Sibling first: a miss on a would store the manifest again.
					for _, variant := range []string{"b", "a"} {
						_, cacheStatus = request(variant, "")
						require.Equal(t, want, cacheStatus)
					}
				})
			}
		}
	}
}

// credentialPartition is one way the cache can tell sessions apart, and how the
// fixture app is sent and reads the session for it.
type credentialPartition struct {
	configure    func(*Config)               // makes the cache tell sessions apart
	send         func(*http.Request, string) // puts a session on a request
	read         func(fiber.Ctx) string      // takes it off again
	vary         string                      // the Vary the app adds, if any
	cacheControl string                      // the Cache-Control the app adds, if any
	recognized   bool                        // the cache takes it for a credential by itself
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

// headerPartition carries the session in the named request header.
func headerPartition(name string, configure func(*Config), recognized bool) credentialPartition {
	return credentialPartition{
		configure:  configure,
		send:       func(req *http.Request, session string) { req.Header.Set(name, session) },
		read:       func(c fiber.Ctx) string { return c.Get(name) },
		recognized: recognized,
	}
}

var credentialPartitions = map[string]credentialPartition{
	"keyCookies": {
		configure:  func(cfg *Config) { cfg.KeyCookies = []string{"session"} },
		send:       sendCookie,
		read:       readCookie,
		recognized: true,
	},
	"varyCookie": {send: sendCookie, read: readCookie, vary: fiber.HeaderCookie, recognized: true},
	"authorization": {
		send: sendBearer,
		read: readBearer,
		// A request carrying Authorization is stored only when the response says a
		// shared cache may.
		cacheControl: "public, max-age=60",
		recognized:   true,
	},
	// An API key, which the cache takes for a credential by name.
	"apiKey": headerPartition("X-Api-Key", func(cfg *Config) { cfg.KeyHeaders = []string{"X-Api-Key"} }, true),
	// A header of the app's own, which it declares one in KeyCredentialHeaders.
	"credentialHeader": headerPartition("X-Company-Token", func(cfg *Config) { cfg.KeyCredentialHeaders = []string{"X-Company-Token"} }, true),
	// Varies on a header the cache recognizes, whatever the key is built from.
	"varyApiKey": func() credentialPartition {
		p := headerPartition("X-Api-Key", nil, true)
		p.vary = "X-Api-Key"
		return p
	}(),
	// Headers nothing in their name or in the config marks as a credential.
	"tenant":       headerPartition("X-Tenant", func(cfg *Config) { cfg.KeyHeaders = []string{"X-Tenant"} }, false),
	"customHeader": headerPartition("X-Session", func(cfg *Config) { cfg.KeyHeaders = []string{"X-Session"} }, false),
	"customVary": func() credentialPartition {
		p := headerPartition("X-Session", nil, false)
		p.vary = "X-Session"
		return p
	}(),
}

// credentialFixture is an app behind the cache that answers per session: valid sessions
// get a body naming them and their X-Variant, any other is handed to refuse.
type credentialFixture struct {
	app       *fiber.App
	t         *testing.T
	probe     *accountingProbe
	ended     sync.Map
	partition credentialPartition
	calls     atomic.Int32
}

type credentialOptions struct {
	refuse func(fiber.Ctx) error // the answer to a session that is not valid, a bare 401 when nil
	// clock and expiration push an entry past its lifetime while storage still holds it.
	clock      *testClock
	custom     fiber.Storage // used instead of storage when set
	partition  string        // a key of credentialPartitions
	storage    string        // memory or external
	vary       string        // a Vary the served answer carries besides the partition's
	valid      []string      // the sessions the app answers, "a" and "b" when nil
	expiration time.Duration
	// invalidateEnded has CacheInvalidator report the sessions the app has ended.
	invalidateEnded bool
}

// storageKinds are the storages every scenario runs against.
var storageKinds = []string{"memory", "external"}

// reply is what a request got back: its status, headers, the cache's verdict and the body.
type reply struct {
	header http.Header
	cache  string
	body   string
	status int
}

// doRequest runs req through app and returns what came back, the body read and closed.
func doRequest(t *testing.T, app *fiber.App, req *http.Request) reply {
	t.Helper()

	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return reply{status: resp.StatusCode, header: resp.Header, cache: resp.Header.Get("X-Cache"), body: string(body)}
}

// getRoot sends GET / with the headers.
func getRoot(t *testing.T, app *fiber.App, headers map[string]string) reply {
	t.Helper()

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	return doRequest(t, app, req)
}

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

	f := &credentialFixture{t: t, probe: &accountingProbe{}, partition: partition}
	// A byte limit turns on the heap accounting a purge has to keep honest.
	cfg := Config{MaxBytes: 1 << 20, accounting: f.probe.record, Expiration: opts.expiration}
	if opts.clock != nil {
		cfg.clock = opts.clock.Now
	}
	if partition.configure != nil {
		partition.configure(&cfg)
	}
	switch {
	case opts.custom != nil:
		cfg.Storage = opts.custom
	case opts.storage == "external":
		cfg.Storage = newFailingCacheStorage()
	}
	if opts.invalidateEnded {
		cfg.CacheInvalidator = func(c fiber.Ctx) bool {
			_, ended := f.ended.Load(partition.read(c))
			return ended
		}
	}

	f.app = fiber.New()
	f.app.Use(New(cfg))
	f.app.Get("/account", func(c fiber.Ctx) error {
		f.calls.Add(1)
		session := partition.read(c)
		if _, ended := f.ended.Load(session); ended || !slices.Contains(valid, session) {
			return refuse(c)
		}

		vary := slices.DeleteFunc([]string{partition.vary, opts.vary}, func(name string) bool { return name == "" })
		if len(vary) > 0 {
			c.Set(fiber.HeaderVary, strings.Join(vary, ", "))
		}
		if partition.cacheControl != "" {
			c.Set(fiber.HeaderCacheControl, partition.cacheControl)
		}
		return c.SendString(accountBody(session, c.Get("X-Variant")))
	})
	return f
}

// get asks for the account as session, which is sent the way the partition
// carries it; an empty session sends nothing.
func (f *credentialFixture) get(session, variant, cacheControl string) reply {
	f.t.Helper()

	req := httptest.NewRequest(fiber.MethodGet, "/account", http.NoBody)
	if session != "" {
		f.partition.send(req, session)
	}
	if variant != "" {
		req.Header.Set("X-Variant", variant)
	}
	if cacheControl != "" {
		req.Header.Set(fiber.HeaderCacheControl, cacheControl)
	}
	return doRequest(f.t, f.app, req)
}

// requireHit asserts the session is answered from the cache with its own data.
func (f *credentialFixture) requireHit(session, variant string) {
	f.t.Helper()

	got := f.get(session, variant, "")
	require.Equal(f.t, fiber.StatusOK, got.status)
	require.Equal(f.t, accountBody(session, variant), got.body)
	require.Equal(f.t, cacheHit, got.cache)
}

// requireRefused asserts a request of session "a", the one the app has ended, reached
// the origin, which refused it with status, and returns what came back.
func (f *credentialFixture) requireRefused(status int, variant, cacheControl string) reply {
	f.t.Helper()

	calls := f.calls.Load()
	got := f.get("a", variant, cacheControl)
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

// refusingAfter refuses with a 401 once strip has taken the credential out of the
// request, as a handler that clears a session it has just rejected would.
func refusingAfter(strip func(*fasthttp.RequestHeader)) func(fiber.Ctx) error {
	return func(c fiber.Ctx) error {
		strip(&c.Request().Header)
		return c.SendStatus(fiber.StatusUnauthorized)
	}
}

// A reload answered 401, 403 or 407 refuses the credential its entry was stored under:
// the entry must go and other credentials' entries stay, even for a bare refusal when
// the cache recognizes the credential, and for any partition when it carries a directive.
func Test_Cache_RejectedCredentialDropsEntry(t *testing.T) {
	t.Parallel()

	for _, partition := range []string{"keyCookies", "varyCookie", "varyApiKey", "authorization", "customHeader", "customVary"} {
		for _, storage := range storageKinds {
			for _, reload := range []string{"max-age=0", "no-cache"} {
				for _, status := range []int{fiber.StatusUnauthorized, fiber.StatusForbidden, fiber.StatusProxyAuthRequired} {
					for _, directive := range []string{"bare", "private", "no-store", "no-cache", "VaryStar"} {
						if directive == "bare" && !credentialPartitions[partition].recognized {
							continue
						}
						t.Run(fmt.Sprintf("%s/%s/%s/%d/%s", partition, storage, reload, status, directive), func(t *testing.T) {
							t.Parallel()

							f := newCredentialFixture(t, &credentialOptions{
								partition: partition,
								storage:   storage,
								refuse:    refusal(status, directive),
							})

							bodySize := uint(len(accountBody("a", "")))
							for _, session := range []string{"a", "b"} {
								got := f.get(session, "", "")
								require.Equal(t, fiber.StatusOK, got.status)
								require.Equal(t, accountBody(session, ""), got.body)
								require.Equal(t, cacheMiss, got.cache)
							}
							require.Equal(t, cacheHit, f.get("a", "", "").cache)
							require.Equal(t, 2*bodySize, f.probe.counted())
							require.Equal(t, 2, f.probe.nodes())

							// A refusal that found no entry of its own purges nothing.
							for _, cacheControl := range []string{"", reload} {
								got := f.get("stranger", "", cacheControl)
								require.Equal(t, status, got.status)
								require.Equal(t, cacheUnreachable, got.cache)
							}
							for _, session := range []string{"b", "a"} {
								require.Equal(t, cacheHit, f.get(session, "", "").cache)
							}
							require.Equal(t, 2*bodySize, f.probe.counted())
							require.Equal(t, 2, f.probe.nodes())

							// The app ends the session; the reload reaches the origin.
							f.ended.Store("a", true)
							got := f.get("a", "", reload)
							require.Equal(t, status, got.status)
							require.Equal(t, cacheUnreachable, got.cache)

							// The same credential must now meet the origin again, not the old body.
							got = f.requireRefused(status, "", "")
							require.Equal(t, cacheUnreachable, got.cache)

							// The purge gave back exactly the refused entry's accounting.
							require.Equal(t, bodySize, f.probe.counted())
							require.Equal(t, 1, f.probe.nodes())

							// Only the refused credential's entry went.
							f.requireHit("b", "")
						})
					}
				}
			}
		}
	}
}

// A handler that refuses by returning the error (fiber.ErrUnauthorized) writes no
// status for the cache to see, yet the refusal still ends the entry; an error
// that says nothing about the credential does not.
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
			for _, reload := range []string{"max-age=0", "no-cache"} {
				for _, answer := range answers {
					t.Run(fmt.Sprintf("%s/%s/%s/%s", partition, storage, reload, answer.name), func(t *testing.T) {
						t.Parallel()

						f := newCredentialFixture(t, &credentialOptions{
							partition: partition,
							storage:   storage,
							refuse:    func(fiber.Ctx) error { return answer.err },
						})
						for _, session := range []string{"a", "b"} {
							require.Equal(t, cacheMiss, f.get(session, "", "").cache)
						}

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

// A refused credential takes its other variants along when its Vary manifest is
// its own, that is when the key holds the credential. A manifest shared with other
// credentials, and an entry no credential is keyed on, stay.
func Test_Cache_RejectedCredentialDropsItsVariants(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		refuse    func(fiber.Ctx) error // the refusal, a bare 401 when nil
		name      string
		partition string
		own       string // the refused request's own entry: gone or kept
		sibling   string // the credential's other variant: gone or kept
		anonymous bool
	}{
		{name: "KeyCookies", partition: "keyCookies", own: "gone", sibling: "gone"},
		{name: "Authorization", partition: "authorization", own: "gone", sibling: "gone"},
		// The manifest is shared by every session, so it stays; the refusal is recorded
		// instead, and the refused session's other variant is revalidated, not served.
		{name: "Vary Cookie", partition: "varyCookie", own: "gone", sibling: "gone"},
		{name: "Vary X-API-Key", partition: "varyApiKey", own: "gone", sibling: "gone"},
		{name: "API key header", partition: "apiKey", own: "gone", sibling: "gone"},
		{name: "header declared in KeyCredentialHeaders", partition: "credentialHeader", own: "gone", sibling: "gone"},
		// Nothing in the name tells a tenant from a credential, so what is keyed on it is shared.
		{name: "tenant header, not a credential", partition: "tenant", own: "kept", sibling: "kept"},
		// An undeclared header: a bare refusal says nothing about its entry, one that says
		// not to keep it ends the entry, and the manifest stays.
		{name: "undeclared header, bare refusal", partition: "customHeader", own: "kept", sibling: "kept"},
		{
			name: "undeclared header, private refusal", partition: "customHeader", own: "gone", sibling: "kept",
			refuse: refusal(fiber.StatusUnauthorized, "private"),
		},
		// Directives change nothing about the siblings: the other sessions keep theirs.
		{
			name: "Vary Cookie, no-store refusal", partition: "varyCookie", own: "gone", sibling: "gone",
			refuse: refusal(fiber.StatusUnauthorized, "no-store"),
		},
		{
			name: "Vary X-API-Key, private refusal", partition: "varyApiKey", own: "gone", sibling: "gone",
			refuse: refusal(fiber.StatusUnauthorized, "private"),
		},
		// A Vary on a header nothing marks as a credential gets no record: the cache
		// cannot tell the header from a tenant, and the sibling ages out.
		{name: "undeclared Vary, bare refusal", partition: "customVary", own: "kept", sibling: "kept"},
		{
			name: "undeclared Vary, private refusal", partition: "customVary", own: "gone", sibling: "kept",
			refuse: refusal(fiber.StatusUnauthorized, "private"),
		},
		{name: "no credential in the key", partition: "keyCookies", own: "kept", sibling: "kept", anonymous: true},
		// The key is decided before the handler runs, so clearing the credential cannot change it.
		{
			name: "KeyCookies, cookie cleared by the handler", partition: "keyCookies", own: "gone", sibling: "gone",
			refuse: refusingAfter(func(h *fasthttp.RequestHeader) { h.DelAllCookies() }),
		},
		{
			name: "Authorization, header cleared by the handler", partition: "authorization", own: "gone", sibling: "gone",
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

				for _, session := range []string{refused, other} {
					for _, variant := range []string{"one", "two"} {
						got := f.get(session, variant, "")
						require.Equal(t, fiber.StatusOK, got.status)
						require.Equal(t, accountBody(session, variant), got.body)
						require.Equal(t, cacheMiss, got.cache)
					}
				}

				f.ended.Store(refused, true)
				got := f.get(refused, "one", "no-cache")
				require.Equal(t, fiber.StatusUnauthorized, got.status)
				require.Equal(t, cacheUnreachable, got.cache)

				got = f.get(refused, "one", "")
				switch scenario.own {
				case "gone":
					require.Equal(t, fiber.StatusUnauthorized, got.status, "the refused variant was answered from the cache")
				case "kept":
					require.Equal(t, cacheHit, got.cache, "a refusal took out an entry every client shares")
				}

				switch scenario.sibling {
				case "gone":
					got = f.get(refused, "two", "")
					require.Equal(t, fiber.StatusUnauthorized, got.status, "a variant of the refused credential was answered from the cache")
				case "kept":
					require.Equal(t, cacheHit, f.get(refused, "two", "").cache)
				}

				for _, variant := range []string{"one", "two"} {
					f.requireHit(other, variant)
				}
			})
		}
	}
}

// The recipe in the docs: KeyCookies plus a CacheInvalidator that reports ended
// sessions. The next request, even one accepting stale entries, meets the origin.
func Test_Cache_InvalidatorCutsOffEndedSession(t *testing.T) {
	t.Parallel()

	for _, storage := range storageKinds {
		for _, request := range []struct{ name, cacheControl string }{{"plain", ""}, {"max-stale", "max-stale"}} {
			t.Run(storage+"/"+request.name, func(t *testing.T) {
				t.Parallel()

				f := newCredentialFixture(t, &credentialOptions{
					partition:       "keyCookies",
					storage:         storage,
					invalidateEnded: true,
				})
				for _, session := range []string{"a", "b"} {
					require.Equal(t, cacheMiss, f.get(session, "", "").cache)
				}
				require.Equal(t, cacheHit, f.get("a", "", "").cache)

				f.ended.Store("a", true)
				f.requireRefused(fiber.StatusUnauthorized, "", request.cacheControl)

				f.requireHit("b", "")
			})
		}
	}
}

// refusalStorage counts the reads of refusal records, which are the keys naming
// "|refused|", and can fail them.
type refusalStorage struct {
	*failingCacheStorage
	reads atomic.Int32
	fail  atomic.Bool
}

func (s *refusalStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if strings.Contains(key, "|refused|") {
		s.reads.Add(1)
		if s.fail.Load() {
			return nil, errors.New("storage unavailable")
		}
	}
	return s.failingCacheStorage.GetWithContext(ctx, key)
}

// A refused session's other variants stay out whatever the other sessions do to the
// URL: a miss of theirs stores the Vary manifest again.
func Test_Cache_RefusedCredentialStaysOutWhileOthersMiss(t *testing.T) {
	t.Parallel()

	for _, partition := range []string{"varyCookie", "varyApiKey"} {
		for _, storage := range storageKinds {
			for _, directive := range []string{"bare", "no-store"} {
				t.Run(partition+"/"+storage+"/"+directive, func(t *testing.T) {
					t.Parallel()

					f := newCredentialFixture(t, &credentialOptions{
						partition: partition,
						storage:   storage,
						vary:      "X-Variant",
						refuse:    refusal(fiber.StatusUnauthorized, directive),
					})
					for _, session := range []string{"a", "b"} {
						for _, variant := range []string{"one", "two"} {
							require.Equal(t, cacheMiss, f.get(session, variant, "").cache)
						}
					}

					f.ended.Store("a", true)
					require.Equal(t, fiber.StatusUnauthorized, f.get("a", "one", "no-cache").status)

					// Another session misses on a variant of its own, which stores the manifest again.
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

// Only a response that varies on a credential costs the read of a refusal record.
func Test_Cache_RefusalRecordIsReadOnlyForCredentialVary(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		partition string
		vary      string
		reads     int32 // per hit
	}{
		{name: "no Vary", partition: "tenant"},
		{name: "Vary: X-Variant", partition: "tenant", vary: "X-Variant"},
		{name: "Vary: Cookie", partition: "varyCookie", reads: 1},
		{name: "Vary: Cookie, X-Variant", partition: "varyCookie", vary: "X-Variant", reads: 1},
		{name: "Vary: X-API-Key", partition: "varyApiKey", reads: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			storage := &refusalStorage{failingCacheStorage: newFailingCacheStorage()}
			f := newCredentialFixture(t, &credentialOptions{
				partition: tc.partition,
				custom:    storage,
				vary:      tc.vary,
				valid:     []string{"a"},
			})
			require.Equal(t, cacheMiss, f.get("a", "", "").cache)
			require.Zero(t, storage.reads.Load(), "a miss has nothing to check")

			for range 3 {
				require.Equal(t, cacheHit, f.get("a", "", "").cache)
			}
			require.Equal(t, 3*tc.reads, storage.reads.Load())
		})
	}
}

// A storage that cannot say whether the credential was refused is not a reason to
// serve it: the entry is revalidated instead.
func Test_Cache_UnreadableRefusalRecordRevalidates(t *testing.T) {
	t.Parallel()

	storage := &refusalStorage{failingCacheStorage: newFailingCacheStorage()}
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

// A session the app ends without the client reloading is cut off by the
// CacheInvalidator recipe, every variant of it at once.
func Test_Cache_InvalidatorEndsEveryVariantOfASession(t *testing.T) {
	t.Parallel()

	for _, storage := range storageKinds {
		t.Run(storage, func(t *testing.T) {
			t.Parallel()

			f := newCredentialFixture(t, &credentialOptions{
				partition:       "varyCookie",
				storage:         storage,
				vary:            "X-Variant",
				invalidateEnded: true,
			})
			variants := []string{"one", "two"}
			for _, session := range []string{"a", "b"} {
				for _, variant := range variants {
					require.Equal(t, cacheMiss, f.get(session, variant, "").cache)
				}
			}

			f.ended.Store("a", true)
			for _, variant := range variants {
				f.requireRefused(fiber.StatusUnauthorized, variant, "")
			}

			for _, variant := range variants {
				f.requireHit("b", variant)
			}
		})
	}
}

// An entry holding a credential's response is never served stale: the storage
// keeps it past its lifetime, so max-stale would otherwise extend it.
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
			for _, scenario := range scenarios {
				t.Run(storage+"/"+maxStale+"/"+scenario.name, func(t *testing.T) {
					t.Parallel()

					clock := newTestClock(time.Unix(1_700_000_000, 0))
					f := newCredentialFixture(t, &credentialOptions{
						partition:  scenario.partition,
						storage:    storage,
						clock:      clock,
						expiration: time.Minute,
					})
					for _, session := range []string{"a", "b"} {
						require.Equal(t, cacheMiss, f.get(session, "", "").cache)
					}

					// Past the entry's lifetime, well inside what the storage keeps.
					clock.Add(2 * time.Minute)
					f.ended.Store("a", true)
					if scenario.stale {
						calls := f.calls.Load()
						got := f.get("a", "", maxStale)
						require.Equal(t, fiber.StatusOK, got.status)
						require.Equal(t, accountBody("a", ""), got.body)
						require.Equal(t, cacheHit, got.cache)
						require.Equal(t, calls, f.calls.Load())
						return
					}
					f.requireRefused(fiber.StatusUnauthorized, "", maxStale)
				})
			}
		}
	}
}

// A refusal is a verdict on a credential, so it leaves an entry whose key holds none.
// A custom KeyGenerator is opaque to the middleware, which assumes it may hold one.
func Test_Cache_RefusalLeavesAnEntryNoCredentialIsKeyedOn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  Config
		kept bool
	}{
		{name: "default key", kept: true},
		{name: "KeyHeaders naming a tenant", cfg: Config{KeyHeaders: []string{"X-Tenant"}}, kept: true},
		{name: "KeyCookies the request does not carry", cfg: Config{KeyCookies: []string{"session"}}, kept: true},
		{name: "custom KeyGenerator", cfg: Config{KeyGenerator: func(c fiber.Ctx) string { return c.Path() }}},
	}

	for _, storage := range storageKinds {
		for _, delivery := range []string{"status", "error"} {
			for _, status := range []int{fiber.StatusUnauthorized, fiber.StatusForbidden, fiber.StatusProxyAuthRequired} {
				for _, tc := range tests {
					t.Run(fmt.Sprintf("%s/%s/%d/%s", storage, delivery, status, tc.name), func(t *testing.T) {
						t.Parallel()

						cfg := tc.cfg
						if storage == "external" {
							cfg.Storage = newFailingCacheStorage()
						}
						app := fiber.New()
						app.Use(New(cfg))
						app.Get("/", func(c fiber.Ctx) error {
							if c.Get("X-Blocked") != "1" {
								return c.SendString("PUBLIC")
							}
							if delivery == "error" {
								return fiber.NewError(status)
							}
							return c.SendStatus(status)
						})

						require.Equal(t, cacheMiss, getRoot(t, app, nil).cache)
						require.Equal(t, cacheHit, getRoot(t, app, nil).cache)

						// A client the app refuses, whatever the key tells of it, reloads.
						getRoot(t, app, map[string]string{"X-Blocked": "1", fiber.HeaderCacheControl: "no-cache"})

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

func Test_CacheStorage_PartialEvictionRetainsBodyRetry(t *testing.T) {
	t.Parallel()

	for _, failBodyRead := range []bool{false, true} {
		name := "bodyPresent"
		if failBodyRead {
			name = "bodyReadError"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			storage := newFailingCacheStorage()
			probe := &accountingProbe{}
			key := cacheKeyVersion + "|GET|/a"
			app := fiber.New()
			app.Use(New(Config{
				Storage: storage, MaxBytes: 15, Expiration: time.Hour,
				accounting: probe.record,
				KeyGenerator: func(c fiber.Ctx) string {
					return c.Path()
				},
			}))
			app.Get("/:name", func(c fiber.Ctx) error {
				return c.SendString("0123456789")
			})
			request := func(path string, status int) {
				t.Helper()
				response, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
				require.NoError(t, err)
				require.Equal(t, status, response.StatusCode)
				require.NoError(t, response.Body.Close())
			}
			request("/a", fiber.StatusOK)
			storage.mu.Lock()
			storage.errs["del|"+key+"_body"] = errors.New("body deletion failed")
			if failBodyRead {
				storage.errs["get|"+key+"_body"] = errors.New("body verification failed")
			}
			storage.mu.Unlock()
			request("/b", fiber.StatusInternalServerError)
			require.Equal(t, uint(10), probe.counted())
			require.Equal(t, 1, probe.nodes())
			storage.mu.Lock()
			require.NotContains(t, storage.data, key)
			require.Contains(t, storage.data, key+"_body")
			storage.errs = make(map[string]error)
			storage.mu.Unlock()
			request("/c", fiber.StatusOK)
			storage.mu.RLock()
			require.NotContains(t, storage.data, key+"_body")
			storage.mu.RUnlock()
			require.Equal(t, uint(10), probe.counted())
			require.Equal(t, 1, probe.nodes())
		})
	}
}

func Test_manager_delIf(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"absent", "deleted", "replaced"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			manager := newManager(nil, true)
			expected := &item{body: []byte("stale")}
			replacement := &item{body: []byte("fresh")}
			want := memoryEntryAbsent
			switch scenario {
			case "deleted":
				want = memoryEntryDeleted
				manager.memory.Set("key", expected, time.Hour)
			case "replaced":
				want = memoryEntryReplaced
				manager.memory.Set("key", replacement, time.Hour)
			}
			require.Equal(t, want, manager.delIf("key", expected))
			if scenario == "replaced" {
				require.Same(t, replacement, manager.memory.Get("key"))
			} else {
				_, err := manager.get(context.Background(), "key")
				require.ErrorIs(t, err, errCacheMiss)
			}
		})
	}
}
