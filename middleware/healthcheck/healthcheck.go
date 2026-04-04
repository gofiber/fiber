package healthcheck

import (
	"github.com/gofiber/fiber/v3"
)

// healthResponse represents the JSON/XML/MsgPack/CBOR response structure.
type healthResponse struct {
	Status string `json:"status" xml:"status" msg:"status"`
}

// New returns a health-check handler that responds based on the provided
// configuration.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if c.Method() != fiber.MethodGet {
			return c.Next()
		}

		healthy := cfg.Probe(c)
		statusCode := fiber.StatusOK
		statusMessage := "OK"

		if !healthy {
			statusCode = fiber.StatusServiceUnavailable
			statusMessage = "Service Unavailable"
		}

		// Set the status code
		c.Status(statusCode)

		// Return response based on configured format
		switch cfg.ResponseFormat {
		case ResponseFormatJSON:
			return c.JSON(healthResponse{Status: statusMessage})
		case ResponseFormatXML:
			return c.XML(healthResponse{Status: statusMessage})
		case ResponseFormatMsgPack:
			return c.MsgPack(healthResponse{Status: statusMessage})
		case ResponseFormatCBOR:
			return c.CBOR(healthResponse{Status: statusMessage})
		default: // ResponseFormatText
			return c.SendString(statusMessage)
		}
	}
}
