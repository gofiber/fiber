package favicon

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

const (
	fPath  = "/favicon.ico"
	hType  = "image/x-icon"
	hAllow = "GET, HEAD, OPTIONS"
	hZero  = "0"
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// Load iconData if provided
	var (
		err           error
		iconData      []byte
		iconLenHeader string
		iconLen       int
		f             fs.File
	)
	if cfg.Data != nil {
		// use the provided favicon data
		iconData = cfg.Data
		iconLenHeader = strconv.Itoa(len(cfg.Data))
		iconLen = len(cfg.Data)
	} else if cfg.File != "" {
		// read from configured filesystem if present
		if cfg.FileSystem != nil {
			f, err = cfg.FileSystem.Open(cfg.File)
			if err != nil {
				panic(err)
			}
			defer func() {
				_ = f.Close() //nolint:errcheck // not needed
			}()
			if iconData, err = readLimited(f, cfg.MaxBytes); err != nil {
				panic(err)
			}
		} else {
			f, err = os.Open(cfg.File)
			if err != nil {
				panic(err)
			}
			defer func() {
				_ = f.Close() //nolint:errcheck // not needed
			}()
			if iconData, err = readLimited(f, cfg.MaxBytes); err != nil {
				panic(err)
			}
		}

		iconLenHeader = strconv.Itoa(len(iconData))
		iconLen = len(iconData)
	}

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Only respond to favicon requests
		if c.Path() != cfg.URL {
			return c.Next()
		}

		// Only allow GET, HEAD and OPTIONS requests
		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			if c.Method() != fiber.MethodOptions {
				c.Status(fiber.StatusMethodNotAllowed)
			} else {
				c.Status(fiber.StatusOK)
			}
			c.Set(fiber.HeaderAllow, hAllow)
			c.Set(fiber.HeaderContentLength, hZero)
			return nil
		}

		// Serve cached favicon
		if iconLen > 0 {
			c.Set(fiber.HeaderContentLength, iconLenHeader)
			c.Set(fiber.HeaderContentType, hType)
			c.Set(fiber.HeaderCacheControl, cfg.CacheControl)
			return c.Status(fiber.StatusOK).Send(iconData)
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}

func readLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	limit := maxBytes + 1
	data, err := io.ReadAll(io.LimitReader(reader, limit))
	if err != nil {
		return nil, fmt.Errorf("favicon: read limited: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("favicon: file size exceeds max bytes %d", maxBytes)
	}
	return data, nil
}
