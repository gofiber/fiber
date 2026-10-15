package limiter

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// FixedWindow implements a fixed-window rate limiting strategy.
type FixedWindow struct{}

// New creates a new fixed window middleware handler
func (FixedWindow) New(cfg *Config) fiber.Handler {
	if cfg == nil {
		defaultCfg := configDefault()
		cfg = &defaultCfg
	}

	// Limiter variables
	locks := new(keyedMutex)

	// Create manager to simplify storage operations ( see manager.go )
	manager := newManager(cfg.Storage, !cfg.DisableValueRedaction)

	// Update timestamp every second
	utils.StartTimeStampUpdater()

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true or if MaxFunc, which returns Max
		// unless configured otherwise, returns 0
		maxRequests, ok := cfg.limit(c)
		if !ok {
			return c.Next()
		}

		// Generate the window from the generator. The storage TTL is derived from
		// it so a sub-second value cannot expire the entry mid-window.
		windowSec := windowSeconds(cfg.ExpirationFunc(c), cfg.Expiration)
		ttl, _ := secondsToDuration(windowSec)

		// Get key from request. A skipped hit is credited back after the handler ran,
		// and a handler can overwrite the request memory an unsafe key string points
		// to, so the key is copied when it is needed again.
		key := cfg.KeyGenerator(c)
		if cfg.SkipSuccessfulRequests || cfg.SkipFailedRequests {
			key = utils.CopyString(key)
		}

		// Lock entry
		mu := locks.lock(key)

		reqCtx := c.Context()

		// Get entry from pool and release when finished
		e, err := manager.get(reqCtx, key)
		if err != nil {
			mu.Unlock()
			return err
		}

		// Get timestamp
		ts := cfg.currentSecond()

		// Open the window if the entry is new, or a fresh one if it has ended
		e.rotateFixed(ts, windowSec)

		// Increment hits
		e.currHits++

		// Calculate when it resets in seconds
		resetInSec := e.exp - ts
		windowExpiresAt := e.exp

		// Set how many hits we have left
		remaining := maxRequests - e.currHits

		// Update storage
		if setErr := manager.set(reqCtx, key, e, ttl); setErr != nil {
			mu.Unlock()
			return errPersist(setErr)
		}

		// Unlock entry
		mu.Unlock()

		// Check if hits exceed the max
		if remaining < 0 {
			return cfg.reject(c, resetInSec)
		}

		// Continue stack for reaching c.Res().StatusCode()
		// Store err for returning
		err = c.Next()

		// Check for SkipFailedRequests and SkipSuccessfulRequests
		if cfg.skipsHit(getEffectiveStatusCode(c, err)) {
			// Lock entry again, the shard of the same key
			mu.Lock()
			entry, getErr := manager.get(reqCtx, key)
			if getErr != nil {
				mu.Unlock()
				return getErr
			}
			e = entry
			// Only credit the hit back if it still belongs to the same window;
			// after a rollover the original hit no longer counts toward e.currHits.
			if e.exp == windowExpiresAt && e.currHits > 0 {
				e.currHits--
			}
			remaining = maxRequests - e.currHits
			if setErr := manager.set(reqCtx, key, e, ttl); setErr != nil {
				mu.Unlock()
				return errPersist(setErr)
			}
			// Unlock entry
			mu.Unlock()
		}

		// We can continue, update RateLimit headers
		cfg.setRateLimitHeaders(c, maxRequests, remaining, resetInSec)

		return err
	}
}

// rotateFixed opens a window for an entry that has none and starts a fresh one
// once the current window has ended.
func (e *item) rotateFixed(ts, windowSec uint64) {
	if e.exp == 0 {
		e.exp = ts + windowSec
	} else if ts >= e.exp {
		e.currHits = 0
		e.exp = ts + windowSec
	}
}
