package middleware

import (
	"io/ioutil"
	"strconv"

	"github.com/gofiber/fiber"
)

// Favicon adds an UUID indentifier to the request
func Favicon(file ...string) fiber.Handler {
	var err error
	var icon []byte

	// Set lookup if provided
	if len(file) > 0 {
		icon, err = ioutil.ReadFile(file[0])
		if err != nil {
			panic(err)
		}
	}
	// Return handler
	return func(c *fiber.Ctx) {
		if len(c.Path()) != 12 || c.Path() != "/favicon.ico" {
			c.Next()
			return
		}

		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			if c.Method() != fiber.MethodOptions {
				c.Status(405)
			} else {
				c.Status(200)
			}
			c.Set(fiber.HeaderAllow, "GET, HEAD, OPTIONS")
			c.Set(fiber.HeaderContentLength, "0")
			return
		}

		if len(icon) > 0 {
			c.Set(fiber.HeaderContentLength, strconv.Itoa(len(icon)))
			c.Set(fiber.HeaderContentType, "image/x-icon")
			c.Set(fiber.HeaderCacheControl, "public, max-age=31536000")
			c.Status(200).SendBytes(icon)
			return
		}

		c.Status(204)
	}
}
