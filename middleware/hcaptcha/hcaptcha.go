// Package hcaptcha is a simple middleware that checks for an HCaptcha UUID
// and then validates it. It returns an error if the UUID is not valid (the request may have been sent by a robot).
package hcaptcha

import (
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v3"
	"net/http"
	"net/url"
)

type HCaptcha struct {
	Config
}

func New(config Config) fiber.Handler {
	if config.SiteVerifyURL == "" {
		config.SiteVerifyURL = DefaultSiteVerifyURL
	}

	if config.ResponseKeyFunc == nil {
		config.ResponseKeyFunc = DefaultResponseKeyFunc
	}

	h := &HCaptcha{
		config,
	}
	return h.Validate
}

func (h *HCaptcha) Validate(c fiber.Ctx) error {
	token, err := h.ResponseKeyFunc(c)
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return err
	}

	res, err := http.PostForm(h.SiteVerifyURL, url.Values{
		"secret":   {h.SecretKey},
		"response": {token},
	})

	if err != nil {
		c.Status(fiber.StatusBadRequest)
		return err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	o := struct {
		Success bool `json:"success"`
	}{}

	if err = json.NewDecoder(res.Body).Decode(&o); err != nil {
		c.Status(fiber.StatusInternalServerError)
		return err
	}

	if !o.Success {
		c.Status(fiber.StatusForbidden)
		return errors.New("unable to check that you are not a robot")
	}

	return c.Next()
}
