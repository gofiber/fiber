package limiter

import (
	"math"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// SlidingWindow implements the sliding-window rate limiting strategy.
type SlidingWindow struct{}

// New creates a new sliding window middleware handler
func (SlidingWindow) New(cfg *Config) fiber.Handler {
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
		// Don't execute middleware if Next returns true or if MaxFunc returns 0
		maxRequests, ok := cfg.limit(c)
		if !ok {
			return c.Next()
		}

		// Generate the window from the generator
		windowSec := windowSeconds(cfg.ExpirationFunc(c), cfg.Expiration)

		// Get key from request, copied since it is read again after the handler,
		// which may overwrite the request memory an unsafe key points to
		key := cfg.KeyGenerator(c)
		if cfg.SkipSuccessfulRequests || cfg.SkipFailedRequests || !cfg.DisableHeaders {
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

		// Rotate window
		resetInSec := e.rotateSliding(ts, windowSec)
		windowExpiresAt := e.exp

		// Increment hits
		e.currHits++

		// Calculate how many hits can be made based on the current rate
		remaining := maxRequests - e.rate(resetInSec, windowSec)

		// Update storage. Garbage collect when the next window ends.
		// |--------------------------|--------------------------|
		//               ^            ^               ^          ^
		//              ts         e.exp   End sample window   End next window
		//               <------------>
		// 				   Reset In Sec
		// resetInSec = e.exp - ts - time until end of current window.
		// duration + expiration = end of next window.
		// Because we don't want to garbage collect in the middle of a window
		// we add the expiration to the duration.
		// Otherwise, after the end of "sample window", attackers could launch
		// a new request with the full window length.
		if setErr := manager.set(reqCtx, key, e, ttlDuration(resetInSec, windowSec)); setErr != nil {
			mu.Unlock()
			return errPersist(setErr)
		}

		// Unlock entry
		mu.Unlock()

		// Check if hits exceed the allowed maximum for this request
		if remaining < 0 {
			return cfg.reject(c, resetInSec)
		}

		// Continue stack for reaching c.Res().StatusCode()
		// Store err for returning
		err = c.Next()

		// Check for SkipFailedRequests and SkipSuccessfulRequests
		skipHit := cfg.skipsHit(getEffectiveStatusCode(c, err))

		if skipHit || !cfg.DisableHeaders {
			// Lock entry
			mu.Lock()
			entry, getErr := manager.get(reqCtx, key)
			if getErr != nil {
				mu.Unlock()
				return getErr
			}
			e = entry

			ts = cfg.currentSecond()
			resetInSec = e.rotateSliding(ts, windowSec)

			if skipHit {
				if counter := e.hitBucket(windowExpiresAt, ts, windowSec); counter != nil && *counter > 0 {
					*counter--
				}
			}

			remaining = maxRequests - e.rate(resetInSec, windowSec)
			if setErr := manager.set(reqCtx, key, e, ttlDuration(resetInSec, windowSec)); setErr != nil {
				mu.Unlock()
				return errPersist(setErr)
			}
			// Unlock entry
			mu.Unlock()

			// We can continue, update RateLimit headers
			cfg.setRateLimitHeaders(c, maxRequests, remaining, resetInSec)
		}

		return err
	}
}

// rotateSliding moves the windows forward to ts and returns the seconds left in the current one.
func (e *item) rotateSliding(ts, windowSec uint64) uint64 {
	// Set expiration if entry does not exist
	if e.exp == 0 {
		e.exp = ts + windowSec
	} else if ts >= e.exp {
		// The entry has expired, handle the expiration.
		// Reset the current hits to 0.
		elapsed := ts - e.exp
		if elapsed >= windowSec {
			e.prevHits = 0
			e.currHits = 0
			e.exp = ts + windowSec
		} else {
			e.prevHits = e.currHits
			e.currHits = 0
			e.exp += windowSec
		}
	}

	// Calculate when it resets in seconds
	return e.exp - ts
}

// rate estimates the requests in the window ending resetInSec from now: the previous
// window's hits, weighted by the share of it still inside the window, plus the current's.
func (e *item) rate(resetInSec, windowSec uint64) int {
	// weight = time until current window reset / total window length, capped at 1: the open
	// window may come from a longer ExpirationFunc window than this request's.
	weight := min(float64(resetInSec)/float64(windowSec), 1)

	return int(math.Ceil(float64(e.prevHits)*weight)) + e.currHits
}

// hitBucket returns the counter holding a hit made while the window ending at
// hitWindowEnd was current, or nil once it has aged out.
func (e *item) hitBucket(hitWindowEnd, ts, windowSec uint64) *int {
	if ts < hitWindowEnd {
		return &e.currHits
	}

	if ts-hitWindowEnd < windowSec {
		return &e.prevHits
	}

	return nil
}
