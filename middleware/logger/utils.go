package logger

import (
	"github.com/gofiber/fiber/v2"
)

func methodColor(method string, colors fiber.Colors) string {
	switch method {
	case fiber.MethodGet:
		return colors.Cyan
	case fiber.MethodPost:
		return colors.Green
	case fiber.MethodPut:
		return colors.Yellow
	case fiber.MethodDelete:
		return colors.Red
	case fiber.MethodPatch:
		return colors.White
	case fiber.MethodHead:
		return colors.Magenta
	case fiber.MethodOptions:
		return colors.Blue
	default:
		return colors.Reset
	}
}

func statusColor(code int, colors fiber.Colors) string {
	switch {
	case code >= fiber.StatusOK && code < fiber.StatusMultipleChoices:
		return colors.Green
	case code >= fiber.StatusMultipleChoices && code < fiber.StatusBadRequest:
		return colors.Blue
	case code >= fiber.StatusBadRequest && code < fiber.StatusInternalServerError:
		return colors.Yellow
	default:
		return colors.Red
	}
}
