package cache

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/storagetest"
	"github.com/stretchr/testify/require"
)

type blockingManifestStorage struct {
	*storagetest.Memory
	blocked      chan struct{}
	release      chan struct{}
	manifestSets atomic.Int32
	failBodySet  atomic.Bool
}

func newBlockingManifestStorage() *blockingManifestStorage {
	return &blockingManifestStorage{
		Memory:  storagetest.New(),
		blocked: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (s *blockingManifestStorage) SetWithContext(ctx context.Context, key string, value []byte, exp time.Duration) error {
	if strings.HasPrefix(key, bodyKeyPrefix) && s.failBodySet.Load() {
		return errors.New("body write failed")
	}
	if strings.HasPrefix(key, manifestKeyPrefix) && s.manifestSets.Add(1) == 2 {
		close(s.blocked)
		<-s.release
	}
	return s.Memory.SetWithContext(ctx, key, value, exp)
}

func (s *blockingManifestStorage) Set(key string, value []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, value, exp)
}

// cacheTestCReloads is the interleaving the concurrent-reload tests share: a
// 100-byte answer is cached at GET /entry, and two reloads of it are parked in
// the origin, the first and then the second.
type cacheTestCReloads struct {
	app    *fiber.App
	probe  *accountingProbe
	calls  *atomic.Int32
	first  *cacheTestBParked
	second *cacheTestBParked
	varyOn string
}

// cacheTestCStartReloads caches /entry behind storage, the in-memory store when
// that is nil, and parks two reloads of it with the origin's second and third
// calls. Unless varyOn is empty, the origin varies on the request header of that
// name, which every request sends.
func cacheTestCStartReloads(t *testing.T, storage fiber.Storage, varyOn string) *cacheTestCReloads {
	t.Helper()

	probe := &accountingProbe{}
	calls := new(atomic.Int32)
	firstEntered := make(chan struct{})
	secondEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})

	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour, MaxBytes: 1000, Storage: storage, accounting: probe.record}))
	app.Get("/entry", func(c fiber.Ctx) error {
		switch calls.Add(1) {
		case 2:
			close(firstEntered)
			<-releaseFirst
		case 3:
			close(secondEntered)
			<-releaseSecond
		}
		if varyOn != "" {
			c.Set(fiber.HeaderVary, varyOn)
		}
		return c.Send(make([]byte, 100))
	})

	r := &cacheTestCReloads{app: app, probe: probe, calls: calls, varyOn: varyOn}
	cacheTestAExpect(t, app, r.request(""), cacheMiss)
	r.first = cacheTestBPark(t, app, r.request("max-age=0"), firstEntered, releaseFirst)
	r.second = cacheTestBPark(t, app, r.request("max-age=0"), secondEntered, releaseSecond)
	return r
}

// request returns GET /entry with the Cache-Control request header cacheControl,
// unless that is empty.
func (r *cacheTestCReloads) request(cacheControl string) *http.Request {
	var opts []apptest.Option
	if r.varyOn != "" {
		opts = append(opts, apptest.WithHeader(r.varyOn, "en"))
	}
	if cacheControl != "" {
		opts = append(opts, apptest.WithHeader(fiber.HeaderCacheControl, cacheControl))
	}
	return apptest.Req(fiber.MethodGet, "/entry", opts...)
}

// settledCharge is what MaxBytes counts for the one entry the reloads leave: the 100 bytes, their key and the
// Content-Type. Every request sends Accept-Language: en, so an origin that varies on it stores the entry under the
// variant's key, with the Vary list.
func (r *cacheTestCReloads) settledCharge() uint {
	key := defaultKey("/entry")
	if r.varyOn == "" {
		return charged(key, 100, textPlain)
	}
	key = strings.Replace(key, "accept-language:0", "accept-language:1|en", 1)
	return charged(variantKey(key, "accept-language"), 100, textPlain, r.varyOn)
}

// requireSettled requires what every interleaving ends with: the entry counted
// once, in one node of the eviction heap, and a hit on /entry, the origin
// having been reached three times only.
func (r *cacheTestCReloads) requireSettled(t *testing.T) {
	t.Helper()

	cacheTestBRequireAccounting(t, r.probe, r.settledCharge(), 1)

	cacheTestAExpect(t, r.app, r.request(""), cacheHit)
	require.Equal(t, int32(3), r.calls.Load())
}

// Both reloads read the same cached generation before entering the origin.
// The second store must replace the first reload's heap node even though the
// heap generation it originally read is no longer current.
func Test_Cache_ConcurrentReloadsReplaceSameKeyHeapNode(t *testing.T) {
	t.Parallel()
	r := cacheTestCStartReloads(t, nil, "")

	r.first.finish(t)
	r.second.finish(t)

	r.requireSettled(t)
}

// A request can reserve space, wait on storage, and then observe another
// request's completed store. The final heap insertion must replace that node.
func Test_Cache_ConcurrentReloadAfterReservationReplacesHeapNode(t *testing.T) {
	t.Parallel()
	storage := newBlockingManifestStorage()
	r := cacheTestCStartReloads(t, storage, fiber.HeaderAcceptLanguage)

	// Store the second reload first, but pause it after reservation and
	// before heap insertion while the first reload finishes its store.
	r.second.open()
	<-storage.blocked
	r.first.finish(t)
	close(storage.release)
	r.second.finish(t)

	r.requireSettled(t)
}

func Test_Cache_ConcurrentReloadStoreFailureRestoresLatestHeapNode(t *testing.T) {
	t.Parallel()
	storage := newBlockingManifestStorage()
	r := cacheTestCStartReloads(t, storage, fiber.HeaderAcceptLanguage)

	r.second.open()
	<-storage.blocked
	r.first.finish(t)
	storage.failBodySet.Store(true)
	close(storage.release)
	failed := r.second.finish(t)
	require.Equal(t, fiber.StatusInternalServerError, failed.StatusCode)
	storage.failBodySet.Store(false)

	r.requireSettled(t)
}
