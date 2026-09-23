package limiter

import (
	"errors"
	"hash/maphash"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/nilerror"
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

// keyedMutex serializes the read-modify-write per key, so one client's storage
// round-trip no longer blocks every other client.
type keyedMutex [lockShards]sync.Mutex

// lock acquires the shard owning key and returns it for unlocking.
func (k *keyedMutex) lock(key string) *sync.Mutex {
	mu := &k[maphash.String(keySeed, key)&(lockShards-1)]
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
