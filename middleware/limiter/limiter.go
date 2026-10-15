package limiter

import (
	"errors"
	"fmt"
	"hash/maphash"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/nilerror"
	"github.com/gofiber/utils/v2"
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

// shardIndex returns the index of the lock shard that guards key.
func shardIndex(key string) uint64 {
	return maphash.String(keySeed, key) & (lockShards - 1)
}

// keyedMutex serializes the read-modify-write of a key's entry, so one client's
// storage round-trip no longer blocks every other client.
//
// A shard is held across the storage calls of that read-modify-write on
// purpose. Without it, two concurrent requests for one key would both read the
// same count and both write count+1, admitting more requests than the limit.
// Issue #4358 asked for no lock across storage I/O, but that needs atomic
// storage operations, which fiber.Storage does not offer. The cost is that two
// keys sharing a shard wait for each other's storage round-trips.
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

// limit returns the maximum number of requests the limiter allows for c. It
// reports false when the limiter stands aside for this request: Next returned
// true, or MaxFunc returned 0.
func (cfg *Config) limit(c fiber.Ctx) (int, bool) {
	maxRequests := cfg.MaxFunc(c)
	if (cfg.Next != nil && cfg.Next(c)) || maxRequests == 0 {
		return 0, false
	}
	return maxRequests, true
}

// skipsHit reports whether a request that ended with statusCode must not count
// against the limit, as SkipSuccessfulRequests and SkipFailedRequests ask.
func (cfg *Config) skipsHit(statusCode int) bool {
	return (cfg.SkipSuccessfulRequests && statusCode < fiber.StatusBadRequest) ||
		(cfg.SkipFailedRequests && statusCode >= fiber.StatusBadRequest)
}

// reject answers a request that is over the limit: unless headers are
// disabled it adds Retry-After, then it hands over to LimitReached.
//
// Retry-After is defined in RFC 9110 section 10.2.3 and is what a 429 response
// carries to say when to retry (RFC 6585 section 4).
func (cfg *Config) reject(c fiber.Ctx, resetInSec uint64) error {
	if !cfg.DisableHeaders {
		c.Set(fiber.HeaderRetryAfter, utils.FormatUint(resetInSec))
	}
	return cfg.LimitReached(c)
}

// setRateLimitHeaders reports the state of the limit on a request that passed.
// Blocked requests keep their hit, so remaining can be below zero by now: it is
// clamped to keep the header meaningful.
func (cfg *Config) setRateLimitHeaders(c fiber.Ctx, maxRequests, remaining int, resetInSec uint64) {
	if cfg.DisableHeaders {
		return
	}
	c.Set(xRateLimitLimit, utils.FormatInt(int64(maxRequests)))
	c.Set(xRateLimitRemaining, utils.FormatInt(int64(max(remaining, 0))))
	c.Set(xRateLimitReset, utils.FormatUint(resetInSec))
}

// errPersist wraps the failure to store an updated entry.
func errPersist(err error) error {
	return fmt.Errorf("limiter: failed to persist state: %w", err)
}

// getEffectiveStatusCode returns the actual status code, considering both the error and response status
func getEffectiveStatusCode(c fiber.Ctx, err error) int {
	if nilerror.IsNil(err) {
		return c.Res().StatusCode()
	}

	// If there's an error and it's a *fiber.Error, use its status code
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) && fiberErr != nil {
		return fiberErr.Code
	}

	// Otherwise, use the response status code
	return c.Res().StatusCode()
}
