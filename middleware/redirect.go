package middleware

import (
	"github.com/gofiber/fiber"
)

// Usage
// app.Use(middleware.HTTPSRedirect())

// HTTPSRedirect ...
func HTTPSRedirect(code ...int) func(*fiber.Ctx) {
	redirectCode := 301
	if len(code) > 0 {
		redirectCode = code[0]
	}
	return func(c *fiber.Ctx) {
		if c.Protocol() != "https" {
			c.Redirect("https://"+c.Hostname()+c.OriginalURL(), redirectCode)
			return
		}
		c.Next()
	}
}

//HTTPSWWWRedirect ...
func HTTPSWWWRedirect(code ...int) func(*fiber.Ctx) {
	redirectCode := 301
	if len(code) > 0 {
		redirectCode = code[0]
	}
	return func(c *fiber.Ctx) {
		if c.Protocol() != "https" && c.Hostname()[:4] != "www." {
			c.Redirect("https://www."+c.Hostname()+c.OriginalURL(), redirectCode)
			return
		}
		c.Next()
	}
}

//HTTPSNonWWWRedirect ...
func HTTPSNonWWWRedirect(code ...int) func(*fiber.Ctx) {
	redirectCode := 301
	if len(code) > 0 {
		redirectCode = code[0]
	}
	return func(c *fiber.Ctx) {
		if c.Protocol() != "https" {
			host := c.Hostname()
			if host[:4] == "www." {
				host = host[4:]
			}
			c.Redirect("https://"+host+c.OriginalURL(), redirectCode)
			return
		}
		c.Next()
	}
}

//WWWRedirect ...
func WWWRedirect(code ...int) func(*fiber.Ctx) {
	redirectCode := 301
	if len(code) > 0 {
		redirectCode = code[0]
	}
	return func(c *fiber.Ctx) {
		if c.Hostname()[:4] != "www." {
			c.Redirect("http://www."+c.Hostname()+c.OriginalURL(), redirectCode)
			return
		}
		c.Next()
	}
}

//NonWWWRedirect ...
func NonWWWRedirect(code ...int) func(*fiber.Ctx) {
	redirectCode := 301
	if len(code) > 0 {
		redirectCode = code[0]
	}
	return func(c *fiber.Ctx) {
		if c.Hostname()[:4] == "www." {
			c.Redirect("http://"+c.Hostname()[4:]+c.OriginalURL(), redirectCode)
			return
		}
		c.Next()
	}
}
