package limiter

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// FixedWindow implements a fixed-window rate limiting strategy.
type FixedWindow struct{}

// New creates a new fixed window middleware handler. Unset fields of cfg take their defaults.
func (FixedWindow) New(cfg *Config) fiber.Handler {
	cfg = withDefaults(cfg)

	// Limiter variables
	locks := new(keyedMutex)

	// Create manager to simplify storage operations ( see manager.go )
	manager := newManager(cfg.Storage, !cfg.DisableValueRedaction)

	// Update timestamp every second
	utils.StartTimeStampUpdater()

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true or if the max is 0
		maxRequests, ok := cfg.limit(c)
		if !ok {
			return c.Next()
		}

		// Generate expiration from generator
		windowSec := windowSeconds(cfg.ExpirationFunc(c), cfg.Expiration)

		// The key may point into request memory the handler can rewrite: entryKey copies it for a Storage
		// that may keep it. The built-in store copies the keys it keeps, so only the re-read needs one.
		key := manager.entryKey(cfg.KeyGenerator(c))
		rereads := cfg.SkipSuccessfulRequests || cfg.SkipFailedRequests

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

		// Rotate window
		e.rotateFixed(ts, windowSec)

		// Increment hits
		e.currHits++

		// Calculate when it resets in seconds
		resetInSec := e.exp - ts
		windowExpiresAt := e.exp

		// Set how many hits we have left
		remaining := maxRequests - e.currHits

		// Update storage
		if setErr := manager.set(reqCtx, key, e, e.ttl(ts)); setErr != nil {
			mu.Unlock()
			return errPersist(setErr)
		}

		// Unlock entry
		mu.Unlock()

		// Check if hits exceed the max
		if remaining < 0 {
			return cfg.reject(c, resetInSec)
		}

		if rereads && manager.copiesKeys() {
			key = utils.CopyString(key)
		}

		// Continue stack for reaching c.Res().StatusCode()
		// Store err for returning
		err = c.Next()

		// Check for SkipFailedRequests and SkipSuccessfulRequests
		if cfg.skipsHit(getEffectiveStatusCode(c, err)) {
			// Lock entry
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
			if setErr := manager.set(reqCtx, key, e, e.ttl(cfg.currentSecond())); setErr != nil {
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

func (e *item) rotateFixed(ts, windowSec uint64) {
	if e.exp == 0 {
		e.exp = ts + windowSec
	} else if ts >= e.exp {
		e.currHits = 0
		e.exp = ts + windowSec
	}
}

// ttl keeps the entry until its window ends, whichever request opened it, for at least a second.
func (e *item) ttl(ts uint64) time.Duration {
	d, _ := secondsToDuration(max(e.exp, ts+1) - ts)
	return d
}
