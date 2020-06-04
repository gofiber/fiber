package middleware

import (
	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
)

// RequestID adds an UUID indentifier to the request
func RequestID() fiber.Handler {
	return func(ctx *fiber.Ctx) {
		// Get id from request
		rid := ctx.Get(fiber.HeaderXRequestID)
		// Create new UUID if empty
		if len(rid) <= 0 {
			rid = utils.UUID()
		}
		// Set new id to response
		ctx.Set(fiber.HeaderXRequestID, rid)
		// Continue stack
		ctx.Next()
	}
}
