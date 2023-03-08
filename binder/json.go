package binder

import (
	"github.com/gofiber/utils/v2"
)

type jsonBinding struct{}

func (*jsonBinding) Name() string {
	return "json"
}

func (*jsonBinding) Bind(body []byte, jsonDecoder utils.JSONUnmarshal, out any) error {
	return jsonDecoder(body, out)
}
