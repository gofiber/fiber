package logger

import (
	"github.com/gofiber/fiber/v2"
)

func methodColor(method string, config ...Config) string {
	cfg := ConfigDefault
	if len(config) >= 1 {
		cfg = config[0]
	}

	switch {
	case !cfg.enableColors:
		return ""
	case method == fiber.MethodGet:
		return cCyan
	case method == fiber.MethodPost:
		return cGreen
	case method == fiber.MethodPut:
		return cYellow
	case method == fiber.MethodDelete:
		return cRed
	case method == fiber.MethodPatch:
		return cWhite
	case method == fiber.MethodHead:
		return cMagenta
	case method == fiber.MethodOptions:
		return cBlue
	default:
		return cReset
	}
}

func statusColor(code int, config ...Config) string {
	cfg := ConfigDefault
	if len(config) >= 1 {
		cfg = config[0]
	}

	switch {
	case !cfg.enableColors:
		return ""
	case code >= fiber.StatusOK && code < fiber.StatusMultipleChoices:
		return cGreen
	case code >= fiber.StatusMultipleChoices && code < fiber.StatusBadRequest:
		return cBlue
	case code >= fiber.StatusBadRequest && code < fiber.StatusInternalServerError:
		return cYellow
	default:
		return cRed
	}
}
