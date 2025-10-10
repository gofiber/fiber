package fiber

import (
	"fmt"
	"net/http"

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
		return wrapHTTPHandler(h), true
	case http.Handler:
		return wrapHTTPHandler(h), true
	case func(http.ResponseWriter, *http.Request):
		return wrapHTTPHandler(http.HandlerFunc(h)), true
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

	for _, arg := range args {
		handler, ok := convertHandler(arg)
		if !ok {
			panic(fmt.Sprintf("%s: invalid handler %T\n", context, arg))
		}
		handlers = append(handlers, handler)
	}

	return handlers
}

// convertHandler normalizes a single handler argument into a Fiber handler.
func convertHandler(arg any) (Handler, bool) {
	switch h := arg.(type) {
	case nil:
		return nil, false
	default:
		return toFiberHandler(h)
	}
}
