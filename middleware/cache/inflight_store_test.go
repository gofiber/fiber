package cache

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/storagetest"
	"github.com/stretchr/testify/require"
)

type cacheTestIGet func(target string, headers ...string) (int, error)

// cacheTestIStoreRace serves GET /* (100 bytes, room for one) over storage, running during at its first write.
func cacheTestIStoreRace(storage *storagetest.Memory, probe *accountingProbe, during func(get cacheTestIGet)) cacheTestIGet {
	app := fiber.New()
	get := func(target string, headers ...string) (int, error) {
		resp, err := app.Test(cacheTestCReq(target, headers...))
		if err != nil {
			return 0, err
		}
		return resp.StatusCode, resp.Body.Close()
	}
	var entered atomic.Bool
	hooked := &hookedStorage{Memory: storage, onSet: func(string, time.Duration) error {
		if entered.CompareAndSwap(false, true) {
			during(get)
		}
		return nil
	}}
	app.Use(New(Config{Expiration: time.Hour, MaxBytes: charged(defaultKey("/second"), 100, textPlain), Storage: hooked, accounting: probe.record}))
	app.Get("/*", func(c fiber.Ctx) error { return c.Send(make([]byte, 100)) })
	return get
}

func bodiesOfSize(storage *storagetest.Memory, size int) int {
	storage.Mu.RLock()
	defer storage.Mu.RUnlock()
	held := 0
	for _, value := range storage.Data {
		if len(value) == size {
			held++
		}
	}
	return held
}

// The heap node is inserted before the entry is written. A store that evicts
// the node in between deletes a key that is not there yet, so the entry written
// afterwards must not stay behind untracked.
func Test_Cache_MaxBytes_EntryEvictedWhileStoredIsNotKept(t *testing.T) {
	t.Parallel()
	storage, probe := storagetest.New(), &accountingProbe{}
	var (
		secondStatus int
		errSecond    error
	)
	get := cacheTestIStoreRace(storage, probe, func(get cacheTestIGet) { secondStatus, errSecond = get("/second") })
	status, err := get("/first")
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, status)
	require.NoError(t, errSecond)
	require.Equal(t, fiber.StatusOK, secondStatus)

	status, err = get("/second", fiber.HeaderCacheControl, "only-if-cached")
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, status)
	status, err = get("/first", fiber.HeaderCacheControl, "only-if-cached")
	require.NoError(t, err)
	require.Equal(t, fiber.StatusGatewayTimeout, status)
	require.Equal(t, 1, probe.nodes())
	require.Equal(t, charged(defaultKey("/second"), 100, textPlain), probe.counted())
	require.Equal(t, 1, bodiesOfSize(storage, 100))
}

// When the entry cannot be taken back out, it has to be tracked again so that a
// later eviction reaches it.
func Test_Cache_MaxBytes_EntryEvictedWhileStoredIsTrackedWhenDeleteFails(t *testing.T) {
	t.Parallel()
	for _, failing := range [][]string{{storagetest.OpDelete}, {storagetest.OpDelete, storagetest.OpGet}} {
		t.Run(strings.Join(failing, " and ")+" fail", func(t *testing.T) {
			t.Parallel()
			storage, probe := storagetest.New(), &accountingProbe{}
			var (
				secondStatus int
				errSecond    error
			)
			get := cacheTestIStoreRace(storage, probe, func(get cacheTestIGet) {
				secondStatus, errSecond = get("/second")
				for _, op := range failing {
					storage.SetErr(op, "*", errors.New("storage unavailable"))
				}
			})
			status, err := get("/first")
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, status)
			require.NoError(t, errSecond)
			require.Equal(t, fiber.StatusOK, secondStatus)
			require.Equal(t, 2, probe.nodes())
			require.Equal(t, charged(defaultKey("/first"), 100, textPlain)+charged(defaultKey("/second"), 100, textPlain), probe.counted())

			storage.ClearErrs()
			status, err = get("/third")
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, status)
			require.Equal(t, 1, probe.nodes())
			require.Equal(t, charged(defaultKey("/third"), 100, textPlain), probe.counted())
			require.Equal(t, 1, bodiesOfSize(storage, 100))
		})
	}
}

func Test_Cache_MaxBytes_EntryTakenBackOutIsNotRetracked(t *testing.T) {
	t.Parallel()
	key := func(target string) string { return cacheKeyVersion + "|GET|" + target }
	gate := newCacheTestGate(true)
	evicted := make(chan error, 1)
	app := fiber.New()
	var entered atomic.Bool
	storage := &hookedStorage{Memory: storagetest.New(), onDelete: func(k string) error {
		if k == key("/first/100") && gate.park() {
			return errors.New("storage unavailable")
		}
		return nil
	}}
	// /evict pops /first and /inflight, then parks deleting /first while /inflight is written.
	storage.onSet = func(k string, _ time.Duration) error {
		if k == bodyKey(key("/inflight/100")) && entered.CompareAndSwap(false, true) {
			go func() {
				resp, err := app.Test(apptest.Req(fiber.MethodGet, "/evict/150"))
				if err == nil {
					err = resp.Body.Close()
				}
				evicted <- err
			}()
			<-gate.entered
		}
		return nil
	}
	probe := &accountingProbe{}
	maxBytes := charged(key("/first/100"), 100, textPlain) + charged(key("/inflight/100"), 100, textPlain)
	app.Use(New(Config{
		ExpirationGenerator: stableAscendingExpiration(),
		KeyGenerator:        pathKey,
		MaxBytes:            maxBytes,
		Storage:             storage,
		accounting:          probe.record,
	}))
	app.Get("/:name/:size", func(c fiber.Ctx) error { return c.Send(make([]byte, fiber.Params[int](c, "size"))) })

	require.Equal(t, cacheMiss, apptest.Expect(t, app, "/first/100", fiber.StatusOK).Header.Get("X-Cache"))
	apptest.Expect(t, app, "/inflight/100", fiber.StatusOK)
	close(gate.release)
	require.NoError(t, apptest.Recv(t, evicted, 5*time.Second, "the evicting request did not finish"))

	require.Equal(t, 1, probe.nodes(), "only /first, whose delete failed, is tracked again")
	require.Equal(t, charged(key("/first/100"), 100, textPlain), probe.counted())
}

// The in-memory store offers no seam between the two steps, so this one races
// for real: distinct keys, every store evicting, then a count of what is held.
func Test_Cache_MaxBytes_ConcurrentStoresStayWithinLimit(t *testing.T) {
	t.Parallel()
	const (
		bodySize = 1024
		maxBytes = 8 * bodySize
		workers  = 16
		perWork  = 4000
	)

	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour, MaxBytes: maxBytes}))
	app.Get("/*", func(c fiber.Ctx) error { return c.Send(make([]byte, bodySize)) })
	handler := app.Handler()

	get := func(target, cacheControl string) int {
		return cacheTestServe(handler, target, fiber.HeaderCacheControl, cacheControl).Response.StatusCode()
	}

	var wg sync.WaitGroup
	for worker := range workers {
		wg.Go(func() {
			for i := range perWork {
				get(fmt.Sprintf("/%d/%d", worker, i), "")
			}
		})
	}
	wg.Wait()

	held := 0
	for worker := range workers {
		for i := range perWork {
			if get(fmt.Sprintf("/%d/%d", worker, i), "only-if-cached") == fiber.StatusOK {
				held += bodySize
			}
		}
	}
	require.LessOrEqual(t, held, maxBytes)
}

// What exceeds MaxBytes once every node is popped is reserved by stores still in
// flight: the popped entries stay stored and counted, and the response is served
// without being stored.
func Test_Cache_MaxBytes_NoSpaceKeepsPoppedEntriesTracked(t *testing.T) {
	t.Parallel()
	gate := newCacheTestGate(true)
	storage := cacheTestParkedDelete(cacheKeyVersion+"|GET|/first/200", gate, nil)
	charge := func(target string, size int) uint {
		return charged(cacheKeyVersion+"|GET|"+target, size, textPlain)
	}
	// Room for /second, /third and /inflight, so /inflight evicts /first.
	maxBytes := charge("/second/200", 200) + charge("/third/200", 200) + charge("/inflight/600", 600)
	probe := &accountingProbe{}
	app := fiber.New()
	app.Use(New(Config{
		ExpirationGenerator: stableAscendingExpiration(),
		KeyGenerator:        pathKey,
		MaxBytes:            maxBytes,
		Storage:             storage,
		accounting:          probe.record,
	}))
	app.Get("/:name/:size", func(c fiber.Ctx) error { return c.Send(make([]byte, fiber.Params[int](c, "size"))) })

	for _, target := range []string{"/first/200", "/second/200", "/third/200"} {
		resp := apptest.Do(t, app, target)
		require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
		require.NoError(t, resp.Body.Close())
	}

	// Reserves its room and stops while it deletes the entry it evicted.
	inFlightDone := make(chan struct{})
	var inFlightResp *http.Response
	var inFlightErr error
	go func() {
		inFlightResp, inFlightErr = app.Test(httptest.NewRequest(fiber.MethodGet, "/inflight/600", http.NoBody))
		close(inFlightDone)
	}()
	<-gate.entered

	released := false
	defer func() {
		if !released {
			close(gate.release)
		}
	}()

	// /inflight's reservation and /nospace do not fit, whatever is evicted.
	resp := apptest.Expect(t, app, "/nospace/600", fiber.StatusOK)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	require.NoError(t, resp.Body.Close())
	require.Equal(t, maxBytes, probe.counted(), "the entries that stay stored must stay counted")
	require.Equal(t, 2, probe.nodes(), "the entries that stay stored must stay evictable")

	// A refresh still finds the node its entry recorded and takes over its share.
	refresh := httptest.NewRequest(fiber.MethodGet, "/third/200", http.NoBody)
	refresh.Header.Set(fiber.HeaderCacheControl, "no-cache")
	resp = apptest.Do(t, app, refresh)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.NoError(t, resp.Body.Close())
	require.Equal(t, maxBytes, probe.counted())
	require.Equal(t, 2, probe.nodes())

	close(gate.release)
	released = true
	<-inFlightDone
	require.NoError(t, inFlightErr)
	require.Equal(t, cacheMiss, inFlightResp.Header.Get("X-Cache"))
	require.NoError(t, inFlightResp.Body.Close())

	// Needs the room of both entries and of the one that was in flight.
	resp = apptest.Do(t, app, "/later/1000")
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.NoError(t, resp.Body.Close())
	require.Equal(t, charge("/later/1000", 1000), probe.counted())
	require.Equal(t, 1, probe.nodes())

	for _, target := range []string{"/second/200", "/third/200", "/inflight/600"} {
		req := httptest.NewRequest(fiber.MethodGet, target, http.NoBody)
		req.Header.Set(fiber.HeaderCacheControl, "only-if-cached")
		resp = apptest.Do(t, app, req)
		require.Equal(t, fiber.StatusGatewayTimeout, resp.StatusCode, "%s must have been evicted", target)
		require.NoError(t, resp.Body.Close())
	}
}
