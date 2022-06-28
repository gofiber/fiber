package fiber

import (
	"bytes"
	"context"
	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type contextKey int

const (
	ContextKey = contextKey(1)
	valuesKey  = "sentry"
)

type Handler struct {
	rePanic         bool
	waitForDelivery bool
	timeOut         time.Duration
}

type Options struct {
	// RePanic configures whether Sentry should repanic after recovery.
	// In most cases, it should be set to false,
	// since fiber does not provide a Recovery handler.
	RePanic bool
	// WaitForDelivery specifies whether to block the request before moving forward with the response.
	WaitForDelivery bool
	// TimeOut for the event delivery requests.
	TimeOut time.Duration
}

// New returns a struct that provides Handle method
// that satisfy fiber.Handler interface.
func New(opt Options) *Handler {
	timeOut := opt.TimeOut
	if timeOut == 0 {
		timeOut = 2 * time.Second
	}
	return &Handler{
		rePanic:         opt.RePanic,
		waitForDelivery: opt.WaitForDelivery,
		timeOut:         timeOut,
	}
}

// GetHubFromContext retrieves attached *sentry.Hub instance from fiber.Ctx
func GetHubFromContext(ctx *fiber.Ctx) *sentry.Hub {
	hub := ctx.Context().Value(valuesKey)
	if hub, ok := hub.(*sentry.Hub); ok {
		return hub
	}
	return nil
}

// Handle wraps fiber.Handler and recovers from caught panics.
func (h *Handler) Handle(handler fiber.Handler) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		hub := sentry.CurrentHub().Clone()
		scope := hub.Scope()
		scope.SetRequest(ctxConvertor(ctx))
		scope.SetRequestBody(ctx.Request().Body())
		defer h.recoveryWithSentry(hub, ctx)
		if err := handler(ctx); err != nil {
			return err
		}
		return nil
	}
}

func (h *Handler) recoveryWithSentry(hub *sentry.Hub, ctx *fiber.Ctx) {
	if err := recover(); err != nil {
		eventID := hub.RecoverWithContext(
			context.WithValue(context.Background(), sentry.RequestContextKey, ctx),
			err,
		)
		if eventID != nil && h.waitForDelivery {
			hub.Flush(h.timeOut)
		}
		if h.rePanic {
			panic(err)
		}
	}
}

func ctxConvertor(ctx *fiber.Ctx) *http.Request {
	defer func() {
		if err := recover(); err != nil {
			sentry.Logger.Printf("%v", err)
		}
	}()

	r := new(http.Request)
	r.Method = ctx.Method()
	r.URL, _ = url.Parse(ctx.Request().URI().String())
	r.Header = make(http.Header)
	r.Header.Add("Host", ctx.Hostname())
	ctx.Request().Header.VisitAll(func(key, value []byte) {
		r.Header.Add(string(key), string(value))
	})
	r.Host = ctx.Hostname()

	ctx.Request().Header.VisitAllCookie(func(key, value []byte) {
		r.AddCookie(&http.Cookie{Name: string(key), Value: string(value)})
	})

	r.RemoteAddr = ctx.IP()
	r.URL.RawQuery = string(ctx.Request().URI().QueryString())
	r.Body = ioutil.NopCloser(bytes.NewReader(ctx.Request().Body()))
	return r
}
