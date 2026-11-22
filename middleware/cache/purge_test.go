package cache

import (
	"strconv"
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

// cacheTestPItem serves /item's version to GET and HEAD, and bumps it for any other method, answering status.
func cacheTestPItem(version *atomic.Int32, status int) fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead {
			return c.SendString("item v" + strconv.Itoa(int(version.Load())))
		}
		version.Add(1)
		return c.SendStatus(status)
	}
}

// Test_Cache_Security_SuccessfulUnsafeRequestEndsTheEntry tests that an unsafe request that succeeds ends
// the response stored for its path (RFC 9111 §4.4).
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
			var version atomic.Int32
			version.Store(1)
			// A test clock moved past the write, as a coarse wall clock (Windows) puts both in one tick.
			clock := cacheTestCClock()
			cfg := tc.cfg
			cfg.Expiration = time.Hour
			cfg.clock = clock.Now
			app := fiber.New()
			app.Use(New(cfg))
			app.All("/item", cacheTestPItem(&version, tc.status))

			get := func() reply { return cacheTestCGet(t, app, "/item") }
			cacheTestCRequireReply(t, get(), cacheMiss, "item v1")
			cacheTestCRequireReply(t, get(), cacheHit, "item v1")

			write := doRequest(t, app, apptest.Req(tc.method, "/item", apptest.WithHeaderIf(tc.header[0], tc.header[1])))
			require.Equal(t, tc.status, write.status)
			clock.Add(time.Second)

			if tc.ends {
				cacheTestCRequireReply(t, get(), cacheMiss, "item v2")
				cacheTestCRequireReply(t, get(), cacheHit, "item v2")
			} else {
				// Not ended, though the handler bumped the version.
				cacheTestCRequireReply(t, get(), cacheHit, "item v1")
			}
		})
	}
}

// A write ends every entry of its path, whatever its query, method or Vary variant; other paths keep theirs.
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

// A write ends the entries of every spelling the router takes for its path: by default, case and trailing slashes.
func Test_Cache_Security_WriteEndsThePathWithoutItsTrailingSlashes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ write, read string }{
		{write: "/item/", read: "/item"},
		{write: "/item", read: "/item/"},
		{write: "/item//", read: "/item"},
	} {
		t.Run(tc.write+" ends "+tc.read, func(t *testing.T) {
			t.Parallel()
			var version atomic.Int32
			version.Store(1)
			app := fiber.New()
			app.Use(New(Config{Expiration: time.Hour}))
			app.All("/item", cacheTestPItem(&version, fiber.StatusNoContent))

			cacheTestCRequireReply(t, cacheTestCGet(t, app, tc.read), cacheMiss, "item v1")
			cacheTestCRequireReply(t, cacheTestCGet(t, app, tc.read), cacheHit, "item v1")
			require.Equal(t, fiber.StatusNoContent, doRequest(t, app, apptest.Req(fiber.MethodPut, tc.write)).status)
			cacheTestCRequireReply(t, cacheTestCGet(t, app, tc.read), cacheMiss, "item v2")
		})
	}
}

// An entry stored by a request that began before the write is ended too; one that began after is not.
func Test_Cache_Security_WriteEndsAnEntryTheRequestBeganBefore(t *testing.T) {
	t.Parallel()
	clock := cacheTestCClock()
	var version atomic.Int32
	version.Store(1)
	parked, release := make(chan struct{}), make(chan struct{})
	var slow atomic.Bool
	slow.Store(true)
	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour, clock: clock.Now}))
	app.All("/item", func(c fiber.Ctx) error {
		if c.Method() == fiber.MethodPut {
			version.Add(1)
			return c.SendStatus(fiber.StatusNoContent)
		}
		read := version.Load()
		if slow.CompareAndSwap(true, false) {
			close(parked)
			<-release
		}
		return c.SendString("item v" + strconv.Itoa(int(read)))
	})

	// A GET reads the item, and the write succeeds before it answers.
	reading := cacheTestBPark(t, app, apptest.Req(fiber.MethodGet, "/item"), parked, release)
	clock.Add(time.Second)
	require.Equal(t, fiber.StatusNoContent, doRequest(t, app, apptest.Req(fiber.MethodPut, "/item")).status)
	clock.Add(time.Second)
	answered := reading.finish(t)
	require.Equal(t, cacheMiss, answered.Header.Get("X-Cache"))

	// What it stored is what the path held before the write: not served.
	clock.Add(time.Second)
	cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheMiss, "item v2")
	cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheHit, "item v2")
}

// Writes and entries are ordered by monotonic time, so wall-clock steps reorder neither.
func Test_Cache_Security_WriteOrderIgnoresWallClockSteps(t *testing.T) {
	t.Parallel()
	for _, step := range []time.Duration{-2 * time.Second, time.Hour} {
		t.Run(step.String(), func(t *testing.T) {
			t.Parallel()
			wall, monotonic := cacheTestCClock(), cacheTestCClock()
			var version atomic.Int32
			version.Store(1)
			parked, release := make(chan struct{}), make(chan struct{})
			var slow atomic.Bool
			slow.Store(true)
			app := fiber.New()
			app.Use(New(Config{Expiration: time.Hour, clock: wall.Now, purges: newPurgeTable(monotonic.Now)}))
			app.All("/item", func(c fiber.Ctx) error {
				if c.Method() == fiber.MethodPut {
					version.Add(1)
					return c.SendStatus(fiber.StatusNoContent)
				}
				read := version.Load()
				if slow.CompareAndSwap(true, false) {
					close(parked)
					<-release
				}
				return c.SendString("item v" + strconv.Itoa(int(read)))
			})

			// A GET reads the item, the wall clock steps, and the write succeeds before the GET answers.
			reading := cacheTestBPark(t, app, apptest.Req(fiber.MethodGet, "/item"), parked, release)
			wall.Add(step)
			monotonic.Add(time.Second)
			require.Equal(t, fiber.StatusNoContent, doRequest(t, app, apptest.Req(fiber.MethodPut, "/item")).status)
			monotonic.Add(time.Second)
			require.Equal(t, cacheMiss, reading.finish(t).Header.Get("X-Cache"))

			// What it stored predates the write, and the response stored after it is served.
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
			var version atomic.Int32
			version.Store(1)
			clock := cacheTestCClock() // moved past the write, as in Test_Cache_Security_SuccessfulUnsafeRequestEndsTheEntry
			app := fiber.New()
			app.Use(New(Config{Storage: storage, Expiration: time.Hour, clock: clock.Now}))
			app.All("/item", cacheTestPItem(&version, fiber.StatusNoContent))

			key := defaultKey("/item")
			if legacy {
				raw, err := cacheTestALegacyItem("item v1").MarshalMsg(nil)
				require.NoError(t, err)
				storage.Data[key] = raw
				storage.Data[bodyKey(key)] = []byte("item v1")
			} else {
				cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheMiss, "item v1")
			}
			cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheHit, "item v1")

			require.Equal(t, fiber.StatusNoContent, doRequest(t, app, apptest.Req(fiber.MethodDelete, "/item")).status)
			clock.Add(time.Second)
			cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheMiss, "item v2")
			cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheHit, "item v2")
		})
	}
}

// The cache does not read the status of a write timeout.New gave up on, as its handler may still be writing it.
func Test_Cache_Security_TimedOutWriteIsNotRead(t *testing.T) {
	t.Parallel()
	finished := make(chan struct{})
	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour}))
	app.Put("/item", timeout.New(func(c fiber.Ctx) error {
		defer close(finished)
		<-c.Context().Done()
		return c.SendStatus(fiber.StatusNoContent)
	}, timeout.Config{Timeout: 10 * time.Millisecond}))

	fctx := cacheTestServe(func(fctx *fasthttp.RequestCtx) {
		fctx.Request.Header.SetMethod(fiber.MethodPut)
		app.Handler()(fctx)
	}, "/item")
	resp := fctx.LastTimeoutErrorResponse()
	require.NotNil(t, resp, "timeout.New did not give up on the handler")
	require.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode())
	apptest.Recv(t, finished, 5*time.Second, "the timed-out handler did not finish")
}

// An entry a write ended is served neither stale nor to only-if-cached.
func Test_Cache_Security_EndedEntryIsNotServedStale(t *testing.T) {
	t.Parallel()
	var version atomic.Int32
	version.Store(1)
	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour}))
	app.All("/item", cacheTestPItem(&version, fiber.StatusNoContent))

	cacheTestCRequireReply(t, cacheTestCGet(t, app, "/item"), cacheMiss, "item v1")
	require.Equal(t, fiber.StatusNoContent, doRequest(t, app, apptest.Req(fiber.MethodPut, "/item")).status)

	for _, directive := range []string{"only-if-cached", "only-if-cached, max-stale"} {
		cacheTestAExpectStatus(t, app, cacheTestAGetCC("/item", directive), cacheUnreachable, fiber.StatusGatewayTimeout)
	}
	got := doRequest(t, app, apptest.Req(fiber.MethodGet, "/item", apptest.WithHeader(fiber.HeaderCacheControl, "max-stale")))
	cacheTestCRequireReply(t, got, cacheMiss, "item v2")
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

// Sequential: testing.AllocsPerRun panics in a parallel test. Folding case must not allocate on a hit.
func Test_purgeTable_slotDoesNotAllocate(t *testing.T) {
	table := newPurgeTable(time.Now)
	for _, target := range [][2]string{{"example.com", "/item"}, {"Example.COM", "/Item/"}} {
		require.Zero(t, testing.AllocsPerRun(100, func() { table.slot(target[0], target[1]) }), "%v", target)
	}
}
