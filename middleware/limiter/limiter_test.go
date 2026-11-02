package limiter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/clocktest"
	"github.com/gofiber/fiber/v3/internal/redact"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/internal/storagetest"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/gofiber/utils/v2"
)

const testLimiterClientKey = "client-key"

var windowStrategies = []struct {
	handler Handler
	name    string
}{
	{name: "fixed", handler: FixedWindow{}},
	{name: "sliding", handler: SlidingWindow{}},
}

// limiterTestMemoryStores are the limiter's own store and an explicit in-memory Storage.
var limiterTestMemoryStores = []struct {
	storage func() fiber.Storage
	name    string
}{
	{name: "default_store", storage: func() fiber.Storage { return nil }},
	{name: "custom_store", storage: func() fiber.Storage { return memory.New() }},
}

type typedNilLimiterError struct {
	message string
}

func (e *typedNilLimiterError) Error() string {
	return e.message
}

// countingFailStorage fails set operations, and optionally get operations,
// after a specified number of successful calls
type countingFailStorage struct {
	*storagetest.Memory
	setFailErr    error
	getFailErr    error
	setCount      int
	failAfterN    int
	getCount      int
	getFailAfterN int
}

func newCountingFailStorage(failAfterN int, err error) *countingFailStorage {
	return &countingFailStorage{Memory: storagetest.New(), failAfterN: failAfterN, setFailErr: err}
}

func (s *countingFailStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	s.getCount++
	if s.getFailErr != nil && s.getCount > s.getFailAfterN {
		return nil, s.getFailErr
	}
	return s.Memory.GetWithContext(ctx, key)
}

func (s *countingFailStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.setCount++
	if s.setCount > s.failAfterN {
		return s.setFailErr
	}
	return s.Memory.SetWithContext(ctx, key, val, exp)
}

func sleepForRetryAfter(t *testing.T, resp *http.Response) {
	t.Helper()

	retryAfter := resp.Header.Get(fiber.HeaderRetryAfter)
	if retryAfter == "" {
		time.Sleep(500 * time.Millisecond)
		return
	}

	seconds, err := strconv.Atoi(retryAfter)
	require.NoError(t, err)

	delay := time.Duration(seconds) * time.Second
	// Sliding window needs roughly 2x the reported delay for the previous window to expire.
	if doubled := 2 * delay; doubled > delay {
		delay = doubled
	}
	if minDelay := 4 * time.Second; delay < minDelay {
		delay = minDelay
	}

	// Window accounting reads the cached second clock, so wall time alone is not
	// enough: a late timestamp updater keeps the old window current.
	clocktest.SleepPast(t, delay)
}

// expiringStorage drops entries once their TTL has passed on the test clock.
type expiringStorage struct {
	*storagetest.Memory
	clock   *clocktest.Clock
	expires map[string]time.Time
}

func (s *expiringStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if exp, ok := s.expires[key]; ok && !s.clock.Now().Before(exp) {
		delete(s.Data, key)
		delete(s.expires, key)
	}
	return s.Memory.GetWithContext(ctx, key)
}

func (s *expiringStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.expires[key] = s.clock.Now().Add(exp)
	return s.Memory.SetWithContext(ctx, key, val, exp)
}

// limiterTestApp returns an app, configured by appCfg if given, that serves GET on every path with a 200
// behind the limiter New(cfg) builds.
func limiterTestApp(cfg *Config, appCfg ...fiber.Config) *fiber.App {
	app := fiber.New(appCfg...)
	app.Use(New(*cfg))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	return app
}

// limiterTestCapture keeps the error an app's ErrorHandler is handed, for a test that looks at what
// the limiter returned.
type limiterTestCapture struct {
	err error
}

// config is an app config whose ErrorHandler answers 500 and keeps the error in c.
func (c *limiterTestCapture) config() fiber.Config {
	return fiber.Config{
		ErrorHandler: func(ctx fiber.Ctx, err error) error {
			c.err = err
			return ctx.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	}
}

// limiterTestClockedApp is limiterTestApp on a clock the test moves by hand: it sets cfg.clock and
// returns the clock beside the app, so a window ends when the test says so rather than when the
// wall clock does.
func limiterTestClockedApp(cfg *Config) (*fiber.App, *clocktest.Clock) {
	clock := clocktest.NewClock(time.Now().Truncate(time.Second))
	cfg.clock = clock.Now
	return limiterTestApp(cfg), clock
}

// limiterTestRequestCtx returns a context that holds a GET /.
func limiterTestRequestCtx() *fasthttp.RequestCtx {
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")
	return fctx
}

// limiterTestHandler returns the fasthttp handler of limiterTestApp(cfg) and a GET / context it has
// served once.
func limiterTestHandler(cfg *Config) (fasthttp.RequestHandler, *fasthttp.RequestCtx) {
	h := limiterTestApp(cfg).Handler()
	fctx := limiterTestRequestCtx()

	h(fctx)
	return h, fctx
}

// limiterTestExhaust sends the limit requests the limiter of app admits, each answered 200 with the limit
// and what is left of it in the rate limit headers, then requires the next request to be answered 429.
func limiterTestExhaust(t *testing.T, app *fiber.App, limit int) {
	t.Helper()

	for i := 1; i <= limit; i++ {
		resp := apptest.Expect(t, app, "/", fiber.StatusOK)
		require.Equal(t, strconv.Itoa(limit), resp.Header.Get(xRateLimitLimit), "request %d", i)
		require.Equal(t, strconv.Itoa(limit-i), resp.Header.Get(xRateLimitRemaining), "request %d", i)
	}

	apptest.Expect(t, app, "/", fiber.StatusTooManyRequests)
}

// limiterTestAsync sends req through app on a goroutine of its own, as a request that is still in
// flight while the test sends others. The returned function waits for the response and fails the
// test if the app returned an error.
func limiterTestAsync(t *testing.T, app *fiber.App, req *http.Request, cfg ...fiber.TestConfig) func() *http.Response {
	t.Helper()

	type result struct {
		resp *http.Response
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := app.Test(req, cfg...)
		done <- result{resp: resp, err: err}
	}()

	return func() *http.Response {
		t.Helper()

		res := <-done
		require.NoError(t, res.err)
		return res.resp
	}
}

func Test_Limiter_DefaultConfigNoPanic(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	require.NotPanics(t, func() { apptest.Expect(t, app, "/", fiber.StatusOK) })
}

func Test_getEffectiveStatusCode_TimedOut(t *testing.T) {
	t.Parallel()
	// A test of Abandon and ForceRelease: apptest.Ctx would release the context a second time.
	c := fiber.New().AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(c.ForceRelease)

	c.Response().SetStatusCode(fiber.StatusOK)
	c.RequestCtx().TimeoutErrorWithCode("upstream too slow", fiber.StatusGatewayTimeout)
	c.Abandon()

	require.Equal(t, fiber.StatusGatewayTimeout, getEffectiveStatusCode(c, nil))
	require.Equal(t, fiber.StatusGatewayTimeout, getEffectiveStatusCode(c, errors.New("generic")))
}

// Test_Limiter_TimedOutRequest checks that the limiter leaves the response of a
// handler timeout.New gave up on alone; fasthttp sends the timeout response.
func Test_Limiter_TimedOutRequest(t *testing.T) {
	t.Parallel()
	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()
			release := make(chan struct{})
			finished := make(chan struct{})
			app := fiber.New()
			app.Use(New(Config{Max: 10, LimiterMiddleware: strategy.handler}))
			app.Get("/", timeout.New(func(c fiber.Ctx) error {
				defer close(finished)
				c.Set("X-Handler", "1")
				<-release
				return nil
			}, timeout.Config{Timeout: 10 * time.Millisecond}))

			fctx := limiterTestRequestCtx()
			app.Handler()(fctx)

			resp := fctx.LastTimeoutErrorResponse()
			require.NotNil(t, resp)
			require.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode())

			close(release)
			apptest.Recv(t, finished, 5*time.Second, "the timed-out handler never finished")
			require.Equal(t, "1", string(fctx.Response.Header.Peek("X-Handler")))
			require.Empty(t, fctx.Response.Header.Peek(xRateLimitLimit), "the limiter wrote to the timed-out handler's response")
		})
	}
}

// Test_getEffectiveStatusCode covers the status a request is counted with: the code of a *fiber.Error,
// else an error status the response already carries, else 500 for a plain error. A typed nil error is no
// error at all and must not panic.
func Test_getEffectiveStatusCode(t *testing.T) {
	t.Parallel()
	teapot := fiber.NewError(fiber.StatusTeapot, "teapot")
	generic := errors.New("generic")
	for _, tc := range []struct {
		err    error
		name   string
		status int // already set on the response
		want   int
	}{
		{name: "fiber error", status: fiber.StatusOK, err: teapot, want: fiber.StatusTeapot},
		{name: "plain error keeps an error status", status: fiber.StatusBadGateway, err: generic, want: fiber.StatusBadGateway},
		{name: "plain error after 200", status: fiber.StatusOK, err: generic, want: fiber.StatusInternalServerError},
		{name: "plain error after 302", status: fiber.StatusFound, err: generic, want: fiber.StatusInternalServerError},
		{name: "typed nil fiber error", status: fiber.StatusAccepted, err: (*fiber.Error)(nil), want: fiber.StatusAccepted},
		{name: "typed nil custom error", status: fiber.StatusAccepted, err: (*typedNilLimiterError)(nil), want: fiber.StatusAccepted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, fiber.New())
			c.Response().SetStatusCode(tc.status)

			require.NotPanics(t, func() { require.Equal(t, tc.want, getEffectiveStatusCode(c, tc.err)) })
		})
	}
}

func Test_Limiter_StorageErrorMessages(t *testing.T) {
	t.Parallel()
	steps := []struct {
		op   string
		want []string
	}{
		{op: storagetest.OpGet, want: []string{"limiter: failed to get key"}},
		{op: storagetest.OpSet, want: []string{"limiter: failed to persist state", "limiter: failed to store key"}},
	}

	for _, strategy := range windowStrategies {
		for _, step := range steps {
			for _, disableRedaction := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/DisableValueRedaction=%t", strategy.name, step.op, disableRedaction), func(t *testing.T) {
					t.Parallel()
					storage := storagetest.New()
					storage.SetErr(step.op, storagePrefix+testLimiterClientKey, errors.New("boom"))

					var captured limiterTestCapture
					app := limiterTestApp(&Config{
						DisableValueRedaction: disableRedaction,
						LimiterMiddleware:     strategy.handler,
						Storage:               storage,
						Max:                   1,
						Expiration:            time.Second,
						KeyGenerator:          func(fiber.Ctx) string { return testLimiterClientKey },
					}, captured.config())

					apptest.Expect(t, app, "/", fiber.StatusInternalServerError)
					require.Error(t, captured.err)
					for _, want := range step.want {
						require.ErrorContains(t, captured.err, want)
					}
					if disableRedaction {
						require.ErrorContains(t, captured.err, testLimiterClientKey)
						require.NotContains(t, captured.err.Error(), redact.Placeholder)
					} else {
						require.ErrorContains(t, captured.err, redact.Placeholder)
						require.NotContains(t, captured.err.Error(), testLimiterClientKey)
					}
				})
			}
		}
	}
}

func requireContextRecords(t *testing.T, records []storagetest.Call, key, wantValue string, wantCanceled bool) {
	t.Helper()

	var matched []storagetest.Call
	for _, rec := range records {
		if rec.Key == key {
			matched = append(matched, rec)
		}
	}
	require.Len(t, matched, 2)
	for _, rec := range matched {
		require.Equal(t, wantValue, rec.Marker)
		require.Equal(t, wantCanceled, rec.Canceled)
	}
}

func Test_Limiter_PropagatesRequestContextToStorage(t *testing.T) {
	t.Parallel()
	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()
			storage := storagetest.New()

			app := fiber.New()

			app.Use(func(c fiber.Ctx) error {
				path := c.Path()
				if path == "/normal" {
					c.SetContext(storagetest.Marked(strategy.name + "-normal"))
				}
				if path == "/rollback" {
					c.SetContext(storagetest.MarkedCanceled(strategy.name + "-rollback"))
				}
				return c.Next()
			})

			app.Use(New(Config{
				Storage:                storage,
				Max:                    1,
				Expiration:             time.Minute,
				SkipSuccessfulRequests: true,
				KeyGenerator:           func(c fiber.Ctx) string { return c.Path() },
				LimiterMiddleware:      strategy.handler,
			}))

			app.Get("/:mode", func(c fiber.Ctx) error { return c.SendString("ok") })

			for _, path := range []string{"/normal", "/rollback"} {
				apptest.Expect(t, app, path, fiber.StatusOK)
			}

			gets, sets := storage.Calls(storagetest.OpGet), storage.Calls(storagetest.OpSet)
			require.Len(t, gets, 4)
			require.Len(t, sets, 4)
			requireContextRecords(t, gets, storagePrefix+"/normal", strategy.name+"-normal", false)
			requireContextRecords(t, gets, storagePrefix+"/rollback", strategy.name+"-rollback", true)
			requireContextRecords(t, sets, storagePrefix+"/normal", strategy.name+"-normal", false)
			requireContextRecords(t, sets, storagePrefix+"/rollback", strategy.name+"-rollback", true)
		})
	}
}

// limiterTestFailingFirst returns a constructor for a storage whose first op, storagetest.OpGet or
// storagetest.OpSet, on the client key fails.
func limiterTestFailingFirst(op string) func() fiber.Storage {
	return func() fiber.Storage {
		return storagetest.New().SetErr(op, storagePrefix+testLimiterClientKey, errors.New("boom"))
	}
}

// limiterTestFailingSecondGet is a storage whose second get fails, the re-read that follows the first.
func limiterTestFailingSecondGet() fiber.Storage {
	st := newCountingFailStorage(0, nil)
	st.getFailErr = errors.New("second get failed")
	st.getFailAfterN = 1
	return st
}

// limiterTestFailingSecondSet is a storage whose second set fails, the write that follows the first.
func limiterTestFailingSecondSet() fiber.Storage {
	return newCountingFailStorage(1, errors.New("second set failed"))
}

// Every storage error path must release the key's shard, or the next request
// for that key hangs.
func Test_Limiter_StorageErrorsReleaseShard(t *testing.T) {
	t.Parallel()
	const (
		getErr = "limiter: failed to get key"
		setErr = "limiter: failed to persist state"
	)

	tests := []struct {
		newStorage     func() fiber.Storage
		middleware     Handler
		name           string
		wantErr        string
		skipSuccessful bool
	}{
		{
			name: "sliding first get", middleware: SlidingWindow{}, wantErr: getErr,
			newStorage: limiterTestFailingFirst(storagetest.OpGet),
		},
		{
			name: "sliding first set", middleware: SlidingWindow{}, wantErr: setErr,
			newStorage: limiterTestFailingFirst(storagetest.OpSet),
		},
		{name: "sliding header get", middleware: SlidingWindow{}, newStorage: limiterTestFailingSecondGet, wantErr: getErr},
		{name: "sliding header set", middleware: SlidingWindow{}, newStorage: limiterTestFailingSecondSet, wantErr: setErr},
		{
			name: "fixed first get", middleware: FixedWindow{}, wantErr: getErr,
			newStorage: limiterTestFailingFirst(storagetest.OpGet),
		},
		{
			name: "fixed first set", middleware: FixedWindow{}, wantErr: setErr,
			newStorage: limiterTestFailingFirst(storagetest.OpSet),
		},
		{name: "fixed skip get", middleware: FixedWindow{}, newStorage: limiterTestFailingSecondGet, wantErr: getErr, skipSuccessful: true},
		{name: "fixed skip set", middleware: FixedWindow{}, newStorage: limiterTestFailingSecondSet, wantErr: setErr, skipSuccessful: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var captured limiterTestCapture
			app := limiterTestApp(&Config{
				Storage:                tc.newStorage(),
				LimiterMiddleware:      tc.middleware,
				Max:                    10,
				Expiration:             time.Second,
				SkipSuccessfulRequests: tc.skipSuccessful,
				KeyGenerator:           func(fiber.Ctx) string { return testLimiterClientKey },
			}, captured.config())

			apptest.Expect(t, app, "/", fiber.StatusInternalServerError)
			require.ErrorContains(t, captured.err, tc.wantErr)

			// A shard left locked by the error path would hang this same-key request.
			apptest.Expect(t, app, "/", fiber.StatusInternalServerError)
		})
	}
}

type skipStep struct {
	path string
	want int
}

func Test_Limiter_Skip_Choices(t *testing.T) {
	t.Parallel()
	scenarios := []struct {
		name  string
		steps []skipStep
		after skipStep // asked after the window; no path means no wait
		cfg   Config
	}{
		{
			name:  "no_skip",
			cfg:   Config{Max: 2},
			steps: []skipStep{{"/fail", fiber.StatusBadRequest}, {"/success", fiber.StatusOK}, {"/success", fiber.StatusTooManyRequests}},
			after: skipStep{"/success", fiber.StatusOK},
		},
		{
			name:  "skip_failed",
			cfg:   Config{Max: 1, SkipFailedRequests: true},
			steps: []skipStep{{"/fail", fiber.StatusBadRequest}, {"/success", fiber.StatusOK}, {"/success", fiber.StatusTooManyRequests}},
			after: skipStep{"/success", fiber.StatusOK},
		},
		{
			name:  "skip_successful",
			cfg:   Config{Max: 1, SkipSuccessfulRequests: true},
			steps: []skipStep{{"/success", fiber.StatusOK}, {"/fail", fiber.StatusBadRequest}, {"/fail", fiber.StatusTooManyRequests}},
			after: skipStep{"/fail", fiber.StatusBadRequest},
		},
		{
			name:  "max_func_zero",
			cfg:   Config{MaxFunc: func(fiber.Ctx) int { return 0 }},
			steps: []skipStep{{"/fail", fiber.StatusBadRequest}, {"/success", fiber.StatusOK}, {"/success", fiber.StatusOK}},
		},
	}

	strategies := []struct {
		handler Handler
		wait    func(t *testing.T, resp *http.Response)
		name    string
	}{
		{name: "fixed", handler: FixedWindow{}, wait: func(*testing.T, *http.Response) { time.Sleep(3 * time.Second) }},
		{name: "sliding", handler: SlidingWindow{}, wait: sleepForRetryAfter},
	}

	for _, scenario := range scenarios {
		for _, strategy := range strategies {
			for _, store := range limiterTestMemoryStores {
				t.Run(scenario.name+"/"+strategy.name+"/"+store.name, func(t *testing.T) {
					t.Parallel()
					cfg := scenario.cfg
					cfg.Expiration = 2 * time.Second
					cfg.LimiterMiddleware = strategy.handler
					cfg.Storage = store.storage()

					app := fiber.New()
					app.Use(New(cfg))
					app.Get("/:status", func(c fiber.Ctx) error {
						if c.Params("status") == "fail" {
							return c.SendStatus(fiber.StatusBadRequest)
						}
						return c.SendStatus(fiber.StatusOK)
					})

					var resp *http.Response
					for _, step := range scenario.steps {
						resp = apptest.Expect(t, app, step.path, step.want)
					}

					if scenario.after.path == "" {
						return
					}

					strategy.wait(t, resp)

					apptest.Expect(t, app, scenario.after.path, scenario.after.want)
				})
			}
		}
	}
}

func Test_Limiter_Sliding_MaxFuncOverridesStaticMax(t *testing.T) {
	t.Parallel()
	app := limiterTestApp(&Config{
		Max:               5,
		MaxFunc:           func(fiber.Ctx) int { return 2 },
		Expiration:        2 * time.Second,
		LimiterMiddleware: SlidingWindow{},
	})

	limiterTestExhaust(t, app, 2)
}

func Test_Limiter_ExpirationFuncOverridesStaticExpiration(t *testing.T) {
	t.Parallel()
	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()
			app, clock := limiterTestClockedApp(&Config{
				Max:               2,
				Expiration:        10 * time.Second,
				ExpirationFunc:    func(_ fiber.Ctx) time.Duration { return 2 * time.Second },
				Storage:           storagetest.New(),
				LimiterMiddleware: strategy.handler,
			})

			for _, want := range []int{fiber.StatusOK, fiber.StatusOK, fiber.StatusTooManyRequests} {
				apptest.Expect(t, app, "/", want)
			}

			// Two 2s windows, far short of the 10s Expiration.
			clock.Add(5 * time.Second)

			apptest.Expect(t, app, "/", fiber.StatusOK)
		})
	}
}

// Test_Limiter_ExpirationFunc_NonPositiveUsesConfiguredExpiration covers an
// ExpirationFunc that returns zero or less: the window is the configured
// Expiration, not the package default of one minute. Only the existing tests
// for this fallback checked that the limiter still limits, so a configured
// ten-second window was a minute long for those requests, and Retry-After and
// X-RateLimit-Reset said so.
func Test_Limiter_ExpirationFunc_NonPositiveUsesConfiguredExpiration(t *testing.T) {
	t.Parallel()
	const expiration = 10 * time.Second

	strategies := []struct {
		handler Handler
		// storage returns the store the strategy is tested with: the fixed window
		// keeps its entry in storage, whose own expiry would otherwise run on
		// real time while the test moves a fake clock.
		storage func() fiber.Storage
		name    string
	}{
		{name: "fixed", handler: FixedWindow{}, storage: func() fiber.Storage { return storagetest.New() }},
		{name: "sliding", handler: SlidingWindow{}, storage: func() fiber.Storage { return nil }},
	}

	for _, strategy := range strategies {
		for _, returned := range []time.Duration{0, -time.Second} {
			t.Run(strategy.name+"/"+returned.String(), func(t *testing.T) {
				t.Parallel()
				app, clock := limiterTestClockedApp(&Config{
					Max:               1,
					Expiration:        expiration,
					ExpirationFunc:    func(fiber.Ctx) time.Duration { return returned },
					LimiterMiddleware: strategy.handler,
					Storage:           strategy.storage(),
				})

				resp := apptest.Expect(t, app, "/", fiber.StatusOK)
				require.Equal(t, "10", resp.Header.Get(xRateLimitReset), "the window is the configured Expiration")

				resp = apptest.Expect(t, app, "/", fiber.StatusTooManyRequests)
				require.Equal(t, "10", resp.Header.Get(fiber.HeaderRetryAfter))

				// Just over the configured window, the client is admitted again. Two
				// whole windows are needed for the sliding one to forget the last.
				clock.Add(2*expiration + time.Second)

				apptest.Expect(t, app, "/", fiber.StatusOK)
			})
		}
	}
}

// Test_Limiter_ExpirationFunc_PositiveStillWins checks the other side of the
// fallback: a positive value from ExpirationFunc keeps overriding Expiration.
func Test_Limiter_ExpirationFunc_PositiveStillWins(t *testing.T) {
	t.Parallel()
	for name, handler := range map[string]Handler{"fixed": FixedWindow{}, "sliding": SlidingWindow{}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app, _ := limiterTestClockedApp(&Config{
				Max:               5,
				Expiration:        time.Minute,
				ExpirationFunc:    func(fiber.Ctx) time.Duration { return 3 * time.Second },
				LimiterMiddleware: handler,
			})

			resp := apptest.Do(t, app, "/")
			require.Equal(t, "3", resp.Header.Get(xRateLimitReset))
		})
	}
}

func Test_Limiter_SubSecondExpirationIsOneSecondWindow(t *testing.T) {
	t.Parallel()
	// A positive Expiration below one second is a one-second window.
	require.Equal(t, 500*time.Millisecond, configDefault(Config{Expiration: 500 * time.Millisecond}).Expiration)

	for _, tc := range []struct {
		handler Handler
		storage func() fiber.Storage
		name    string
		advance time.Duration // how far the clock moves to be admitted again
	}{
		// Past the one-second window the client is admitted again.
		{name: "fixed", handler: FixedWindow{}, storage: func() fiber.Storage { return storagetest.New() }, advance: 1300 * time.Millisecond},
		// Two whole windows on, the previous window carries no weight.
		{name: "sliding", handler: SlidingWindow{}, storage: func() fiber.Storage { return nil }, advance: 2300 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, clock := limiterTestClockedApp(&Config{
				Max:               1,
				Expiration:        500 * time.Millisecond,
				LimiterMiddleware: tc.handler,
				Storage:           tc.storage(),
			})

			apptest.Expect(t, app, "/", fiber.StatusOK)

			resp := apptest.Expect(t, app, "/", fiber.StatusTooManyRequests)
			require.Equal(t, "1", resp.Header.Get(fiber.HeaderRetryAfter))

			clock.Add(tc.advance)

			apptest.Expect(t, app, "/", fiber.StatusOK)
		})
	}
}

// limiterTestConcurrentOK sends n concurrent requests and requires each to be
// answered 200 "Hello tester!".
func limiterTestConcurrentOK(t *testing.T, app *fiber.App, n int) {
	t.Helper()

	var wg sync.WaitGroup

	for range n {
		wg.Go(func() {
			resp := apptest.Do(t, app, "/")
			assert.Equal(t, fiber.StatusOK, resp.StatusCode)

			body := apptest.Body(t, resp)
			assert.Equal(t, "Hello tester!", body)
		})
	}

	wg.Wait()
}

// requireBurstLimited sends n concurrent requests, expects a 429, then a 200 after the window.
func requireBurstLimited(t *testing.T, app *fiber.App, n int) {
	t.Helper()

	limiterTestConcurrentOK(t, app, n)

	apptest.Expect(t, app, "/", fiber.StatusTooManyRequests)

	time.Sleep(3 * time.Second)

	apptest.Expect(t, app, "/", fiber.StatusOK)
}

// Test_Limiter_Concurrency sends a burst of concurrent requests that the limit admits: the request after
// the burst is limited until the window ends. A MaxFunc that returns 0 limits nothing.
func Test_Limiter_Concurrency(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		cfg     Config
		burst   int
		custom  bool // an explicit in-memory Storage instead of the limiter's own store
		limited bool // the request after the burst is limited
	}{
		{name: "default_store", cfg: Config{Max: 50}, burst: 50, limited: true},
		{name: "custom_store", cfg: Config{Max: 50}, custom: true, burst: 50, limited: true},
		{name: "max_func", cfg: Config{MaxFunc: func(fiber.Ctx) int { return 10 }}, custom: true, burst: 10, limited: true},
		{name: "max_func_zero", cfg: Config{MaxFunc: func(fiber.Ctx) int { return 0 }}, custom: true, burst: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := tc.cfg
			cfg.Expiration = 2 * time.Second
			if tc.custom {
				cfg.Storage = memory.New()
			}

			app := fiber.New()
			app.Use(New(cfg))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("Hello tester!") })

			if tc.limited {
				requireBurstLimited(t, app, tc.burst)
				return
			}
			limiterTestConcurrentOK(t, app, tc.burst)
			apptest.Expect(t, app, "/", fiber.StatusOK)
		})
	}
}

// stallingStorage parks Get for stallKey until release is closed.
type stallingStorage struct {
	fiber.Storage
	entered  chan struct{}
	release  chan struct{}
	stallKey string
	once     sync.Once
}

func (s *stallingStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if key == s.stallKey {
		s.once.Do(func() { close(s.entered) })
		<-s.release
	}
	return s.Storage.GetWithContext(ctx, key)
}

// A key stuck in a storage round-trip must not hold up requests for other keys.
func Test_Limiter_Concurrency_UnrelatedKeys(t *testing.T) {
	t.Parallel()
	fast := "fast"
	for i := 0; shardIndex(storagePrefix+fast) == shardIndex(storagePrefix+"slow"); i++ {
		fast = "fast" + strconv.Itoa(i)
	}

	newReq := func(client string) *http.Request {
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set("X-Client", client)
		return req
	}

	for _, mw := range []Handler{FixedWindow{}, SlidingWindow{}} {
		st := &stallingStorage{Storage: memory.New(), stallKey: storagePrefix + "slow", entered: make(chan struct{}), release: make(chan struct{})}
		app := fiber.New()
		app.Use(New(Config{
			Storage:           st,
			LimiterMiddleware: mw,
			KeyGenerator:      func(c fiber.Ctx) string { return c.Get("X-Client") },
		}))
		app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

		done := make(chan error, 1)
		go func() {
			_, err := app.Test(newReq("slow"), fiber.TestConfig{}) // no timeout, it waits for release
			done <- err
		}()
		apptest.Recv(t, st.entered, time.Second, "%T: the slow request never reached the storage", mw)

		resp, err := app.Test(newReq(fast))
		close(st.release)
		require.NoError(t, err, "%T: a request for an unrelated key waited on the stalled one", mw)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		require.NoError(t, <-done)
	}
}

func Test_Limiter_Sliding_Window_RecalculatesAfterHandlerDelay(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{Max: 2, Expiration: time.Second, LimiterMiddleware: SlidingWindow{}}))

	app.Get("/", func(c fiber.Ctx) error {
		time.Sleep(600 * time.Millisecond)
		return c.SendStatus(fiber.StatusOK)
	})

	for range 2 {
		apptest.Expect(t, app, "/", fiber.StatusOK)
	}

	time.Sleep(2*time.Second + 100*time.Millisecond)

	resp := apptest.Expect(t, app, "/", fiber.StatusOK)
	require.Equal(t, "2", resp.Header.Get(xRateLimitLimit))
	require.Equal(t, "1", resp.Header.Get(xRateLimitRemaining))
	require.NotEmpty(t, resp.Header.Get(xRateLimitReset))
}

func Test_Limiter_Sliding_Window_ExpiresStalePrevHits(t *testing.T) {
	t.Parallel()
	app, clock := limiterTestClockedApp(&Config{Max: 1, Expiration: time.Second, LimiterMiddleware: SlidingWindow{}})

	apptest.Expect(t, app, "/", fiber.StatusOK)

	// Two full windows: the previous hits age out.
	clock.Add(2500 * time.Millisecond)

	resp := apptest.Expect(t, app, "/", fiber.StatusOK)
	require.Equal(t, "0", resp.Header.Get(xRateLimitRemaining))
}

func Test_Limiter_Sliding_Window_SkipFailedRequests_DecrementsPreviousWindow(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                2,
		Expiration:         200 * time.Millisecond,
		SkipFailedRequests: true,
		LimiterMiddleware:  SlidingWindow{},
	}))

	app.Get("/:mode", func(c fiber.Ctx) error {
		if c.Params("mode") == "fail" {
			time.Sleep(300 * time.Millisecond)
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	waitForFail := limiterTestAsync(t, app, apptest.Req(fiber.MethodGet, "/fail"))

	time.Sleep(220 * time.Millisecond)

	apptest.Expect(t, app, "/ok", fiber.StatusOK)

	resp := waitForFail()
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "2", resp.Header.Get(xRateLimitLimit))
	require.Equal(t, "1", resp.Header.Get(xRateLimitRemaining))
	assert.NotEmpty(t, resp.Header.Get(xRateLimitReset))
}

func Test_Limiter_Fixed_Window_SkipSuccessfulRequests_DoesNotCreditNextWindow(t *testing.T) {
	t.Parallel()
	clock := clocktest.NewClock(time.Now().Truncate(time.Second))
	app := fiber.New()

	app.Use(New(Config{
		Max:                    2,
		Expiration:             time.Second,
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      FixedWindow{},
		clock:                  clock.Now,
		Storage:                storagetest.New(),
	}))

	started := make(chan struct{})
	release := make(chan struct{})
	app.Get("/:mode", func(c fiber.Ctx) error {
		switch c.Params("mode") {
		case "slow":
			// Hold the successful request open until the window has rolled over.
			close(started)
			<-release
			return c.SendStatus(fiber.StatusOK)
		case "fail":
			// A failed request is not skipped, so it seeds the new window.
			return c.SendStatus(fiber.StatusInternalServerError)
		default:
			return c.SendStatus(fiber.StatusOK)
		}
	})

	// The slow successful request increments currHits in the first window and
	// only finishes after the window has rolled over. With SkipSuccessfulRequests
	// it credits its hit back; the credit must not be applied to the new window,
	// otherwise its currHits underflows and the next window grants extra requests.
	waitForSlow := limiterTestAsync(t, app, apptest.Req(fiber.MethodGet, "/slow"), fiber.TestConfig{Timeout: 5 * time.Second})

	// Roll the window over only once the slow request holds a hit in the old one,
	// then seed the new window with one counted hit before releasing it.
	<-started
	clock.Add(1500 * time.Millisecond)
	apptest.Expect(t, app, "/fail", fiber.StatusInternalServerError)
	close(release)

	resp := waitForSlow()
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "2", resp.Header.Get(xRateLimitLimit))
	// The credit is skipped after the rollover, so remaining reflects the new
	// window's single counted hit (2 - 1). Before the fix the slow request
	// wrongly decremented the new window's currHits and this read "2".
	require.Equal(t, "1", resp.Header.Get(xRateLimitRemaining))
	assert.NotEmpty(t, resp.Header.Get(xRateLimitReset))
}

// limiterBenchHandler returns the handler of an app that answers GET / with "Hello, World!" behind the
// limiter New(cfg) builds, and a context holding that request.
func limiterBenchHandler(cfg *Config) (fasthttp.RequestHandler, *fasthttp.RequestCtx) {
	app := fiber.New()
	app.Use(New(*cfg))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("Hello, World!") })
	return app.Handler(), limiterTestRequestCtx()
}

func Benchmark_Limiter_Custom_Store(b *testing.B) {
	h, fctx := limiterBenchHandler(&Config{Max: 100, Expiration: 60 * time.Second, Storage: memory.New()})

	for b.Loop() {
		h(fctx)
	}
}

// A Config given straight to a strategy, or none, gets the defaults New would give it.
func Test_Limiter_RawConfigUsesDefaults(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cfg   *Config
		name  string
		limit int
	}{
		{name: "nil", cfg: nil, limit: defaultLimiterMax},
		{name: "ConfigDefault", cfg: &ConfigDefault, limit: defaultLimiterMax},
		{name: "zero value", cfg: &Config{}, limit: defaultLimiterMax},
		{name: "only Max", cfg: &Config{Max: 2}, limit: 2},
		{name: "MaxFunc wins over Max", cfg: &Config{Max: 9, MaxFunc: func(fiber.Ctx) int { return 3 }}, limit: 3},
		{name: "only ExpirationFunc", cfg: &Config{ExpirationFunc: func(fiber.Ctx) time.Duration { return time.Hour }}, limit: defaultLimiterMax},
	}

	for _, strategy := range windowStrategies {
		for _, tc := range tests {
			t.Run(strategy.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				app := fiber.New()
				app.Use(strategy.handler.New(tc.cfg))
				app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

				limiterTestExhaust(t, app, tc.limit)
			})
		}
	}
}

// A strategy given no Config uses the exported ConfigDefault as the application left it.
//
// Intentionally not parallel: it replaces ConfigDefault, and Go pauses parallel
// tests while a sequential one runs.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Limiter_NilConfigKeepsCustomizedDefault(t *testing.T) {
	prev := ConfigDefault
	t.Cleanup(func() { ConfigDefault = prev })
	ConfigDefault.DisableHeaders = true

	for _, strategy := range windowStrategies {
		app := fiber.New()
		app.Use(strategy.handler.New(nil))
		app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err, strategy.name)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, strategy.name)
		require.Empty(t, resp.Header.Get(xRateLimitLimit), strategy.name)
	}
}

// A strategy leaves the Config it is given alone: ConfigDefault is shared.
func Test_Limiter_RawConfigIsNotModified(t *testing.T) {
	t.Parallel()
	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()
			for _, cfg := range []*Config{{Max: 2}, &ConfigDefault} {
				before := *cfg
				strategy.handler.New(cfg)

				require.Nil(t, cfg.MaxFunc)
				require.Nil(t, cfg.ExpirationFunc)
				require.Equal(t, before.Max, cfg.Max)
				require.Equal(t, before.Expiration, cfg.Expiration)
				require.Equal(t, before.KeyGenerator == nil, cfg.KeyGenerator == nil)
				require.Equal(t, before.LimitReached == nil, cfg.LimitReached == nil)
			}
		})
	}
}

// Remaining reads 0 although rejected requests keep their hit.
func Test_Limiter_RemainingHeaderIsNeverNegative(t *testing.T) {
	t.Parallel()
	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()
			clock := clocktest.NewClock(time.Now().Truncate(time.Second))
			app := fiber.New()

			app.Use(New(Config{
				Max:                    1,
				Expiration:             time.Minute,
				SkipSuccessfulRequests: true,
				LimiterMiddleware:      strategy.handler,
				clock:                  clock.Now,
				Storage:                storagetest.New(),
			}))

			started := make(chan struct{})
			release := make(chan struct{})
			app.Get("/:mode", func(c fiber.Ctx) error {
				if c.Params("mode") == "slow" {
					close(started)
					<-release
				}
				return c.SendStatus(fiber.StatusOK)
			})

			waitForSlow := limiterTestAsync(t, app, apptest.Req(fiber.MethodGet, "/slow"), fiber.TestConfig{Timeout: 5 * time.Second})

			<-started
			for range 2 {
				apptest.Expect(t, app, "/fast", fiber.StatusTooManyRequests)
			}
			close(release)

			resp := waitForSlow()
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
			require.Equal(t, "1", resp.Header.Get(xRateLimitLimit))
			require.Equal(t, "0", resp.Header.Get(xRateLimitRemaining))
		})
	}
}

// Skip* counts an error as the 500 it is answered with, a *fiber.Error whatever status the response still carries.
func Test_Limiter_ErrorCountsAsFailed(t *testing.T) {
	t.Parallel()
	plain, typed := errors.New("boom"), fiber.NewErrorf(fiber.StatusInternalServerError, "Error")
	skipFailed, skipSuccessful := Config{SkipFailedRequests: true}, Config{SkipSuccessfulRequests: true}
	tests := []struct {
		err  error
		name string
		want []int // statuses of the requests, Max being 1
		cfg  Config
	}{
		{name: "plain_skip_failed", err: plain, cfg: skipFailed, want: []int{500, 500, 500}},
		{name: "plain_skip_successful", err: plain, cfg: skipSuccessful, want: []int{500, 429, 429}},
		{name: "fiber_error_skip_successful", err: typed, cfg: skipSuccessful, want: []int{500, 429}},
	}

	for _, strategy := range windowStrategies {
		for _, tc := range tests {
			t.Run(strategy.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				cfg := tc.cfg
				cfg.Max = 1
				cfg.Expiration = time.Minute
				cfg.DisableHeaders = true
				cfg.LimiterMiddleware = strategy.handler

				app := fiber.New()
				app.Use(New(cfg))
				app.Get("/", func(fiber.Ctx) error { return tc.err })

				got := make([]int, 0, len(tc.want))
				for range tc.want {
					resp := apptest.Do(t, app, "/")
					got = append(got, resp.StatusCode)
				}
				require.Equal(t, tc.want, got)
			})
		}
	}
}

func Test_Limiter_ConfigDefaultCopyKeepsMax(t *testing.T) {
	t.Parallel()
	cfg := ConfigDefault
	cfg.Max = 2

	limiterTestExhaust(t, limiterTestApp(&cfg), 2)
}

func Test_Limiter_Next_BeforeMaxFunc(t *testing.T) {
	t.Parallel()
	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()
			var maxFuncCalls atomic.Int32
			app := fiber.New()
			app.Use(New(Config{
				Next: func(c fiber.Ctx) bool { return c.Path() == "/skipped" },
				MaxFunc: func(fiber.Ctx) int {
					maxFuncCalls.Add(1)
					return 5
				},
				LimiterMiddleware: strategy.handler,
			}))
			app.Get("/*", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

			for range 3 {
				apptest.Expect(t, app, "/skipped", fiber.StatusOK)
			}
			require.Zero(t, maxFuncCalls.Load(), "MaxFunc ran for requests that Next skips")

			apptest.Expect(t, app, "/limited", fiber.StatusOK)
			require.EqualValues(t, 1, maxFuncCalls.Load())
		})
	}
}

func Test_Limiter_NegativeMaxFuncRejectsEveryRequest(t *testing.T) {
	t.Parallel()
	for _, strategy := range windowStrategies {
		for _, limit := range []int{-1, math.MinInt} {
			t.Run(fmt.Sprintf("%s/%d", strategy.name, limit), func(t *testing.T) {
				t.Parallel()
				app := limiterTestApp(&Config{
					MaxFunc:           func(fiber.Ctx) int { return limit },
					LimiterMiddleware: strategy.handler,
				})

				apptest.Expect(t, app, "/", fiber.StatusTooManyRequests)
			})
		}
	}
}

// limiterTestExpiration is an ExpirationFunc that gives a request for path the window d and every other
// request the window others.
func limiterTestExpiration(path string, d, others time.Duration) func(fiber.Ctx) time.Duration {
	return func(c fiber.Ctx) time.Duration {
		if c.Path() == path {
			return d
		}
		return others
	}
}

// Requests sharing a key share the window of the one that opened it (the docs' /login example).
func Test_Limiter_ExpirationFunc_SharedKeyKeepsOpeningWindow(t *testing.T) {
	t.Parallel()
	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()
			app, clock := limiterTestClockedApp(&Config{
				Max:               1000,
				Expiration:        30 * time.Second,
				ExpirationFunc:    limiterTestExpiration("/login", 60*time.Second, 30*time.Second),
				LimiterMiddleware: strategy.handler,
				Storage:           storagetest.New(),
			})

			reset := func(path string) string {
				resp := apptest.Expect(t, app, path, fiber.StatusOK)
				return resp.Header.Get(xRateLimitReset)
			}

			require.Equal(t, "30", reset("/"))

			// /login joins the open window.
			clock.Add(time.Second)
			require.Equal(t, "29", reset("/login"))

			if strategy.name != "fixed" {
				return
			}

			// The next window follows its first request.
			clock.Add(30 * time.Second)
			require.Equal(t, "60", reset("/login"))
		})
	}
}

// A shorter window joining an open one must not shorten the entry's life.
func Test_Limiter_Fixed_ShorterWindowDoesNotShortenTheEntry(t *testing.T) {
	t.Parallel()
	type step struct {
		path string
		at   time.Duration
		want int
	}
	tests := []struct {
		name        string
		steps       []step
		cfg         Config
		shortStatus int // what "/" answers
	}{
		{
			name:        "counted",
			shortStatus: fiber.StatusOK,
			steps: []step{
				{"/login", 0, fiber.StatusOK},
				{"/", time.Second, fiber.StatusOK},
				{"/login", 35 * time.Second, fiber.StatusTooManyRequests},
			},
		},
		{
			name:        "credited_back",
			cfg:         Config{SkipFailedRequests: true},
			shortStatus: fiber.StatusInternalServerError,
			steps: []step{
				{"/login", 0, fiber.StatusOK},
				{"/", time.Second, fiber.StatusInternalServerError},
				{"/login", 35 * time.Second, fiber.StatusOK},
				{"/login", 36 * time.Second, fiber.StatusTooManyRequests},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			start := time.Now().Truncate(time.Second)
			clock := clocktest.NewClock(start)
			cfg := tc.cfg
			cfg.Max = 2
			cfg.ExpirationFunc = limiterTestExpiration("/login", time.Minute, 30*time.Second)
			cfg.LimiterMiddleware = FixedWindow{}
			cfg.clock = clock.Now
			cfg.Storage = &expiringStorage{Memory: storagetest.New(), clock: clock, expires: map[string]time.Time{}}

			app := fiber.New()
			app.Use(New(cfg))
			app.Get("/login", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
			app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(tc.shortStatus) })

			for _, s := range tc.steps {
				clock.Add(start.Add(s.at).Sub(clock.Now()))
				resp := apptest.Do(t, app, s.path)
				require.Equal(t, s.want, resp.StatusCode, "%s at +%s", s.path, s.at)
			}
		})
	}
}

// A shorter window joining a longer open one must not inflate the previous weight.
func Test_Limiter_Sliding_MixedWindowsDoNotInflateThePreviousWindow(t *testing.T) {
	t.Parallel()
	app, clock := limiterTestClockedApp(&Config{
		Max:               5,
		Expiration:        10 * time.Second,
		ExpirationFunc:    limiterTestExpiration("/short", 2*time.Second, 10*time.Second),
		LimiterMiddleware: SlidingWindow{},
		Storage:           storagetest.New(),
	})

	apptest.Do(t, app, "/")
	clock.Add(10 * time.Second) // the first window becomes the previous one

	resp := apptest.Do(t, app, "/")
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "3", resp.Header.Get(xRateLimitRemaining))

	// 9 of 10s are left: the previous hit weighs ceil(0.9) = 1, the rate is 1 + 2, leaving 2 of 5.
	clock.Add(time.Second)
	resp = apptest.Do(t, app, "/short")
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "2", resp.Header.Get(xRateLimitRemaining))
}

// limiterStores are the built-in store, which copies keys, and a Storage that keeps the string it is given.
var limiterStores = []struct {
	storage func() fiber.Storage
	name    string
}{
	{name: "builtin", storage: func() fiber.Storage { return nil }},
	{name: "keeps_key", storage: func() fiber.Storage { return storagetest.New() }},
}

// keyOverwritingApp serves a handler that overwrites the memory behind the key (c.Get does not copy).
func keyOverwritingApp(cfg *Config) *fiber.App {
	cfg.Expiration = time.Minute
	cfg.KeyGenerator = func(c fiber.Ctx) string { return c.Get("X-Client") }

	app := fiber.New()
	app.Use(New(*cfg))
	app.Get("/", func(c fiber.Ctx) error {
		// Same length, so fasthttp overwrites the key's bytes in place.
		c.Request().Header.Set("X-Client", "BBBB")
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

// sendAsClient sends a request under the key keyOverwritingApp's handler overwrites.
func sendAsClient(t *testing.T, app *fiber.App) *http.Response {
	t.Helper()

	return apptest.Do(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHeader("X-Client", "AAAA")))
}

// A handler overwriting the memory behind the key must not misdirect the credit.
func Test_Limiter_KeyOverwrittenByHandler(t *testing.T) {
	t.Parallel()
	for _, strategy := range windowStrategies {
		for _, store := range limiterStores {
			t.Run(strategy.name+"/"+store.name, func(t *testing.T) {
				t.Parallel()
				app := keyOverwritingApp(&Config{
					Max:                    1,
					SkipSuccessfulRequests: true,
					LimiterMiddleware:      strategy.handler,
					Storage:                store.storage(),
				})
				for i := range 3 {
					require.Equal(t, fiber.StatusOK, sendAsClient(t, app).StatusCode, "request %d", i+1)
				}
			})
		}
	}
}

// A Storage that keeps the key it is given must not be handed memory the request owns, even when
// nothing reads the entry again: the next request rewrites that memory and the client finds nothing.
func Test_Limiter_KeyOverwrittenByHandler_NothingRereads(t *testing.T) {
	t.Parallel()
	for _, strategy := range windowStrategies {
		for _, store := range limiterStores {
			t.Run(strategy.name+"/"+store.name, func(t *testing.T) {
				t.Parallel()
				// The key aliases request memory, as c.Get does; the handler rewrites it, as the next request would.
				first, second := []byte("AAAA"), []byte("AAAA")
				current := first
				app := fiber.New()
				app.Use(New(Config{
					Max:               1,
					Expiration:        time.Minute,
					DisableHeaders:    true,
					KeyGenerator:      func(fiber.Ctx) string { return utils.UnsafeString(current) },
					LimiterMiddleware: strategy.handler,
					Storage:           store.storage(),
				}))
				app.Get("/", func(c fiber.Ctx) error {
					copy(current, "BBBB")
					return c.SendStatus(fiber.StatusOK)
				})

				status := func() int {
					resp := apptest.Do(t, app, "/")
					require.NoError(t, resp.Body.Close())
					return resp.StatusCode
				}
				require.Equal(t, fiber.StatusOK, status(), "first request")
				current = second
				require.Equal(t, fiber.StatusTooManyRequests, status(), "the same client, again")
			})
		}
	}
}

// A handler overwriting the key must not misdirect the sliding window's header refresh.
func Test_Limiter_KeyOverwrittenByHandler_Headers(t *testing.T) {
	t.Parallel()
	for _, store := range limiterStores {
		t.Run(store.name, func(t *testing.T) {
			t.Parallel()
			app := keyOverwritingApp(&Config{Max: 5, LimiterMiddleware: SlidingWindow{}, Storage: store.storage()})
			for i, remaining := range []string{"4", "3", "2"} {
				resp := sendAsClient(t, app)
				require.Equal(t, fiber.StatusOK, resp.StatusCode, "request %d", i+1)
				require.Equal(t, remaining, resp.Header.Get(xRateLimitRemaining), "request %d", i+1)
			}
		})
	}
}

// rejectingHandler returns a handler and context for a limiter whose Max requests are used up.
func rejectingHandler(tb testing.TB, cfg *Config, expiration time.Duration) (fasthttp.RequestHandler, *fasthttp.RequestCtx) {
	tb.Helper()

	cfg.Max = 1
	cfg.Expiration = expiration
	cfg.KeyGenerator = func(fiber.Ctx) string { return testLimiterClientKey }

	return limiterTestHandler(cfg)
}

// limiterTestFewestAllocs is the fewest allocations of many single runs of h on fctx, not an
// average: under -race sync.Pool drops a quarter of what it is handed at random, so an average
// varies from one run to the next.
//
// It calls testing.AllocsPerRun, so its callers must not be parallel.
func limiterTestFewestAllocs(h fasthttp.RequestHandler, fctx *fasthttp.RequestCtx) float64 {
	fewest := math.MaxFloat64
	for range 200 {
		fewest = min(fewest, testing.AllocsPerRun(1, func() { h(fctx) }))
	}
	return fewest
}

// A rejected request never reaches the handler, so it skips the key copy made for the read after it.
//
// Intentionally not parallel: testing.AllocsPerRun panics when called from a parallel test.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Limiter_RejectDoesNotCopyKey(t *testing.T) {
	rejectAllocs := func(t *testing.T, cfg Config) float64 {
		t.Helper()

		// A Retry-After under 100 formats without allocating.
		h, fctx := rejectingHandler(t, &cfg, time.Minute)
		h(fctx)
		require.Equal(t, fiber.StatusTooManyRequests, fctx.Response.StatusCode())

		return limiterTestFewestAllocs(h, fctx)
	}

	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			bare := rejectAllocs(t, Config{LimiterMiddleware: strategy.handler, DisableHeaders: true})
			// SkipSuccessfulRequests refunds every hit here, so nothing would be rejected.
			rereading := rejectAllocs(t, Config{LimiterMiddleware: strategy.handler, DisableHeaders: true, SkipFailedRequests: true})
			require.InDelta(t, bare, rereading, 0, "the Skip options must not add to a rejected request")

			// Sliding also re-reads the entry for the headers.
			if strategy.name == "sliding" {
				headers := rejectAllocs(t, Config{LimiterMiddleware: strategy.handler})
				require.InDelta(t, bare, headers, 0, "the header refresh must not add to a rejected request")
			}
		})
	}
}

// admittedHandler returns a handler and context for a limiter that admits every request, with the
// headers off and, unless cfg has its own, the key testLimiterClientKey.
func admittedHandler(tb testing.TB, cfg *Config) (fasthttp.RequestHandler, *fasthttp.RequestCtx) {
	tb.Helper()

	cfg.Max = math.MaxInt32
	cfg.Expiration = time.Minute
	cfg.DisableHeaders = true
	if cfg.KeyGenerator == nil {
		cfg.KeyGenerator = func(fiber.Ctx) string { return testLimiterClientKey }
	}

	h, fctx := limiterTestHandler(cfg)
	require.Equal(tb, fiber.StatusOK, fctx.Response.StatusCode())
	return h, fctx
}

// The built-in store copies the keys it keeps, so an admitted request copies the key only for the re-read.
//
// Intentionally not parallel: testing.AllocsPerRun panics when called from a parallel test.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Limiter_AdmitCopiesKeyOnlyForReread(t *testing.T) {
	admitAllocs := func(t *testing.T, cfg Config) float64 {
		t.Helper()

		return limiterTestFewestAllocs(admittedHandler(t, &cfg))
	}

	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			bare := admitAllocs(t, Config{LimiterMiddleware: strategy.handler})
			// A 200 is not a failed request, so SkipFailedRequests adds the copy and no second read.
			rereading := admitAllocs(t, Config{LimiterMiddleware: strategy.handler, SkipFailedRequests: true})
			require.InDelta(t, bare+1, rereading, 0, "only the re-read may copy the key")
		})
	}
}

// With the built-in store an admitted request allocates the key once, in the store, and once more when
// the limiter reads the entry again after the handler. A copy made before that, as a Storage that may
// keep the key needs, would be one more on every request.
//
// Intentionally not parallel: it counts the allocations of the whole process.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Limiter_AdmitKeyCopiesWithBuiltInStore(t *testing.T) {
	const shortLen, longLen = 8, 160 // a longer key than 192 bytes is hashed before it reaches the store

	// The bytes an admitted request allocates: the fewest of many single runs, as in the tests above.
	admitBytes := func(t *testing.T, cfg Config, keyLen int) float64 {
		t.Helper()

		key := strings.Repeat("k", keyLen)
		cfg.KeyGenerator = func(fiber.Ctx) string { return key }
		h, fctx := admittedHandler(t, &cfg)

		fewest := math.MaxFloat64
		var before, after runtime.MemStats
		for range 400 {
			runtime.ReadMemStats(&before)
			h(fctx)
			runtime.ReadMemStats(&after)
			fewest = min(fewest, float64(after.TotalAlloc-before.TotalAlloc))
		}
		return fewest
	}

	for _, strategy := range windowStrategies {
		for _, tc := range []struct {
			name   string
			cfg    Config
			copies float64
		}{
			{name: "admit", cfg: Config{}, copies: 1},
			// A 200 is not a failed request, so SkipFailedRequests adds the re-read.
			{name: "re-read", cfg: Config{SkipFailedRequests: true}, copies: 2},
		} {
			t.Run(strategy.name+"/"+tc.name, func(t *testing.T) {
				cfg := tc.cfg
				cfg.LimiterMiddleware = strategy.handler

				// What a request allocates grows with the key by one key length per copy of it.
				copies := (admitBytes(t, cfg, longLen) - admitBytes(t, cfg, shortLen)) / (longLen - shortLen)
				require.Less(t, copies, tc.copies+0.5, "%.2f copies of the key per request", copies)
			})
		}
	}
}

// The key comes from the request, so the entry must not land on one that another
// middleware keeps in a shared Storage.
func Test_Limiter_SharedStorage_KeyNamespace(t *testing.T) {
	t.Parallel()
	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()
			const foreignKey = "csrf:token"
			storage := storagetest.New()
			storage.Data[foreignKey] = []byte("+")

			app := fiber.New()
			app.Use(New(Config{
				Storage:           storage,
				Max:               1,
				Expiration:        time.Minute,
				LimiterMiddleware: strategy.handler,
				KeyGenerator:      func(c fiber.Ctx) string { return c.Get("X-Client") },
			}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

			req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			req.Header.Set("X-Client", foreignKey)
			apptest.Expect(t, app, req, fiber.StatusOK)
			require.Equal(t, []byte("+"), storage.Data[foreignKey])
			require.Contains(t, storage.Data, storagePrefix+foreignKey)
		})
	}
}

func Test_Limiter_DisableValueRedaction_ShowsBoundedKey(t *testing.T) {
	t.Parallel()
	oversizedKey := strings.Repeat("k", maxKeyLength*2)
	storage := storagetest.New()
	storage.SetErr(storagetest.OpGet, storagePrefix+boundKey(oversizedKey), errors.New("boom"))

	var captured limiterTestCapture
	app := limiterTestApp(&Config{
		Storage:               storage,
		DisableValueRedaction: true,
		KeyGenerator:          func(fiber.Ctx) string { return oversizedKey },
	}, captured.config())

	apptest.Do(t, app, "/")
	require.ErrorContains(t, captured.err, boundKey(oversizedKey))
	require.NotContains(t, captured.err.Error(), oversizedKey)
}

// Test_Limiter_StorageRoundTrips pins the storage reads and writes of a passing request, as the docs describe.
func Test_Limiter_StorageRoundTrips(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		cfg   Config
		trips int // reads, and as many writes
	}{
		{name: "fixed", cfg: Config{LimiterMiddleware: FixedWindow{}}, trips: 1},
		{name: "fixed_skip_successful", cfg: Config{LimiterMiddleware: FixedWindow{}, SkipSuccessfulRequests: true}, trips: 2},
		{name: "sliding", cfg: Config{LimiterMiddleware: SlidingWindow{}}, trips: 2},
		{name: "sliding_disable_headers", cfg: Config{LimiterMiddleware: SlidingWindow{}, DisableHeaders: true}, trips: 1},
		{
			name: "sliding_disable_headers_skip_successful", trips: 2,
			cfg: Config{LimiterMiddleware: SlidingWindow{}, DisableHeaders: true, SkipSuccessfulRequests: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			storage := storagetest.New()
			cfg := tc.cfg
			cfg.Max = 1
			cfg.Expiration = time.Second
			cfg.Storage = storage

			apptest.Expect(t, limiterTestApp(&cfg), "/", fiber.StatusOK)

			require.Len(t, storage.Calls(storagetest.OpGet), tc.trips)
			require.Len(t, storage.Calls(storagetest.OpSet), tc.trips)
		})
	}
}

func Test_Limiter_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Next: func(_ fiber.Ctx) bool { return true }}))

	apptest.Expect(t, app, "/", fiber.StatusNotFound)
}

func Test_Limiter_Headers(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Max: 50, Expiration: 2 * time.Second}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("Hello tester!") })

	resp := apptest.Do(t, app, "/")
	require.Equal(t, "50", resp.Header.Get("X-RateLimit-Limit"))
	if v := resp.Header.Get("X-RateLimit-Remaining"); v == "" {
		t.Error("The X-RateLimit-Remaining header is not set correctly - value is an empty string.")
	}
	if v := resp.Header.Get("X-RateLimit-Reset"); (v != "1") && (v != "2") {
		t.Error("The X-RateLimit-Reset header is not set correctly - value is out of bounds.")
	}
}

func Test_Limiter_Disable_Headers(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Max: 1, Expiration: 2 * time.Second, DisableHeaders: true}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("Hello tester!") })

	resp := apptest.ExpectBody(t, app, "/", fiber.StatusOK, "Hello tester!")
	require.Empty(t, resp.Header.Get("X-RateLimit-Limit"))
	require.Empty(t, resp.Header.Get("X-RateLimit-Remaining"))
	require.Empty(t, resp.Header.Get("X-RateLimit-Reset"))

	// the limited request carries no headers either
	resp = apptest.Expect(t, app, "/", fiber.StatusTooManyRequests)
	require.Empty(t, resp.Header.Get(fiber.HeaderRetryAfter))
	require.Empty(t, resp.Header.Get("X-RateLimit-Limit"))
	require.Empty(t, resp.Header.Get("X-RateLimit-Remaining"))
	require.Empty(t, resp.Header.Get("X-RateLimit-Reset"))
}

func Benchmark_Limiter(b *testing.B) {
	h, fctx := limiterBenchHandler(&Config{Max: 100, Expiration: 60 * time.Second})

	for b.Loop() {
		h(fctx)
	}
}

func Benchmark_Limiter_Parallel_Keys(b *testing.B) {
	var clients atomic.Uint64

	app := fiber.New()

	app.Use(New(Config{
		Max:          b.N, // no key can pass b.N hits, so no iteration takes the 429 path
		Expiration:   60 * time.Second,
		KeyGenerator: func(c fiber.Ctx) string { return c.Get("X-Client") },
	}))

	app.Get("/", func(c fiber.Ctx) error { return c.SendString("Hello, World!") })

	h := app.Handler()

	// One distinct key per goroutine: unrelated clients must not serialize.
	b.RunParallel(func(pb *testing.PB) {
		fctx := limiterTestRequestCtx()
		fctx.Request.Header.Set("X-Client", utils.FormatUint(clients.Add(1)))

		for pb.Next() {
			h(fctx)
		}
	})
}

// Benchmark_Limiter_Admit measures requests that pass: Max is too high to reach the 429 path.
func Benchmark_Limiter_Admit(b *testing.B) {
	oversizedKey := strings.Repeat("x", maxKeyLength*4)
	oversized := func(fiber.Ctx) string { return oversizedKey }

	benchmarks := []struct {
		name string
		cfg  Config
	}{
		{name: "fixed", cfg: Config{LimiterMiddleware: FixedWindow{}}},
		{name: "fixed_skip_successful", cfg: Config{LimiterMiddleware: FixedWindow{}, SkipSuccessfulRequests: true}},
		{name: "fixed_oversized_key", cfg: Config{LimiterMiddleware: FixedWindow{}, KeyGenerator: oversized}},
		{name: "sliding", cfg: Config{LimiterMiddleware: SlidingWindow{}}},
		{name: "sliding_disable_headers", cfg: Config{LimiterMiddleware: SlidingWindow{}, DisableHeaders: true}},
		{name: "sliding_skip_successful", cfg: Config{LimiterMiddleware: SlidingWindow{}, SkipSuccessfulRequests: true}},
		{name: "sliding_oversized_key", cfg: Config{LimiterMiddleware: SlidingWindow{}, KeyGenerator: oversized}},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			cfg := bm.cfg
			cfg.Max = math.MaxInt32
			cfg.Expiration = time.Hour
			h, fctx := limiterBenchHandler(&cfg)

			b.ReportAllocs()

			for b.Loop() {
				h(fctx)
			}
		})
	}
}

// Benchmark_Limiter_Reject measures requests the limit rejects: the handler never runs.
func Benchmark_Limiter_Reject(b *testing.B) {
	benchmarks := []struct {
		name string
		cfg  Config
	}{
		{name: "fixed", cfg: Config{LimiterMiddleware: FixedWindow{}}},
		{name: "fixed_skip_failed", cfg: Config{LimiterMiddleware: FixedWindow{}, SkipFailedRequests: true}},
		{name: "sliding", cfg: Config{LimiterMiddleware: SlidingWindow{}}},
		{name: "sliding_disable_headers", cfg: Config{LimiterMiddleware: SlidingWindow{}, DisableHeaders: true}},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			h, fctx := rejectingHandler(b, &bm.cfg, time.Hour)

			b.ReportAllocs()

			for b.Loop() {
				h(fctx)
			}
		})
	}
}

func Test_Sliding_Window(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Max:               10,
		Expiration:        1 * time.Second,
		Storage:           memory.New(),
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c fiber.Ctx) error { return c.SendString("Hello tester!") })

	for _, step := range []struct {
		wait     time.Duration // before the requests
		requests int
		want     int
	}{
		{requests: 5, want: fiber.StatusOK},
		{wait: 3 * time.Second, requests: 5, want: fiber.StatusOK},
		{wait: 3 * time.Second, requests: 5, want: fiber.StatusOK},
		{wait: 3 * time.Second, requests: 10, want: fiber.StatusOK},
		// requests should fail now
		{requests: 5, want: fiber.StatusTooManyRequests},
	} {
		time.Sleep(step.wait)

		for range step.requests {
			apptest.Expect(t, app, "/", step.want)
		}
	}
}

// A sub-second ExpirationFunc floors to a 1s window: a 0s one made the sliding rate NaN.
func Test_Limiter_SubSecondExpirationFunc(t *testing.T) {
	t.Parallel()
	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()
			app, _ := limiterTestClockedApp(&Config{
				Max:            5,
				ExpirationFunc: func(fiber.Ctx) time.Duration { return 500 * time.Millisecond },
				// the memory store expires by the real clock, which a slow runner could outrun
				Storage:           storagetest.New(),
				LimiterMiddleware: strategy.handler,
			})

			for i := 1; i <= 5; i++ {
				resp := apptest.Do(t, app, "/")
				require.Equal(t, fiber.StatusOK, resp.StatusCode, "request %d", i)
				require.Equal(t, "1", resp.Header.Get(xRateLimitReset), "request %d", i)
			}

			resp := apptest.Expect(t, app, "/", fiber.StatusTooManyRequests)
			require.Equal(t, "1", resp.Header.Get(fiber.HeaderRetryAfter))
		})
	}
}

// Test_Limiter_OversizedKeyGeneratorIsBounded proves the property manager's
// boundKey exists for: an oversized key — the shape a raw, unvalidated
// ProxyHeader value or an unchecked custom KeyGenerator would produce — never
// reaches the storage backend verbatim. Runs both window strategies, since
// each calls the manager independently.
func Test_Limiter_OversizedKeyGeneratorIsBounded(t *testing.T) {
	t.Parallel()
	for _, strategy := range []Handler{FixedWindow{}, SlidingWindow{}} {
		t.Run(fmt.Sprintf("%T", strategy), func(t *testing.T) {
			t.Parallel()
			oversizedKey := strings.Repeat("x", maxKeyLength*4)
			storage := storagetest.New()
			app := fiber.New()
			app.Use(New(Config{
				Storage:           storage,
				Max:               1,
				Expiration:        time.Minute,
				LimiterMiddleware: strategy,
				KeyGenerator:      func(fiber.Ctx) string { return oversizedKey },
			}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

			apptest.Expect(t, app, "/", fiber.StatusOK)

			require.Len(t, storage.Data, 1, "exactly one bucket should have been written")
			for storedKey := range storage.Data {
				require.NotEqual(t, oversizedKey, storedKey, "the raw oversized key must never reach storage")
				require.LessOrEqual(t, len(storedKey), maxKeyLength)
				require.True(t, strings.HasPrefix(storedKey, storagePrefix+hashPrefix))
			}

			// The same oversized key must still be recognized as the same client on
			// the next request, so rate limiting keeps working for it rather than
			// silently exempting it or colliding it with someone else's bucket.
			apptest.Expect(t, app, "/", fiber.StatusTooManyRequests)
			require.Len(t, storage.Data, 1, "the second request must land in the same bucket, not a new one")
		})
	}
}

// Test_Limiter_HeadersExactWireFormat pins serialized field-name casing, values,
// order and CRLF for both strategies, normalization modes and disabled headers.
func Test_Limiter_HeadersExactWireFormat(t *testing.T) {
	t.Parallel()
	type step struct {
		headers string
		status  int
	}
	for _, ws := range windowStrategies {
		for _, mode := range []struct {
			name            string
			prefix          string
			disableApp      bool
			disableResponse bool
		}{
			{name: "normalized", prefix: "X-Ratelimit-"},
			{name: "app_unnormalized", prefix: "X-RateLimit-", disableApp: true},
			{name: "response_unnormalized", prefix: "X-RateLimit-", disableResponse: true},
		} {
			allowed := func(remaining, reset string) step {
				return step{status: fiber.StatusOK, headers: mode.prefix + "Limit: 2\r\n" +
					mode.prefix + "Remaining: " + remaining + "\r\n" +
					mode.prefix + "Reset: " + reset + "\r\n"}
			}
			for _, disableHeaders := range []bool{false, true} {
				t.Run(ws.name+"/"+mode.name+"/disabled="+strconv.FormatBool(disableHeaders), func(t *testing.T) {
					t.Parallel()
					steps := []step{allowed("1", "10"), allowed("0", "9"), {
						status: fiber.StatusTooManyRequests, headers: "Retry-After: 8\r\n",
					}}
					if disableHeaders {
						for i := range steps {
							steps[i].headers = ""
						}
					}
					clock := clocktest.NewClock(time.Unix(1_700_000_000, 0))
					app := fiber.New(fiber.Config{DisableHeaderNormalizing: mode.disableApp})
					if mode.disableResponse {
						app.Use(func(c fiber.Ctx) error {
							c.Response().Header.DisableNormalizing()
							return c.Next()
						})
					}
					app.Use(New(Config{
						Max:               2,
						Expiration:        10 * time.Second,
						LimiterMiddleware: ws.handler,
						DisableHeaders:    disableHeaders,
						clock:             clock.Now,
					}))
					app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })
					handler := app.Handler()

					for i, want := range steps {
						fctx := limiterTestRequestCtx()
						// Calling Handler directly bypasses fasthttp's setup of the header store.
						if app.Server().DisableHeaderNamesNormalizing {
							fctx.Response.Header.DisableNormalizing()
						}
						handler(fctx)

						require.Equal(t, want.status, fctx.Response.StatusCode(), "request %d", i)
						var got []string
						for line := range strings.SplitAfterSeq(string(fctx.Response.Header.Header()), "\r\n") {
							lower := strings.ToLower(line)
							if strings.HasPrefix(lower, "x-ratelimit-") || strings.HasPrefix(lower, "retry-after:") {
								got = append(got, line)
							}
						}
						require.Equal(t, want.headers, strings.Join(got, ""), "request %d", i)
						clock.Add(time.Second)
					}
				})
			}
		}
	}
}

type limiterHeaderWrite struct {
	key   string
	value string
}

// limiterHeaderCtx retains the original strings as well as suppressing and
// rewriting headers, all of which a custom Set implementation is allowed to do.
type limiterHeaderCtx struct {
	writes *[]limiterHeaderWrite
	fiber.DefaultCtx
}

func (c *limiterHeaderCtx) Set(key, value string) {
	switch key {
	case xRateLimitLimit, xRateLimitRemaining, xRateLimitReset, fiber.HeaderRetryAfter:
		*c.writes = append(*c.writes, limiterHeaderWrite{key: key, value: value})
	}
	switch key {
	case xRateLimitLimit:
		return
	case xRateLimitRemaining:
		value = "custom-" + value
	}
	c.DefaultCtx.Set(key, value)
}

func Test_Limiter_HeadersCustomCtx(t *testing.T) {
	t.Parallel()
	for _, ws := range windowStrategies {
		for _, disableHeaders := range []bool{false, true} {
			t.Run(ws.name+"/disabled="+strconv.FormatBool(disableHeaders), func(t *testing.T) {
				t.Parallel()
				var writes []limiterHeaderWrite
				app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
					return &limiterHeaderCtx{DefaultCtx: *fiber.NewDefaultCtx(app), writes: &writes}
				})
				clock := clocktest.NewClock(time.Unix(1_700_000_000, 0))
				app.Use(New(Config{
					Max:               2,
					Expiration:        10 * time.Second,
					LimiterMiddleware: ws.handler,
					DisableHeaders:    disableHeaders,
					clock:             clock.Now,
				}))
				app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })
				handler := app.Handler()
				for i := range 3 {
					fctx := limiterTestRequestCtx()
					handler(fctx)
					h := &fctx.Response.Header
					require.Empty(t, h.Peek(xRateLimitLimit))
					if i < 2 {
						require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode())
						if !disableHeaders {
							require.Equal(t, "custom-"+strconv.Itoa(1-i), string(h.Peek(xRateLimitRemaining)))
							require.Equal(t, strconv.Itoa(10-i), string(h.Peek(xRateLimitReset)))
						}
					} else {
						require.Equal(t, fiber.StatusTooManyRequests, fctx.Response.StatusCode())
						if !disableHeaders {
							require.Equal(t, "8", string(h.Peek(fiber.HeaderRetryAfter)))
						}
					}
					clock.Add(time.Second)
				}
				if disableHeaders {
					require.Empty(t, writes)
				} else {
					require.Equal(t, []limiterHeaderWrite{
						{key: xRateLimitLimit, value: "2"},
						{key: xRateLimitRemaining, value: "1"},
						{key: xRateLimitReset, value: "10"},
						{key: xRateLimitLimit, value: "2"},
						{key: xRateLimitRemaining, value: "0"},
						{key: xRateLimitReset, value: "9"},
						{key: fiber.HeaderRetryAfter, value: "8"},
					}, writes)
				}
			})
		}
	}
}

func Test_Limiter_HeaderValues(t *testing.T) {
	t.Parallel()
	for _, unnormalized := range []bool{false, true} {
		t.Run("unnormalized="+strconv.FormatBool(unnormalized), func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, fiber.New())
			var want fasthttp.ResponseHeader
			if unnormalized {
				c.Response().Header.DisableNormalizing()
				want.DisableNormalizing()
			}
			cfg := Config{}
			for _, values := range []struct {
				limit     int
				remaining int
				reset     uint64
			}{
				{limit: math.MinInt, remaining: math.MinInt, reset: math.MaxUint64},
				{limit: math.MaxInt, remaining: math.MaxInt, reset: math.MaxUint64},
				{limit: 10000, remaining: 9999, reset: 86400},
				{limit: 0, remaining: 0, reset: 0},
			} {
				want.Set(xRateLimitLimit, utils.FormatInt(int64(values.limit)))
				want.Set(xRateLimitRemaining, utils.FormatInt(int64(max(values.remaining, 0))))
				want.Set(xRateLimitReset, utils.FormatUint(values.reset))
				cfg.setRateLimitHeaders(c, values.limit, values.remaining, values.reset)
				require.Equal(t, string(want.Header()), string(c.Response().Header.Header()))
			}
		})
	}
}

func Benchmark_Limiter_Headers(b *testing.B) {
	for _, values := range []struct {
		name      string
		limit     int
		remaining int
		reset     uint64
	}{
		{name: "small", limit: 2, remaining: 1, reset: 10},
		{name: "large", limit: math.MaxInt, remaining: math.MaxInt - 1, reset: math.MaxUint64},
	} {
		for _, legacy := range []bool{false, true} {
			b.Run(values.name+"/legacy="+strconv.FormatBool(legacy), func(b *testing.B) {
				c := apptest.Ctx(b, fiber.New())
				cfg := Config{}
				set := func() { cfg.setRateLimitHeaders(c, values.limit, values.remaining, values.reset) }
				if legacy {
					set = func() {
						c.Set(xRateLimitLimit, utils.FormatInt(int64(values.limit)))
						c.Set(xRateLimitRemaining, utils.FormatInt(int64(max(values.remaining, 0))))
						c.Set(xRateLimitReset, utils.FormatUint(values.reset))
					}
				}
				set() // Warm the header storage; measure number formatting and writes.
				b.ReportAllocs()
				for b.Loop() {
					set()
				}
			})
		}
	}
}
