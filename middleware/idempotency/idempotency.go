package idempotency

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
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

func IsFromCache(c fiber.Ctx) bool {
	return c.Locals(localsKeyIsFromCache) != nil
}

func WasPutToCache(c fiber.Ctx) bool {
	return c.Locals(localsKeyWasPutToCache) != nil
}

func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	keepResponseHeadersMap := make(map[string]struct{}, len(cfg.KeepResponseHeaders))
	for _, h := range cfg.KeepResponseHeaders {
		keepResponseHeadersMap[strings.ToLower(h)] = struct{}{}
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
				for _, val := range vals {
					c.RequestCtx().Response.Header.Add(header, val)
				}
			}

			if len(res.Body) != 0 {
				if err := c.Send(res.Body); err != nil {
					return true, err
				}
			}

			_ = c.Locals(localsKeyIsFromCache, true)

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
		key := utils.CopyString(c.Get(cfg.KeyHeader))
		if key == "" {
			return c.Next()
		}

		// Validate key
		if err := cfg.KeyHeaderValidate(key); err != nil {
			return err
		}

		// First-pass: if the idempotency key is in the storage, get and return the response
		if ok, err := maybeWriteCachedResponse(c, key); err != nil {
			return fmt.Errorf("failed to write cached response at fastpath: %w", err)
		} else if ok {
			return nil
		}

		if err := cfg.Lock.Lock(key); err != nil {
			return fmt.Errorf("failed to lock: %w", err)
		}
		defer func() {
			if err := cfg.Lock.Unlock(key); err != nil {
				log.Errorf("[IDEMPOTENCY] failed to unlock key %q: %v", key, err)
			}
		}()

		// Lock acquired. If the idempotency key now is in the storage, get and return the response
		if ok, err := maybeWriteCachedResponse(c, key); err != nil {
			return fmt.Errorf("failed to write cached response while locked: %w", err)
		} else if ok {
			return nil
		}

		// Execute the request handler
		if err := c.Next(); err != nil {
			// If the request handler returned an error, return it and skip idempotency
			return err
		}

		// Construct response
		res := &response{
			StatusCode: c.Response().StatusCode(),

			Body: utils.CopyBytes(c.Response().Body()),
		}
		{
			headers := make(map[string][]string)
			if err := c.Bind().RespHeader(headers); err != nil {
				return fmt.Errorf("failed to bind to response headers: %w", err)
			}

			if cfg.KeepResponseHeaders == nil {
				// Keep all
				res.Headers = headers
			} else {
				// Filter
				res.Headers = make(map[string][]string)
				for h := range headers {
					if _, ok := keepResponseHeadersMap[utils.ToLower(h)]; ok {
						res.Headers[h] = headers[h]
					}
				}
			}
		}

		// Marshal response
		bs, err := res.MarshalMsg(nil)
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}

		// Store response
		if err := cfg.Storage.SetWithContext(c, key, bs, cfg.Lifetime); err != nil {
			return fmt.Errorf("failed to save response: %w", err)
		}

		_ = c.Locals(localsKeyWasPutToCache, true)

		return nil
	}
}
