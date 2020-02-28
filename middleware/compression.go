package middleware

import "github.com/gofiber/fiber"

// Compression ...
func Compression(level ...int) func(*fiber.Ctx) {

	// 1: CompressDefaultCompression
	// 2: CompressBestSpeed
	// 3: CompressBestCompression
	// 4: CompressHuffmanOnly
	var lvl = 1
	// Set compression level if provided
	if len(level) > 0 {
		lvl = level[0]
	}
	// Set config default values
	return func(c *fiber.Ctx) {
		c.Compress(lvl)
		c.Next()
	}
}
