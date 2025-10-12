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
	switch h := handler.(type) {
	case Handler:
		if h == nil {
			return nil, false
		}
		return h, true
	case http.HandlerFunc:
		if h == nil {
			return nil, false
		}
		return wrapHTTPHandler(h), true
	case http.Handler:
		if h == nil {
			return nil, false
		}
		hv := reflect.ValueOf(h)
		if hv.Kind() == reflect.Pointer && hv.IsNil() {
			return nil, false
		}
		return wrapHTTPHandler(h), true
	case func(http.ResponseWriter, *http.Request):
		if h == nil {
			return nil, false
		}
		return wrapHTTPHandler(http.HandlerFunc(h)), true
	case fasthttp.RequestHandler:
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			h(c.RequestCtx())
			return nil
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
		var (
			handler Handler
			ok      bool
		)

		switch h := arg.(type) {
		case nil:
			handler, ok = nil, false
		default:
			handler, ok = toFiberHandler(h)
		}

		if !ok {
			panic(fmt.Sprintf("%s: invalid handler #%d (%T)\n", context, i, arg))
		}
		handlers = append(handlers, handler)
	}

	return handlers
}
