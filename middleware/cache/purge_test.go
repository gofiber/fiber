package cache

import (
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/storagetest"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// cacheTestPItemApp serves /item via New(*cfg): reads of its version run whileReading; writes bump it and answer status.
func cacheTestPItemApp(cfg *Config, status int, whileReading ...func()) *fiber.App {
	var version atomic.Int32
	version.Store(1)
	app := fiber.New()
	app.Use(New(*cfg))
	app.All("/item", func(c fiber.Ctx) error {
		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			version.Add(1)
			return c.SendStatus(status)
		}
		read := version.Load()
		for _, hook := range whileReading {
			hook()
		}
		return c.SendString("item v" + strconv.Itoa(int(read)))
	})
	return app
}

// RFC 9111 §4.4: a successful unsafe request ends the response stored for its path.
func Test_Cache_Security_SuccessfulUnsafeRequestEndsTheEntry(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		method string
		header [2]string // of the request that writes
		cfg    Config
		status int  // of the response to it
		ends   bool // whether the entry is gone after
	}{
		{name: "PUT", method: fiber.MethodPut, status: fiber.StatusNoContent, ends: true},
		{name: "POST", method: fiber.MethodPost, status: fiber.StatusCreated, ends: true},
		{name: "PATCH", method: fiber.MethodPatch, status: fiber.StatusOK, ends: true},
		{name: "DELETE", method: fiber.MethodDelete, status: fiber.StatusNoContent, ends: true},
		{name: "a redirect", method: fiber.MethodPost, status: fiber.StatusSeeOther, ends: true},
		{name: "a request that asks not to be stored", method: fiber.MethodPost, header: [2]string{fiber.HeaderCacheControl, "no-store"}, status: fiber.StatusOK, ends: true},
		{name: "a client error", method: fiber.MethodPost, status: fiber.StatusUnprocessableEntity},
		{name: "a server error", method: fiber.MethodPut, status: fiber.StatusInternalServerError},
		{name: "OPTIONS", method: fiber.MethodOptions, status: fiber.StatusOK},
		{name: "a method named in Methods", method: fiber.MethodPost, cfg: Config{Methods: []string{fiber.MethodGet, fiber.MethodHead, fiber.MethodPost}}, status: fiber.StatusOK},
		{
			name: "a request Next skips", method: fiber.MethodPost, status: fiber.StatusOK,
			cfg: Config{Next: func(c fiber.Ctx) bool { return c.Method() == fiber.MethodPost }},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// A test clock moved past the write, as a coarse wall clock (Windows) puts both in one tick.
			clock := cacheTestCClock()
			cfg := tc.cfg
			cfg.Expiration = time.Hour
			cfg.clock = clock.Now
			app := cacheTestPItemApp(&cfg, tc.status)

			get := func() reply { return cacheTestCGet(t, app, "/item") }
			cacheTestCRequireReply(t, get(), cacheMiss, "item v1")
			cacheTestCRequireReply(t, get(), cacheHit, "item v1")

			apptest.Expect(t, app, apptest.Req(tc.method, "/item", apptest.WithHeaderIf(tc.header[0], tc.header[1])), tc.status)
			clock.Add(time.Second)

			if tc.ends {
				cacheTestCRequireReply(t, get(), cacheMiss, "item v2")
				cacheTestCRequireReply(t, get(), cacheHit, "item v2")
			} else {
				cacheTestCRequireReply(t, get(), cacheHit, "item v1", "not ended, though the handler bumped the version")
			}
		})
	}
}

// only-if-cached forbids the handler (RFC 9111 §5.2.1.7), and nothing is stored for a method Methods does not
// name: such a request is answered 504, so a write it carries neither runs nor ends anything. Next still skips.
func Test_Cache_OnlyIfCachedForAMethodNotCached(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		method string
		status int  // of the request that sends only-if-cached
		next   bool // Next skips every request but a GET
		ran    bool // whether its handler ran
	}{
		{name: "PUT", method: fiber.MethodPut, status: fiber.StatusGatewayTimeout},
		{name: "OPTIONS", method: fiber.MethodOptions, status: fiber.StatusGatewayTimeout},
		{name: "skipped by Next", method: fiber.MethodPut, next: true, status: fiber.StatusNoContent, ran: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clock := cacheTestCClock()
			cfg := Config{Expiration: time.Hour, clock: clock.Now}
			if tc.next {
				cfg.Next = func(c fiber.Ctx) bool { return c.Method() != fiber.MethodGet }
			}
			app := cacheTestPItemApp(&cfg, fiber.StatusNoContent)
			cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheMiss, "item v1")

			req := apptest.Req(tc.method, "/item", apptest.WithHeader(fiber.HeaderCacheControl, "only-if-cached"))
			apptest.Expect(t, app, req, tc.status)
			clock.Add(time.Second)

			cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheHit, "item v1", "nothing ended the entry")
			want := "item v1"
			if tc.ran {
				want = "item v2"
			}
			require.Equal(t, want, cacheTestCGet(t, app, "/item", fiber.HeaderCacheControl, "no-cache").body)
		})
	}
}

// An empty Methods turns caching off, so the middleware passes every request on, only-if-cached included.
func Test_Cache_OnlyIfCachedWithCachingTurnedOff(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Methods: []string{}}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("page") })
	apptest.ExpectBody(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderCacheControl, "only-if-cached")), fiber.StatusOK, "page")
}

func Test_Cache_Security_WriteEndsEveryEntryOfThePath(t *testing.T) {
	t.Parallel()
	// A path that does not share the slot of /item, which the table chooses by a hash of its own.
	purges := newPurgeTable(time.Now)
	elsewhere := "/elsewhere"
	for i := 0; purges.slot("example.com", elsewhere) == purges.slot("example.com", "/item"); i++ {
		elsewhere = "/elsewhere" + strconv.Itoa(i)
	}

	var version atomic.Int32
	version.Store(1)
	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour, purges: purges}))
	app.All("/*", func(c fiber.Ctx) error {
		if c.Method() == fiber.MethodPut {
			version.Add(1)
			return c.SendStatus(fiber.StatusNoContent)
		}
		c.Vary(fiber.HeaderAcceptLanguage)
		return c.SendString(c.Path() + " v" + strconv.Itoa(int(version.Load())) + " " + c.Get(fiber.HeaderAcceptLanguage))
	})

	type request struct {
		method, target, language string
	}
	send := func(r request) reply {
		return doRequest(t, app, apptest.Req(r.method, r.target, apptest.WithHeaderIf(fiber.HeaderAcceptLanguage, r.language)))
	}
	requests := []request{
		{fiber.MethodGet, "/item", "en"},
		{fiber.MethodGet, "/item", "de"},
		{fiber.MethodGet, "/item?page=2", ""},
		{fiber.MethodHead, "/item", ""},
		{fiber.MethodGet, "/ITEM", ""},
		{fiber.MethodGet, elsewhere, ""},
	}
	for _, r := range requests {
		require.Equal(t, cacheMiss, send(r).cache, "%v", r)
		require.Equal(t, cacheHit, send(r).cache, "%v", r)
	}

	// The write names a query of its own, which the path does not hold apart.
	require.Equal(t, fiber.StatusNoContent, send(request{fiber.MethodPut, "/item?dry-run=1", ""}).status)

	for _, r := range requests {
		want := cacheMiss
		if r.target == elsewhere {
			want = cacheHit
		}
		require.Equal(t, want, send(r).cache, "%v", r)
	}
}

func Test_Cache_Security_WriteEndsThePathWithoutItsTrailingSlashesAndTheHostWithoutItsPort(t *testing.T) {
	t.Parallel()
	const host = "example.com"
	for _, tc := range []struct{ write, writeHost, read, readHost string }{
		{write: "/item/", writeHost: host, read: "/item", readHost: host},
		{write: "/item", writeHost: host, read: "/item/", readHost: host},
		{write: "/item//", writeHost: host, read: "/item", readHost: host},
		{write: "/item", writeHost: host + ":80", read: "/item", readHost: host},
		{write: "/item", writeHost: host, read: "/item", readHost: host + ":443"},
		{write: "/item", writeHost: "[::1]:8080", read: "/item", readHost: "[::1]"},
	} {
		t.Run(tc.writeHost+tc.write+" ends "+tc.readHost+tc.read, func(t *testing.T) {
			t.Parallel()
			app := cacheTestPItemApp(&Config{Expiration: time.Hour}, fiber.StatusNoContent)
			get := func() reply {
				return doRequest(t, app, apptest.Req(fiber.MethodGet, tc.read, apptest.WithHost(tc.readHost)))
			}
			cacheTestCRequireReply(t, get(), cacheMiss, "item v1")
			cacheTestCRequireReply(t, get(), cacheHit, "item v1")
			apptest.Expect(t, app, apptest.Req(fiber.MethodPut, tc.write, apptest.WithHost(tc.writeHost)), fiber.StatusNoContent)
			cacheTestCRequireReply(t, get(), cacheMiss, "item v2")
		})
	}
}

func Test_Cache_Security_WriteEndsAnEntryTheRequestBeganBefore(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		step time.Duration // of the wall clock while the GET is parked
		own  bool          // the write table keeps a monotonic clock of its own
	}{
		{name: "on the default table"},
		{name: "wall clock set back", step: -2 * time.Second, own: true},
		{name: "wall clock set forward", step: time.Hour, own: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wall := cacheTestCClock()
			monotonic, cfg := wall, Config{Expiration: time.Hour, clock: wall.Now}
			if tc.own {
				monotonic = cacheTestCClock()
				cfg.purges = newPurgeTable(monotonic.Now)
			}
			parked, release := make(chan struct{}), make(chan struct{})
			var slow atomic.Bool
			slow.Store(true)
			app := cacheTestPItemApp(&cfg, fiber.StatusNoContent, func() {
				if slow.CompareAndSwap(true, false) {
					close(parked)
					<-release
				}
			})

			reading := cacheTestBPark(t, app, apptest.Req(fiber.MethodGet, "/item"), parked, release)
			wall.Add(tc.step)
			monotonic.Add(time.Second)
			apptest.Expect(t, app, apptest.Req(fiber.MethodPut, "/item"), fiber.StatusNoContent)
			monotonic.Add(time.Second)
			require.Equal(t, cacheMiss, reading.finish(t).Header.Get("X-Cache"))

			monotonic.Add(time.Second)
			cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheMiss, "item v2")
			monotonic.Add(time.Second)
			cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheHit, "item v2")
		})
	}
}

// An external Storage keeps an entry's stamp; any write ends an entry without one, from an earlier version.
func Test_Cache_Security_WriteEndsEntriesOfAStorage(t *testing.T) {
	t.Parallel()
	for _, legacy := range []bool{false, true} {
		t.Run("legacy="+strconv.FormatBool(legacy), func(t *testing.T) {
			t.Parallel()
			storage := storagetest.New()
			clock := cacheTestCClock() // moved past the write, as in Test_Cache_Security_SuccessfulUnsafeRequestEndsTheEntry
			app := cacheTestPItemApp(&Config{Storage: storage, Expiration: time.Hour, clock: clock.Now}, fiber.StatusNoContent)

			key := defaultKey("/item")
			if legacy {
				raw, err := cacheTestALegacyItem("item v1").MarshalMsg(nil)
				require.NoError(t, err)
				storage.Data[key] = raw
				storage.Data[bodyKey(key)] = append(make([]byte, bodyStampLen), "item v1"...) // stamped with its generation, 0
			} else {
				cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheMiss, "item v1")
			}
			cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheHit, "item v1")

			apptest.Expect(t, app, apptest.Req(fiber.MethodDelete, "/item"), fiber.StatusNoContent)
			clock.Add(time.Second)
			cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheMiss, "item v2")
			cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheHit, "item v2")
		})
	}
}

func Test_Cache_Security_TimedOutWriteEndsTheEntryUnread(t *testing.T) {
	t.Parallel()
	finished := make(chan struct{})
	var version atomic.Int32
	version.Store(1)
	clock := cacheTestCClock() // moved past the write, as in Test_Cache_Security_SuccessfulUnsafeRequestEndsTheEntry
	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour, clock: clock.Now}))
	app.Get("/item", func(c fiber.Ctx) error { return c.SendString("item v" + strconv.Itoa(int(version.Load()))) })
	app.Put("/item", timeout.New(func(c fiber.Ctx) error {
		defer close(finished)
		<-c.Context().Done()
		version.Add(1)
		return c.SendStatus(fiber.StatusNoContent)
	}, timeout.Config{Timeout: 10 * time.Millisecond}))
	cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheMiss, "item v1")
	cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheHit, "item v1")

	fctx := cacheTestServe(func(fctx *fasthttp.RequestCtx) {
		fctx.Request.Header.SetMethod(fiber.MethodPut)
		app.Handler()(fctx)
	}, "/item")
	resp := fctx.LastTimeoutErrorResponse()
	require.NotNil(t, resp, "timeout.New did not give up on the handler")
	require.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode())
	apptest.Recv(t, finished, 5*time.Second, "the timed-out handler did not finish")
	clock.Add(time.Second)
	cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheMiss, "item v2")
}

func Test_Cache_Security_EndedEntryIsNotServedStale(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		directive, cache, body string
		status                 int
	}{
		{directive: "only-if-cached", cache: cacheUnreachable, status: fiber.StatusGatewayTimeout, body: "Gateway Timeout"},
		{directive: "only-if-cached, max-stale", cache: cacheUnreachable, status: fiber.StatusGatewayTimeout, body: "Gateway Timeout"},
		{directive: "max-stale", cache: cacheMiss, status: fiber.StatusOK, body: "item v2"},
	} {
		t.Run(tc.directive, func(t *testing.T) {
			t.Parallel()
			app := cacheTestPItemApp(&Config{Expiration: time.Hour}, fiber.StatusNoContent)
			cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheMiss, "item v1")
			apptest.Expect(t, app, apptest.Req(fiber.MethodPut, "/item"), fiber.StatusNoContent)
			got := doRequest(t, app, cacheTestAGetCC("/item", tc.directive))
			require.Equal(t, tc.status, got.status)
			cacheTestCRequireReply(t, got, tc.cache, tc.body)
		})
	}
}

// cacheTestPKeptPath returns a path as long as /item whose purge slot is not that of /item, which the
// table chooses by a hash of its own, so a write to /item cannot end its entries.
func cacheTestPKeptPath(purges *purgeTable) string {
	for i := 100; ; i++ {
		if path := "/k" + strconv.Itoa(i); purges.slot("example.com", path) != purges.slot("example.com", "/item") {
			return path
		}
	}
}

// cacheTestPEvictionApp serves any path through New(*cfg), its entries keyed by URL: a GET answers "x"
// after whileReading, a PUT 204.
func cacheTestPEvictionApp(cfg *Config, whileReading func(fiber.Ctx)) *fiber.App {
	keyed := *cfg
	keyed.KeyGenerator = func(c fiber.Ctx) string { return c.OriginalURL() }
	app := fiber.New()
	app.Use(New(keyed))
	app.All("/*", func(c fiber.Ctx) error {
		if c.Method() == fiber.MethodPut {
			return c.SendStatus(fiber.StatusNoContent)
		}
		if whileReading != nil {
			whileReading(c)
		}
		return c.SendString("x")
	})
	return app
}

// An entry a write ended makes room first: no request can be served it, while an older live entry can,
// and so can an expired one with max-stale.
func Test_Cache_WriteEndedEntryIsEvictedFirst(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		stale  bool // the kept entry has expired when room is needed, so only max-stale reaches it
		parked bool // the ended entry is stored by a request that began before the write
	}{
		{name: "before an older live entry"},
		{name: "before an expired entry max-stale can use", stale: true},
		{name: "stored by a request that began before the write", parked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clock := cacheTestCClock()
			purges := newPurgeTable(clock.Now)
			kept := cacheTestPKeptPath(purges) + "?v=0"
			lifetime := func(c fiber.Ctx, _ *Config) time.Duration {
				if tc.stale && c.OriginalURL() == kept {
					return 10 * time.Second
				}
				return time.Hour
			}
			parked, release := make(chan struct{}), make(chan struct{})
			var parking atomic.Bool
			parking.Store(tc.parked)
			app := cacheTestPEvictionApp(&Config{
				MaxBytes: 2 * charged(cacheKeyVersion+"|GET|"+kept, len("x"), textPlain), Expiration: time.Hour,
				ExpirationGenerator: lifetime, clock: clock.Now, purges: purges,
			}, func(c fiber.Ctx) {
				if c.Path() == "/item" && parking.CompareAndSwap(true, false) {
					close(parked)
					<-release
				}
			})

			cacheTestAExpect(t, app, kept, cacheMiss)
			clock.Add(time.Second)
			if tc.parked {
				reading := cacheTestBPark(t, app, apptest.Req(fiber.MethodGet, "/item?v=0"), parked, release)
				clock.Add(time.Second)
				apptest.Expect(t, app, apptest.Req(fiber.MethodPut, "/item"), fiber.StatusNoContent)
				clock.Add(time.Second)
				require.Equal(t, cacheMiss, reading.finish(t).Header.Get("X-Cache"))
			} else {
				cacheTestAExpect(t, app, "/item?v=0", cacheMiss)
				clock.Add(time.Second)
				apptest.Expect(t, app, apptest.Req(fiber.MethodPut, "/item"), fiber.StatusNoContent)
			}
			clock.Add(time.Minute)

			// Room for one more: the ended entry goes, not the one stored before it.
			cacheTestAExpect(t, app, "/next?v=0", cacheMiss)
			if tc.stale {
				cacheTestAExpect(t, app, cacheTestAGetCC(kept, "max-stale"), cacheHit)
			} else {
				cacheTestAExpect(t, app, kept, cacheHit)
			}
		})
	}
}

// A failed eviction puts the ended entry it took back as ended, so the next one makes room with it again.
func Test_CacheStorage_FailedEvictionPutsBackAWriteEndedEntryAsEnded(t *testing.T) {
	t.Parallel()
	clock := cacheTestCClock()
	purges := newPurgeTable(clock.Now)
	kept, item := cacheTestPKeptPath(purges)+"?v=0", cacheKeyVersion+"|GET|/item?v=0"
	storage := storagetest.New()
	app := cacheTestPEvictionApp(&Config{
		MaxBytes: 2 * charged(item, len("x"), textPlain), Storage: storage, Expiration: time.Hour, clock: clock.Now, purges: purges,
	}, nil)

	cacheTestAExpect(t, app, kept, cacheMiss)
	clock.Add(time.Second)
	cacheTestAExpect(t, app, "/item?v=0", cacheMiss)
	clock.Add(time.Second)
	apptest.Expect(t, app, apptest.Req(fiber.MethodPut, "/item"), fiber.StatusNoContent)
	clock.Add(time.Second)

	storage.SetErr(storagetest.OpDelete, item, errors.New("eviction delete failed"))
	cacheTestAExpect(t, app, "/next?v=0", cacheUnreachable)
	storage.SetErr(storagetest.OpDelete, item, nil)
	cacheTestAExpect(t, app, "/next?v=0", cacheMiss)
	cacheTestAExpect(t, app, kept, cacheHit)
	require.False(t, storage.Holds(item))
}

func Test_unsafeMethod(t *testing.T) {
	t.Parallel()
	for method, want := range map[string]bool{
		fiber.MethodGet: false, fiber.MethodHead: false, fiber.MethodOptions: false, fiber.MethodTrace: false, fiber.MethodQuery: false,
		fiber.MethodPost: true, fiber.MethodPut: true, fiber.MethodPatch: true, fiber.MethodDelete: true, fiber.MethodConnect: true,
		"PROPFIND": true, "": true,
	} {
		require.Equal(t, want, unsafeMethod(method), method)
	}
}

func Test_purgeTable(t *testing.T) {
	t.Parallel()
	table := newPurgeTable(time.Now)
	stored := func(at uint64) *item { return &item{stored: at} }

	t.Run("nothing is superseded before the first write", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, table.slots.Load())
		require.False(t, table.supersedes("example.com", "/item", stored(0)))
	})

	t.Run("the slot ignores case", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, table.slot("example.com", "/item"), table.slot("EXAMPLE.com", "/Item"))
	})

	t.Run("the slot ignores trailing slashes", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, table.slot("example.com", "/item"), table.slot("example.com", "/item//"))
		require.Equal(t, table.slot("example.com", "/"), table.slot("example.com", "//"))
	})

	t.Run("a write supersedes the entries stored before it, and the ones it cannot date", func(t *testing.T) {
		t.Parallel()
		own := newPurgeTable(time.Now)
		own.mark(own.slot("example.com", "/item"), 100)
		require.True(t, own.supersedes("example.com", "/item", stored(99)))
		require.True(t, own.supersedes("example.com", "/item", stored(100)), "stored in the instant of the write")
		require.True(t, own.supersedes("example.com", "/Item", stored(0)), "an entry with no time")
		require.False(t, own.supersedes("example.com", "/item", stored(101)))
	})

	t.Run("the clock counts from the one reading it starts at, and never below it", func(t *testing.T) {
		t.Parallel()
		clock := cacheTestCClock()
		own := newPurgeTable(clock.Now)
		start := own.stamp()
		require.Equal(t, unixNanos(time.Unix(1_700_000_000, 0)), start)
		clock.Add(time.Second)
		require.Equal(t, start+uint64(time.Second), own.stamp())
		clock.Add(-time.Hour) // a clock without a monotonic reading, set back past the start
		require.Equal(t, start, own.stamp())
	})

	t.Run("a slot only goes forward", func(t *testing.T) {
		t.Parallel()
		own := newPurgeTable(time.Now)
		slot := own.slot("example.com", "/item")
		own.mark(slot, 200)
		own.mark(slot, 100)
		require.True(t, own.supersedes("example.com", "/item", stored(200)))
		require.False(t, own.supersedes("example.com", "/item", stored(201)))
	})
}

// Sequential: testing.AllocsPerRun panics in a parallel test.
func Test_purgeTable_slotDoesNotAllocate(t *testing.T) {
	table := newPurgeTable(time.Now)
	long := "/" + strings.Repeat("Segment/", 40) // longer than the buffer it is folded in
	for _, target := range [][2]string{
		{"example.com", "/item"}, {"Example.COM", "/Item/"}, {"example.com", long}, {strings.Repeat("a", 200) + ".com", "/item"},
	} {
		require.Zero(t, testing.AllocsPerRun(100, func() { table.slot(target[0], target[1]) }), "%v", target)
	}
	// Folded through the buffer, a long path keeps the slot it has in one piece and in another case.
	require.Equal(t, table.slot("example.com", long), table.slot("EXAMPLE.com", strings.ToLower(long)+"/"))
}
