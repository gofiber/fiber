package limiter

import (
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

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

	var (
		// Limiter variables
		mux        = &sync.RWMutex{}
		expiration = uint64(cfg.Expiration.Seconds())
	)

	// Create manager to simplify storage operations ( see manager.go )
	manager := newManager(cfg.Storage, !cfg.DisableValueRedaction)

	// Update timestamp every second
	utils.StartTimeStampUpdater()

	// Return new handler
	return func(c fiber.Ctx) error {
		// Generate maxRequests from generator, if no generator was provided the default value returned is 5
		maxRequests := cfg.MaxFunc(c)

		// Don't execute middleware if Next returns true or if the max is 0
		if (cfg.Next != nil && cfg.Next(c)) || maxRequests == 0 {
			return c.Next()
		}

		// Get key from request
		key := cfg.KeyGenerator(c)

		// Lock entry
		mux.Lock()

		reqCtx := c.Context()

		// Get entry from pool and release when finished
		e, err := manager.get(reqCtx, key)
		if err != nil {
			mux.Unlock()
			return err
		}

		// Get timestamp
		ts := uint64(utils.Timestamp())

		// Rotate window
		resetInSec := rotateWindow(e, ts, expiration)

		// Increment hits
		e.currHits++

		// weight = time until current window reset / total window length
		weight := float64(resetInSec) / float64(expiration)

		// rate = request count in previous window - weight + request count in current window
		rate := int(float64(e.prevHits)*weight) + e.currHits

		// Calculate how many hits can be made based on the current rate
		remaining := maxRequests - rate

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
		if setErr := manager.set(reqCtx, key, e, ttlDuration(resetInSec, expiration)); setErr != nil {
			mux.Unlock()
			return fmt.Errorf("limiter: failed to persist state: %w", setErr)
		}

		// Unlock entry
		mux.Unlock()

		// Check if hits exceed the allowed maximum for this request
		if remaining < 0 {
			// Return response with Retry-After header
			// https://tools.ietf.org/html/rfc6584
			if !cfg.DisableHeaders {
				c.Set(fiber.HeaderRetryAfter, strconv.FormatUint(resetInSec, 10))
			}

			// Call LimitReached handler
			return cfg.LimitReached(c)
		}

		// Continue stack for reaching c.Response().StatusCode()
		// Store err for returning
		err = c.Next()

		// Get the effective status code from either the error or response
		statusCode := getEffectiveStatusCode(c, err)

		skipHit := (cfg.SkipSuccessfulRequests && statusCode < fiber.StatusBadRequest) ||
			(cfg.SkipFailedRequests && statusCode >= fiber.StatusBadRequest)

		// Lock entry
		mux.Lock()
		entry, getErr := manager.get(reqCtx, key)
		if getErr != nil {
			mux.Unlock()
			return getErr
		}
		e = entry

		ts = uint64(utils.Timestamp())
		resetInSec = rotateWindow(e, ts, expiration)
		weight = float64(resetInSec) / float64(expiration)

		if skipHit {
			if e.currHits > 0 {
				e.currHits--
			} else if e.prevHits > 0 {
				e.prevHits--
			}
		}

		rate = int(float64(e.prevHits)*weight) + e.currHits
		remaining = maxRequests - rate
		if setErr := manager.set(reqCtx, key, e, ttlDuration(resetInSec, expiration)); setErr != nil {
			mux.Unlock()
			return fmt.Errorf("limiter: failed to persist state: %w", setErr)
		}
		// Unlock entry
		mux.Unlock()

		// We can continue, update RateLimit headers
		if !cfg.DisableHeaders {
			c.Set(xRateLimitLimit, strconv.Itoa(maxRequests))
			c.Set(xRateLimitRemaining, strconv.Itoa(remaining))
			c.Set(xRateLimitReset, strconv.FormatUint(resetInSec, 10))
		}

		return err
	}
}

func rotateWindow(e *item, ts, expiration uint64) uint64 {
	// Set expiration if entry does not exist
	if e.exp == 0 {
		e.exp = ts + expiration
	} else if ts >= e.exp {
		// The entry has expired, handle the expiration.
		// Set the prevHits to the current hits and reset the hits to 0.
		e.prevHits = e.currHits

		// Reset the current hits to 0.
		e.currHits = 0

		// Check how much into the current window it currently is and sets the
		// expiry based on that; otherwise, this would only reset on
		// the next request and not show the correct expiry.
		elapsed := ts - e.exp
		if elapsed >= expiration {
			e.exp = ts + expiration
		} else {
			e.exp = ts + expiration - elapsed
		}
	}

	// Calculate when it resets in seconds
	return e.exp - ts
}

func ttlDuration(resetInSec, expiration uint64) time.Duration {
	resetDuration, ok := secondsToDuration(resetInSec)
	if !ok {
		return time.Duration(math.MaxInt64)
	}

	expirationDuration, ok := secondsToDuration(expiration)
	if !ok {
		return time.Duration(math.MaxInt64)
	}

	if resetDuration > time.Duration(math.MaxInt64)-expirationDuration {
		return time.Duration(math.MaxInt64)
	}

	return resetDuration + expirationDuration
}

func secondsToDuration(seconds uint64) (time.Duration, bool) {
	const maxSeconds = math.MaxInt64 / int64(time.Second)

	if seconds > uint64(maxSeconds) {
		return time.Duration(math.MaxInt64), false
	}

	return time.Duration(seconds) * time.Second, true
}
