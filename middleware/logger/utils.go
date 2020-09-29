package logger

import (
	"github.com/gofiber/fiber/v2"
)

func methodColor(method string) string {
	switch method {
	case fiber.MethodGet:
		return cCyan
	case fiber.MethodPost:
		return cGreen
	case fiber.MethodPut:
		return cYellow
	case fiber.MethodDelete:
		return cRed
	case fiber.MethodPatch:
		return cWhite
	case fiber.MethodHead:
		return cMagenta
	case fiber.MethodOptions:
		return cBlue
	default:
		return cReset
	}
}

func statusColor(code int) string {
	switch {
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
