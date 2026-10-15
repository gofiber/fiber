package limiter

import (
	"math"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

const defaultLimiterMax = 5

// Config defines the config for middleware.
type Config struct {
	// Storage is used to store the state of the middleware
	//
	// Default: an in memory store for this process only
	Storage fiber.Storage

	// LimiterMiddleware is the struct that implements a limiter middleware.
	//
	// Default: a new Fixed Window Rate Limiter
	LimiterMiddleware Handler
	// Next defines a function to skip this middleware when returned true. It is
	// called before MaxFunc, so a request it skips never reaches MaxFunc.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// A function to dynamically calculate the max requests supported by the rate limiter middleware.
	// Returning 0 skips the limiter for the request; a negative value rejects every request.
	//
	// Default: func(c fiber.Ctx) int {
	//   return cfg.Max
	// }
	MaxFunc func(c fiber.Ctx) int

	// A function to dynamically calculate the expiration time for rate limiter entries.
	// Window accounting is whole-second, so a positive value below one second is floored to
	// one second. A zero or negative value falls back to the configured Expiration.
	//
	// The window of a key keeps the length given by the request that opened it, so requests
	// that share a key (by default the client IP) share one window whatever their own
	// ExpirationFunc says. A route that needs its own window needs its own key.
	//
	// Default: A function that returns the static `Expiration` value from the config.
	ExpirationFunc func(c fiber.Ctx) time.Duration

	// clock overrides the timestamp source used for window accounting. It is
	// unexported so it never becomes part of the public API and exists solely to
	// make time-dependent tests deterministic. When nil, the cached
	// utils.Timestamp() is used on the hot path.
	clock func() time.Time

	// KeyGenerator allows you to generate custom keys, by default c.IP() is used.
	// Behind a reverse proxy, c.IP() only identifies a client when TrustProxy,
	// TrustProxyConfig and ProxyHeader are configured, plus EnableIPValidation when
	// the header is a chain the proxy appends to (X-Forwarded-For); otherwise it is
	// the proxy's address or the raw, client-influenced forwarded header.
	//
	// Default: func(c fiber.Ctx) string {
	//   return c.IP()
	// }
	KeyGenerator func(fiber.Ctx) string

	// LimitReached is called when a request hits the limit
	//
	// Default: func(c fiber.Ctx) error {
	//   return c.SendStatus(fiber.StatusTooManyRequests)
	// }
	LimitReached fiber.Handler

	// Max number of recent connections during `Expiration` seconds before sending a 429 response
	//
	// Default: 5
	Max int

	// Expiration is the time on how long to keep records of requests in memory
	//
	// Default: 1 * time.Minute
	Expiration time.Duration

	// When set to true, requests with StatusCode >= 400 won't be counted.
	// A handler that returns an error other than a *fiber.Error counts as a 500 unless it
	// already set an error status, which is the response the default error handler gives it.
	//
	// Default: false
	SkipFailedRequests bool

	// When set to true, requests with StatusCode < 400 won't be counted.
	// A handler that returns an error other than a *fiber.Error counts as a 500 unless it
	// already set an error status, which is the response the default error handler gives it.
	//
	// Default: false
	SkipSuccessfulRequests bool

	// When set to true, the middleware will not include the rate limit headers (X-RateLimit-* and Retry-After) in the response.
	//
	// Default: false
	DisableHeaders bool

	// DisableValueRedaction turns off masking limiter keys in logs and error messages when set to true.
	// The key shown is the one the storage sees, so a key longer than 192 bytes appears as its
	// SHA-256 digest.
	//
	// Default: false
	DisableValueRedaction bool
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Max:        defaultLimiterMax,
	Expiration: 1 * time.Minute,
	// Note: MaxFunc and ExpirationFunc are intentionally nil here so that configDefault()
	// can create proper closures that reference the configured Max and Expiration values,
	// also when a caller starts from a copy of ConfigDefault and changes them.
	KeyGenerator: func(c fiber.Ctx) string {
		return c.IP()
	},
	LimitReached: func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTooManyRequests)
	},
	SkipFailedRequests:     false,
	SkipSuccessfulRequests: false,
	DisableHeaders:         false,
	DisableValueRedaction:  false,
	LimiterMiddleware:      FixedWindow{},
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Use default config if nothing provided
	var cfg Config
	if len(config) < 1 {
		cfg = ConfigDefault
	} else {
		cfg = config[0]
	}

	// Set default values
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.Max <= 0 {
		cfg.Max = ConfigDefault.Max
	}
	if cfg.Expiration <= 0 {
		cfg.Expiration = ConfigDefault.Expiration
	}
	if cfg.KeyGenerator == nil {
		cfg.KeyGenerator = ConfigDefault.KeyGenerator
	}
	if cfg.LimitReached == nil {
		cfg.LimitReached = ConfigDefault.LimitReached
	}
	if cfg.LimiterMiddleware == nil {
		cfg.LimiterMiddleware = ConfigDefault.LimiterMiddleware
	}
	if cfg.MaxFunc == nil {
		cfg.MaxFunc = func(_ fiber.Ctx) int {
			return cfg.Max
		}
	}
	if cfg.ExpirationFunc == nil {
		cfg.ExpirationFunc = func(_ fiber.Ctx) time.Duration {
			return cfg.Expiration
		}
	}
	return cfg
}

// windowSeconds turns a per-request expiration into the whole-second window used
// for accounting. A zero or negative value falls back to configured, the
// Expiration the limiter was set up with. A positive sub-second value floors
// to 1s instead of truncating to a 0-second window (NaN rate in sliding, no
// limiting at all in fixed).
func windowSeconds(d, configured time.Duration) uint64 {
	if d <= 0 {
		d = configured
	}
	if d <= 0 {
		// A Config that did not go through configDefault carries no Expiration.
		d = ConfigDefault.Expiration
	}
	if sec := uint64(d.Seconds()); sec > 0 {
		return sec
	}
	return 1
}

// currentSecond returns the current Unix time in whole seconds used for window
// accounting. Production reads the cached utils.Timestamp() (refreshed by a 1s
// ticker) to keep the hot path allocation- and syscall-free; tests can inject a
// deterministic clock via the unexported clock field.
func (cfg *Config) currentSecond() uint64 {
	if cfg.clock != nil {
		// Guard against pre-epoch (negative) timestamps so the int64 -> uint64
		// conversion never wraps to a huge value and corrupts window math.
		sec := cfg.clock().Unix()
		if sec < 0 {
			return 0
		}
		return uint64(sec)
	}
	return uint64(utils.Timestamp())
}

// secondsToDuration converts whole seconds to a Duration. It saturates at the
// largest Duration and reports false when seconds does not fit.
func secondsToDuration(seconds uint64) (time.Duration, bool) {
	const maxSeconds = math.MaxInt64 / int64(time.Second)

	if seconds > uint64(maxSeconds) {
		return time.Duration(math.MaxInt64), false
	}

	return time.Duration(seconds) * time.Second, true
}

// ttlDuration returns how long a sliding-window entry must be kept: the time
// left in the current window plus one whole window, so it outlives the window
// that still weighs on the next one. It saturates instead of overflowing.
func ttlDuration(resetSec, windowSec uint64) time.Duration {
	resetDuration, ok := secondsToDuration(resetSec)
	if !ok {
		return time.Duration(math.MaxInt64)
	}

	windowDuration, ok := secondsToDuration(windowSec)
	if !ok {
		return time.Duration(math.MaxInt64)
	}

	if resetDuration > time.Duration(math.MaxInt64)-windowDuration {
		return time.Duration(math.MaxInt64)
	}

	return resetDuration + windowDuration
}
