package fiber

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// toFiberHandler converts a supported handler type to a Fiber handler.
func toFiberHandler(handler any) (Handler, bool) {
	if handler == nil {
		return nil, false
	}

	switch handler.(type) {
	case Handler, func(Ctx): // (1)-(2) Fiber handlers
		return adaptFiberHandler(handler)
	case func(Req, Res) error, func(Req, Res), func(Req, Res, func() error) error, func(Req, Res, func() error), func(Req, Res, func()) error, func(Req, Res, func()): // (3)-(8) Express-style request handlers
		return adaptExpressHandler(handler)
	case http.HandlerFunc, http.Handler, func(http.ResponseWriter, *http.Request): // (9)-(11) net/http handlers
		return adaptHTTPHandler(handler)
	case fasthttp.RequestHandler, func(*fasthttp.RequestCtx) error: // (12)-(13) fasthttp handlers
		return adaptFastHTTPHandler(handler)
	default: // (14) unsupported handler type
		return nil, false
	}
}

func adaptFiberHandler(handler any) (Handler, bool) {
	switch h := handler.(type) {
	case Handler: // (1) direct Fiber handler
		if h == nil {
			return nil, false
		}
		return h, true
	case func(Ctx): // (2) Fiber handler without error return
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			h(c)
			return nil
		}, true
	default:
		return nil, false
	}
}

func adaptExpressHandler(handler any) (Handler, bool) {
	switch h := handler.(type) {
	case func(Req, Res) error: // (3) Express-style handler with error return
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			return h(c.Req(), c.Res())
		}, true
	case func(Req, Res): // (4) Express-style handler without error return
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			h(c.Req(), c.Res())
			return nil
		}, true
	case func(Req, Res, func() error) error: // (5) Express-style handler with error-returning next callback and error return
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			return h(c.Req(), c.Res(), func() error {
				return c.Next()
			})
		}, true
	case func(Req, Res, func() error): // (6) Express-style handler with error-returning next callback
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			var nextErr error
			h(c.Req(), c.Res(), func() error {
				nextErr = c.Next()
				return nextErr
			})
			return nextErr
		}, true
	case func(Req, Res, func()) error: // (7) Express-style handler with no-arg next callback and error return
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			var nextErr error
			err := h(c.Req(), c.Res(), func() {
				nextErr = c.Next()
			})
			if err != nil {
				return err
			}
			return nextErr
		}, true
	case func(Req, Res, func()): // (8) Express-style handler with no-arg next callback
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			var nextErr error
			h(c.Req(), c.Res(), func() {
				nextErr = c.Next()
			})
			return nextErr
		}, true
	default:
		return nil, false
	}
}

func adaptHTTPHandler(handler any) (Handler, bool) {
	switch h := handler.(type) {
	case http.HandlerFunc: // (9) net/http HandlerFunc
		if h == nil {
			return nil, false
		}
		return wrapHTTPHandler(h), true
	case http.Handler: // (10) net/http Handler implementation
		if h == nil {
			return nil, false
		}
		hv := reflect.ValueOf(h)
		if hv.Kind() == reflect.Pointer && hv.IsNil() {
			return nil, false
		}
		return wrapHTTPHandler(h), true
	case func(http.ResponseWriter, *http.Request): // (11) net/http function handler
		if h == nil {
			return nil, false
		}
		return wrapHTTPHandler(http.HandlerFunc(h)), true
	default:
		return nil, false
	}
}

func adaptFastHTTPHandler(handler any) (Handler, bool) {
	switch h := handler.(type) {
	case fasthttp.RequestHandler: // (12) fasthttp handler
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			h(c.RequestCtx())
			return nil
		}, true
	case func(*fasthttp.RequestCtx) error: // (13) fasthttp handler with error return
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			return h(c.RequestCtx())
		}, true
	default:
		return nil, false
	}
}

// wrapHTTPHandler adapts a net/http handler to a Fiber handler.
func wrapHTTPHandler(handler http.Handler) Handler {
	if handler == nil {
		return nil
	}

	adapted := fasthttpadaptor.NewFastHTTPHandler(handler)

	return func(c Ctx) error {
		adapted(c.RequestCtx())
		return nil
	}
}

// collectHandlers converts a slice of handler arguments to Fiber handlers.
// The context string is used to provide informative panic messages when an
// unsupported handler type is encountered.
func collectHandlers(context string, args ...any) []Handler {
	handlers := make([]Handler, 0, len(args))

	for i, arg := range args {
		handler, ok := toFiberHandler(arg)

		if !ok {
			panic(fmt.Sprintf("%s: invalid handler #%d (%T)\n", context, i, arg))
		}
		handlers = append(handlers, handler)
	}

	return handlers
}
