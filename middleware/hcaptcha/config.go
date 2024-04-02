package hcaptcha

import (
	"bytes"
	"encoding/json"
	"github.com/gofiber/fiber/v3"
)

const DefaultSiteVerifyURL = "https://api.hcaptcha.com/siteverify"

type Config struct {
	// SecretKey is the secret key you get from HCaptcha when you create a new application
	SecretKey string
	// ResponseKeyFunc should return the generated pass UUID from the ctx, which will be validated
	ResponseKeyFunc func(fiber.Ctx) (string, error)
	// SiteVerifyURL is the endpoint URL where the program should verify the given token
	// default value is: "https://api.hcaptcha.com/siteverify"
	SiteVerifyURL string
}

func DefaultResponseKeyFunc(c fiber.Ctx) (string, error) {
	data := struct {
		HCaptchaToken string `json:"hcaptcha_token"`
	}{}

	err := json.NewDecoder(bytes.NewReader(c.Body())).Decode(&data)

	if err != nil {
		return "", err
	}

	return data.HCaptchaToken, nil
}
