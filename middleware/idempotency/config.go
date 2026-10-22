package idempotency

import (
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
)

var ErrInvalidIdempotencyKey = errors.New("invalid idempotency key")

// Config defines the config for middleware.
type Config struct {
	// Lock locks an idempotency key.
	//
	// Optional. Default: an in-memory locker for this process only.
	Lock Locker

	// Storage stores response data by idempotency key.
	//
	// Optional. Default: an in-memory storage for this process only.
	Storage fiber.Storage

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: a function which skips the middleware on safe HTTP request method.
	Next func(c fiber.Ctx) bool

	// KeyHeaderValidate defines a function to validate the syntax of the idempotency header.
	//
	// Optional. Default: a function which ensures the header is 36 characters long (the size of an UUID).
	// It checks the length only, so it accepts a key that is easy to guess; see KeyScope.
	KeyHeaderValidate func(string) error

	// KeyScope returns the scope an idempotency key belongs to, such as the ID of the
	// authenticated user. A response is stored under the scope and the key together, and
	// replayed only to a request that carries the same key in the same scope, so a key that
	// another user sent or guessed returns nothing stored for you. Requests for which it
	// returns the same string share a scope, the empty string included.
	//
	// Optional. Default: nil (the key alone identifies a request)
	KeyScope func(c fiber.Ctx) string

	// KeyHeader is the name of the header that contains the idempotency key.
	//
	// Optional. Default: X-Idempotency-Key
	KeyHeader string

	// KeepResponseHeaders is a list of headers that should be kept from the original response.
	// A response is replayed to every request that carries its key, so Set-Cookie and the
	// obsolete Set-Cookie2 are kept only when named here: a stored cookie would hand the
	// session of the first client to the others. Every other header is kept by default, so
	// a session or token carried in one, such as the session ID the session middleware
	// writes to the header it reads it from, needs a list that leaves it out, or KeyScope.
	//
	// Optional. Default: nil (to keep all headers but the cookies)
	KeepResponseHeaders []string

	// Lifetime is the maximum lifetime of an idempotency key.
	//
	// Optional. Default: 30 * time.Minute
	Lifetime time.Duration

	// DisableValueRedaction turns off masking idempotency keys in logs and errors when set to true.
	//
	// Optional. Default: false
	DisableValueRedaction bool
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next: func(c fiber.Ctx) bool {
		// Skip middleware if the request was done using a safe HTTP method
		return c.IsSafe()
	},

	Lifetime: 30 * time.Minute,

	KeyHeader: "X-Idempotency-Key",
	KeyHeaderValidate: func(k string) error {
		if l, wl := len(k), 36; l != wl { // UUID length is 36 chars
			return fmt.Errorf("%w: invalid length: %d != %d", ErrInvalidIdempotencyKey, l, wl)
		}

		return nil
	},

	KeyScope: nil,

	KeepResponseHeaders: nil,

	Lock: nil, // Set in configDefault so we don't allocate data here.

	Storage:               nil, // Set in configDefault so we don't allocate data here.
	DisableValueRedaction: false,
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		cfg := ConfigDefault

		cfg.Lock = NewMemoryLock()
		cfg.Storage = memory.New(memory.Config{
			GCInterval: cfg.Lifetime / 2, // Half the lifetime interval
		})

		return cfg
	}

	// Override default config
	cfg := config[0]

	// Set default values

	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}

	if cfg.Lifetime.Nanoseconds() == 0 {
		cfg.Lifetime = ConfigDefault.Lifetime
	}

	if cfg.KeyHeader == "" {
		cfg.KeyHeader = ConfigDefault.KeyHeader
	}
	if cfg.KeyHeaderValidate == nil {
		cfg.KeyHeaderValidate = ConfigDefault.KeyHeaderValidate
	}

	if cfg.KeepResponseHeaders != nil && len(cfg.KeepResponseHeaders) == 0 {
		cfg.KeepResponseHeaders = ConfigDefault.KeepResponseHeaders
	}

	if cfg.Lock == nil {
		cfg.Lock = NewMemoryLock()
	}

	if cfg.Storage == nil {
		cfg.Storage = memory.New(memory.Config{
			GCInterval: cfg.Lifetime / 2,
		})
	}

	return cfg
}
