package fiber

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

type expressErrorMode uint8

const (
	expressErrorModeCollect expressErrorMode = iota + 1
	expressErrorModeResume
)

type expressErrorState struct {
	err          error
	handlerIndex int
	mode         expressErrorMode
}

var expressErrorStateKey = &struct{}{}

func getExpressErrorState(c Ctx) (*expressErrorState, *DefaultCtx) {
	dc, ok := c.(*DefaultCtx)
	if !ok {
		return nil, nil
	}
	if value := dc.RequestCtx().UserValue(expressErrorStateKey); value != nil {
		if state, ok := value.(*expressErrorState); ok {
			return state, dc
		}
	}
	return nil, dc
}

func collectExpressError(c Ctx, dc *DefaultCtx, handlerIndex int) error {
	if dc == nil {
		return c.Next()
	}
	state := &expressErrorState{
		mode:         expressErrorModeCollect,
		handlerIndex: handlerIndex,
	}
	dc.RequestCtx().SetUserValue(expressErrorStateKey, state)
	err := dc.Next()
	dc.RequestCtx().SetUserValue(expressErrorStateKey, nil)
	return err
}

func resumeExpressError(c Ctx, dc *DefaultCtx, handlerIndex int, err error) error {
	if dc == nil {
		return c.Next()
	}
	dc.setIndexHandler(handlerIndex)
	state := &expressErrorState{
		mode:         expressErrorModeResume,
		handlerIndex: handlerIndex,
		err:          err,
	}
	dc.RequestCtx().SetUserValue(expressErrorStateKey, state)
	nextErr := dc.Next()
	dc.RequestCtx().SetUserValue(expressErrorStateKey, nil)
	return nextErr
}

type expressErrorPrep struct {
	dc           *DefaultCtx
	err          error
	handlerIndex int
	skip         bool
}

func prepareExpressError(c Ctx) expressErrorPrep {
	state, dc := getExpressErrorState(c)
	prep := expressErrorPrep{dc: dc}

	if dc != nil {
		prep.handlerIndex = dc.indexHandler
		if state != nil {
			if state.mode == expressErrorModeCollect && dc.indexHandler > state.handlerIndex {
				prep.err = dc.Next()
				prep.skip = true
				return prep
			}
			if state.mode == expressErrorModeResume && dc.indexHandler > state.handlerIndex {
				prep.err = state.err
				return prep
			}
		}
	}

	prep.err = collectExpressError(c, dc, prep.handlerIndex)
	return prep
}

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
	case func(error, Req, Res) error, func(error, Req, Res), func(error, Req, Res, func() error) error, func(error, Req, Res, func() error), func(error, Req, Res, func()) error, func(error, Req, Res, func()): // (9)-(14) Express-style error handlers
		return adaptExpressErrorHandler(handler)
	case http.HandlerFunc, http.Handler, func(http.ResponseWriter, *http.Request): // (15)-(17) net/http handlers
		return adaptHTTPHandler(handler)
	case fasthttp.RequestHandler, func(*fasthttp.RequestCtx) error: // (18)-(19) fasthttp handlers
		return adaptFastHTTPHandler(handler)
	default: // (20) unsupported handler type
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

func adaptExpressErrorHandler(handler any) (Handler, bool) {
	switch h := handler.(type) {
	case func(error, Req, Res) error: // (9) Express-style error handler with error return
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			prep := prepareExpressError(c)
			if prep.skip {
				return prep.err
			}
			if prep.err == nil {
				return nil
			}
			return h(prep.err, c.Req(), c.Res())
		}, true
	case func(error, Req, Res): // (10) Express-style error handler
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			prep := prepareExpressError(c)
			if prep.skip {
				return prep.err
			}
			if prep.err == nil {
				return nil
			}
			h(prep.err, c.Req(), c.Res())
			return nil
		}, true
	case func(error, Req, Res, func() error) error: // (11) Express-style error handler with error-returning next callback and error return
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			prep := prepareExpressError(c)
			if prep.skip {
				return prep.err
			}
			if prep.err == nil {
				return nil
			}
			nextCalled := false
			var nextErr error
			handlerErr := h(prep.err, c.Req(), c.Res(), func() error {
				nextCalled = true
				nextErr = resumeExpressError(c, prep.dc, prep.handlerIndex, prep.err)
				return nextErr
			})
			if handlerErr != nil {
				return handlerErr
			}
			if nextCalled {
				return nextErr
			}
			return nil
		}, true
	case func(error, Req, Res, func() error): // (12) Express-style error handler with error-returning next callback
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			prep := prepareExpressError(c)
			if prep.skip {
				return prep.err
			}
			if prep.err == nil {
				return nil
			}
			nextCalled := false
			var nextErr error
			h(prep.err, c.Req(), c.Res(), func() error {
				nextCalled = true
				nextErr = resumeExpressError(c, prep.dc, prep.handlerIndex, prep.err)
				return nextErr
			})
			if nextCalled {
				return nextErr
			}
			return nil
		}, true
	case func(error, Req, Res, func()) error: // (13) Express-style error handler with no-arg next callback and error return
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			prep := prepareExpressError(c)
			if prep.skip {
				return prep.err
			}
			if prep.err == nil {
				return nil
			}
			nextCalled := false
			handlerErr := h(prep.err, c.Req(), c.Res(), func() {
				nextCalled = true
			})
			if handlerErr != nil {
				return handlerErr
			}
			if nextCalled {
				return resumeExpressError(c, prep.dc, prep.handlerIndex, prep.err)
			}
			return nil
		}, true
	case func(error, Req, Res, func()): // (14) Express-style error handler with no-arg next callback
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			prep := prepareExpressError(c)
			if prep.skip {
				return prep.err
			}
			if prep.err == nil {
				return nil
			}
			nextCalled := false
			h(prep.err, c.Req(), c.Res(), func() {
				nextCalled = true
			})
			if nextCalled {
				return resumeExpressError(c, prep.dc, prep.handlerIndex, prep.err)
			}
			return nil
		}, true
	default:
		return nil, false
	}
}

func adaptHTTPHandler(handler any) (Handler, bool) {
	switch h := handler.(type) {
	case http.HandlerFunc: // (15) net/http HandlerFunc
		if h == nil {
			return nil, false
		}
		return wrapHTTPHandler(h), true
	case http.Handler: // (16) net/http Handler implementation
		if h == nil {
			return nil, false
		}
		hv := reflect.ValueOf(h)
		if hv.Kind() == reflect.Pointer && hv.IsNil() {
			return nil, false
		}
		return wrapHTTPHandler(h), true
	case func(http.ResponseWriter, *http.Request): // (17) net/http function handler
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
	case fasthttp.RequestHandler: // (18) fasthttp handler
		if h == nil {
			return nil, false
		}
		return func(c Ctx) error {
			h(c.RequestCtx())
			return nil
		}, true
	case func(*fasthttp.RequestCtx) error: // (19) fasthttp handler with error return
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
