package binder

import (
	"github.com/gofiber/fiber/v3/utils"
)

type jsonBinding struct{}

func (*jsonBinding) Name() string {
	return "json"
}

func (b *jsonBinding) Bind(body []byte, jsonDecoder utils.JSONUnmarshal, out any) error {
	return jsonDecoder(body, out)
}
