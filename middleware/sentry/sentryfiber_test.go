package fiber

import (
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"strings"
	"testing"
)

func TestIntegration(t *testing.T) {
	largePayLoad := strings.Repeat("Large", 3*1024)
	app := fiber.New()
	tests := []struct {
		Path      string
		Method    string
		Body      string
		Handler   fiber.Handler
		WantEvent *sentry.Event
	}{
		{
			Path: "/panic",
			Handler: func(ctx *fiber.Ctx) error {
				panic("test panic")
			},
			WantEvent: &sentry.Event{
				Level:   sentry.LevelFatal,
				Message: "test panic",
				Request: &sentry.Request{
					URL:    "http://example.com/panic",
					Method: "GET",
					Headers: map[string]string{
						"Host":       "example.com",
						"User-Agent": "fiber",
					},
				},
			},
		},
		{
			Path:   "/post",
			Method: "POST",
			Body:   "payload",
			Handler: func(ctx *fiber.Ctx) error {
				hub := GetHubFromContext(ctx)
				hub.CaptureMessage("post: " + string(ctx.Request().Body()))
				return nil
			},
			WantEvent: &sentry.Event{
				Level:   sentry.LevelInfo,
				Message: "post: payload",
				Request: &sentry.Request{
					URL:    "http://example.com/post",
					Method: "POST",
					Data:   "payload data",
					Headers: map[string]string{
						"Host":       "example.com",
						"User-Agent": "fiber",
					},
				},
			},
		},
		{
			Path: "/get",
			Handler: func(ctx *fiber.Ctx) error {
				hub := GetHubFromContext(ctx)
				hub.CaptureMessage("get")
				return nil
			},
			WantEvent: &sentry.Event{
				Level:   sentry.LevelInfo,
				Message: "get",
				Request: &sentry.Request{
					URL:    "http://example.com/get",
					Method: "GET",
					Headers: map[string]string{
						"Host":       "example.com",
						"User-Agent": "fiber",
					},
				},
			},
		},
		{
			Path:   "/post/large",
			Method: "POST",
			Body:   largePayLoad,
			Handler: func(ctx *fiber.Ctx) error {
				hub := GetHubFromContext(ctx)
				hub.CaptureMessage(fmt.Sprintf("post: %d KB", len(ctx.Request().Body())/1024))
				return nil
			},
			WantEvent: &sentry.Event{
				Level:   sentry.LevelInfo,
				Message: "post: 15 kb",
				Request: &sentry.Request{
					URL:    "http://example.com/post/large",
					Method: "POST",
					Data:   "",
					Headers: map[string]string{
						"Host":       "example.com",
						"User-Agent": "fiber",
					},
				},
			},
		},
		{
			Path:   "/post/body-ignored",
			Method: "POST",
			Handler: func(ctx *fiber.Ctx) error {
				hub := GetHubFromContext(ctx)
				hub.CaptureMessage("body ignored")
				return nil
			},
			WantEvent: &sentry.Event{
				Level:   sentry.LevelInfo,
				Message: "body ignored",
				Request: &sentry.Request{
					URL:    "http://example.com/post/body-ignored",
					Method: "POST",
					Data:   "client sends, fiber always reads",
					Headers: map[string]string{
						"Host":       "example.com",
						"User-Agent": "fiber",
					},
				},
			},
		},
	}
	eventCh := make(chan *sentry.Event, len(tests))
	if err := sentry.Init(sentry.ClientOptions{
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			eventCh <- event
			return event
		},
	}); err != nil {
		t.Fatal(err)
	}

	sentryHandler := New(Options{})
	handler := func(ctx *fiber.Ctx) error {
		for _, tt := range tests {
			if ctx.Path() == tt.Path {
				if err := tt.Handler(ctx); err != nil {
					return err
				}
				return nil
			}
		}
		t.Errorf("unhandled request: %#v", ctx)
		return nil
	}
	app.Use(sentryHandler.Handle(handler))
	if err := app.Listen(":3000"); err != nil {
		t.Error(err)
	}
}
