package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"

	"github.com/gofiber/utils/v2"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/ctxlocal"
	"github.com/gofiber/fiber/v3/internal/errorreplay"
	"github.com/gofiber/fiber/v3/log"
)

// Inspired by https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-idempotency-key-header-02
// and https://github.com/penguin-statistics/backend-next/blob/f2f7d5ba54fc8a58f168d153baa17b2ad4a14e45/internal/pkg/middlewares/idempotency.go

// The contextKey type is unexported to prevent collisions with context keys defined in
// other packages.
type contextKey int

const (
	localsKeyIsFromCache contextKey = iota //
	localsKeyWasPutToCache
)

const redactedKey = "[redacted]"

// isCookieHeader reports whether name is a header that sets a cookie: Set-Cookie
// or its obsolete RFC 2965 spelling.
func isCookieHeader(name string) bool {
	return utils.EqualFold(name, fiber.HeaderSetCookie) || utils.EqualFold(name, fiber.HeaderSetCookie2)
}

// scopedKey returns the key a response is stored and locked under when
// Config.KeyScope is set: a digest of the scope and the key together. The
// length of the scope goes in front of it, so no two pairs share the bytes
// that are hashed and a key cannot spell out another scope, and the digest has
// the same length whatever either is, which a storage that limits the length of
// its keys needs.
func scopedKey(scope, key string) string {
	// A user ID and a UUID fit the array; a longer pair grows onto the heap.
	var buf [128]byte
	b := utils.AppendUint(buf[:0], uint64(len(scope)))
	b = append(b, ':')
	b = append(b, scope...)
	b = append(b, key...)
	sum := sha256.Sum256(b)
	var out [sha256.Size * 2]byte
	hex.Encode(out[:], sum[:])
	return string(out[:])
}

// IsFromCache reports whether the middleware served the response from the
// cache for the current request.
func IsFromCache(c fiber.Ctx) bool {
	return c.Locals(localsKeyIsFromCache) != nil
}

// WasPutToCache reports whether the middleware stored the response produced by
// the current request in the cache.
func WasPutToCache(c fiber.Ctx) bool {
	val := c.Locals(localsKeyWasPutToCache)
	if wasPut, ok := val.(bool); ok {
		return wasPut
	}
	return val != nil
}

// New creates idempotency middleware that caches responses keyed by the
// configured idempotency header.
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	redactKeys := !cfg.DisableValueRedaction

	maskKey := func(key string) string {
		if redactKeys {
			return redactedKey
		}
		return key
	}

	// Snapshot the configured names so later mutation of the caller's slice
	// cannot change an already-constructed handler. Matching uses
	// utils.EqualFold, so no lowercased copies are needed and comparing
	// against the canonical-case names fasthttp reports stays allocation-free.
	keepResponseHeaders := slices.Clone(cfg.KeepResponseHeaders)

	// keepHeader reports whether a header the handler wrote is stored and a
	// stored one is replayed: with no list, every header but the cookies, which
	// only a name in the list keeps; with one, the named headers alone.
	keepHeader := func(header string) bool {
		if keepResponseHeaders == nil {
			return !isCookieHeader(header)
		}
		return slices.ContainsFunc(keepResponseHeaders, func(keep string) bool {
			return utils.EqualFold(header, keep)
		})
	}

	maybeWriteCachedResponse := func(c fiber.Ctx, key string) (bool, error) {
		if val, err := cfg.Storage.GetWithContext(c, key); err != nil {
			return false, fmt.Errorf("failed to read response: %w", err)
		} else if val != nil {
			var res response
			if _, err := res.UnmarshalMsg(val); err != nil {
				return false, fmt.Errorf("failed to unmarshal response: %w", err)
			}

			_ = c.Status(res.StatusCode)

			for header, vals := range res.Headers {
				// A stored entry goes through the same filter: one an earlier
				// version wrote, or one written under another configuration,
				// may hold more than this one keeps.
				if !keepHeader(header) {
					continue
				}
				for _, val := range vals {
					c.RequestCtx().Response.Header.Add(header, val)
				}
			}

			if len(res.Body) != 0 {
				if err := c.Send(res.Body); err != nil {
					return true, err
				}
			}

			_ = ctxlocal.Set(c, localsKeyIsFromCache, true)

			return true, nil
		}

		return false, nil
	}

	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Don't execute middleware if the idempotency key is empty
		key := c.Get(cfg.KeyHeader)
		if key == "" {
			return c.Next()
		}
		// A refused request runs no handler, so there is nothing to store for its retry.
		if errorreplay.Active(c.RequestCtx()) {
			return c.Next()
		}
		// Copied once, before anything reads it: the validator may keep what it
		// is handed, a handler that rewrote the header would not change what is
		// hashed or logged, and the lock and the storage retain the unscoped key.
		key = utils.CopyString(key)

		// Validate key
		if err := cfg.KeyHeaderValidate(key); err != nil {
			return err
		}

		// The key a response is stored and locked under: a digest of the scope
		// and the key when the config scopes it, otherwise the key alone.
		storageKey := key
		if cfg.KeyScope != nil {
			storageKey = scopedKey(cfg.KeyScope(c), key)
		}

		// First-pass: if the idempotency key is in the storage, get and return the response
		if ok, err := maybeWriteCachedResponse(c, storageKey); err != nil {
			return fmt.Errorf("failed to write cached response at fastpath: %w", err)
		} else if ok {
			return nil
		}

		if err := cfg.Lock.Lock(storageKey); err != nil {
			return fmt.Errorf("failed to lock: %w", err)
		}
		defer func() {
			if err := cfg.Lock.Unlock(storageKey); err != nil {
				log.Errorf("[IDEMPOTENCY] failed to unlock key %q: %v", maskKey(key), err)
			}
		}()

		// Lock acquired. If the idempotency key now is in the storage, get and return the response
		if ok, err := maybeWriteCachedResponse(c, storageKey); err != nil {
			return fmt.Errorf("failed to write cached response while locked: %w", err)
		} else if ok {
			return nil
		}

		// Execute the request handler
		if err := c.Next(); err != nil {
			// If the request handler returned an error, return it and skip idempotency
			return err
		}

		// Body would hold a stream that declares no length until it ends, maybe never, so it goes out unrecorded.
		if c.Response().IsBodyStream() && c.Response().Header.ContentLength() < 0 {
			return nil
		}

		// Construct response
		res := &response{
			StatusCode: c.Res().StatusCode(),
			Body:       c.Response().Body(),
		}
		{
			// Read the header lines as the handler wrote them. The response
			// header binder would split values on commas whenever the app sets
			// EnableSplittingOnParsers, turning one Set-Cookie with an Expires
			// date into two broken lines on replay.
			headers := make(map[string][]string)
			for k, v := range c.Response().Header.All() {
				// Judged before the name is copied, so a line that is not
				// kept costs nothing.
				if !keepHeader(utils.UnsafeString(k)) {
					continue
				}
				name := string(k)
				headers[name] = append(headers[name], string(v))
			}
			res.Headers = headers
		}

		// Marshal response
		bs, err := res.MarshalMsg(nil)
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}

		// Store response
		if err := cfg.Storage.SetWithContext(c, storageKey, bs, cfg.Lifetime); err != nil {
			return fmt.Errorf("failed to save response: %w", err)
		}

		_ = ctxlocal.Set(c, localsKeyWasPutToCache, true)

		return nil
	}
}
