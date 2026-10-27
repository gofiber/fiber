package limiter

import (
	"errors"
	"fmt"
	"hash/maphash"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/nilerror"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

const (
	// X-RateLimit-* headers
	xRateLimitLimit     = "X-RateLimit-Limit"
	xRateLimitRemaining = "X-RateLimit-Remaining"
	xRateLimitReset     = "X-RateLimit-Reset"
)

// lockShards is a power of two so the shard index is a mask. A fixed array stays
// allocation-free and, unlike a per-key map, cannot be grown with fresh keys.
const lockShards = 64

// keySeed randomizes the shard mapping per process, so a client cannot pick
// keys (by default its IP) that all land on one shard.
var keySeed = maphash.MakeSeed()

func shardIndex(key string) uint64 {
	return maphash.String(keySeed, key) & (lockShards - 1)
}

// keyedMutex serializes the read-modify-write per key, so one client's storage
// round-trip no longer blocks every other client. A shard stays locked across
// storage calls because fiber.Storage has no atomic increment.
type keyedMutex [lockShards]sync.Mutex

// lock acquires the shard owning key and returns it for unlocking.
func (k *keyedMutex) lock(key string) *sync.Mutex {
	mu := &k[shardIndex(key)]
	mu.Lock()
	return mu
}

// Handler defines a rate-limiting strategy that can produce a middleware
// handler using the provided configuration.
type Handler interface {
	New(config *Config) fiber.Handler
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Return the specified middleware handler.
	return cfg.LimiterMiddleware.New(&cfg)
}

// withDefaults returns a copy of cfg with the defaults New applies; nil gives ConfigDefault as it is.
func withDefaults(cfg *Config) *Config {
	var filled Config
	if cfg == nil {
		filled = configDefault()
	} else {
		filled = configDefault(*cfg)
	}
	return &filled
}

// limit returns the limit for c, or false when Next or a zero MaxFunc skips it.
// Negative limits become -1 so subtracting hits cannot wrap.
func (cfg *Config) limit(c fiber.Ctx) (int, bool) {
	if cfg.Next != nil && cfg.Next(c) {
		return 0, false
	}
	maxRequests := cfg.MaxFunc(c)
	return max(maxRequests, -1), maxRequests != 0
}

func (cfg *Config) skipsHit(statusCode int) bool {
	return (cfg.SkipSuccessfulRequests && statusCode < fiber.StatusBadRequest) ||
		(cfg.SkipFailedRequests && statusCode >= fiber.StatusBadRequest)
}

// reject sets Retry-After (RFC 9110 section 10.2.3, RFC 6585 section 4), then calls LimitReached.
func (cfg *Config) reject(c fiber.Ctx, resetInSec uint64) error {
	if !cfg.DisableHeaders {
		c.Set(fiber.HeaderRetryAfter, utils.FormatUint(resetInSec))
	}
	return cfg.LimitReached(c)
}

// setRateLimitHeaders clamps remaining: blocked requests keep their hit, so it can go negative.
func (cfg *Config) setRateLimitHeaders(c fiber.Ctx, maxRequests, remaining int, resetInSec uint64) {
	if cfg.DisableHeaders || timeoutResponse(c) != nil {
		return
	}
	c.Set(xRateLimitLimit, utils.FormatInt(int64(maxRequests)))
	c.Set(xRateLimitRemaining, utils.FormatInt(int64(max(remaining, 0))))
	c.Set(xRateLimitReset, utils.FormatUint(resetInSec))
}

// timeoutResponse returns what fasthttp sends for a request timeout.New gave up on, or nil.
// IsAbandoned goes first: its atomic load orders the read after the timeout middleware's write.
func timeoutResponse(c fiber.Ctx) *fasthttp.Response {
	if !c.IsAbandoned() {
		return nil
	}
	return c.RequestCtx().LastTimeoutErrorResponse()
}

func errPersist(err error) error {
	return fmt.Errorf("limiter: failed to persist state: %w", err)
}

// getEffectiveStatusCode returns the status the Skip* options judge: a plain error
// counts as a 500 unless an error status is already set.
func getEffectiveStatusCode(c fiber.Ctx, err error) int {
	if resp := timeoutResponse(c); resp != nil {
		return resp.StatusCode()
	}
	if nilerror.IsNil(err) {
		return c.Res().StatusCode()
	}

	// If there's an error and it's a *fiber.Error, use its status code
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) && fiberErr != nil {
		return fiberErr.Code
	}

	if status := c.Res().StatusCode(); status >= fiber.StatusBadRequest {
		return status
	}
	return fiber.StatusInternalServerError
}
