package logger

import (
	"github.com/gofiber/fiber/v3"
	fiberlog "github.com/gofiber/fiber/v3/log"
)

// fiberMiddlewareTags lists the context tag names that Fiber's built-in
// middlewares register through RegisterContextTag. They are pre-registered
// here as no-op stubs so a logger format that references e.g. ${api-key}
// compiles even when the corresponding middleware (here keyauth) has not yet
// been initialized — the slot is filled in once the middleware's New() runs.
//
// The names are sourced from the canonical fiberlog tag constants so that
// renames in one place cascade automatically.
var fiberMiddlewareTags = []string{
	fiberlog.TagAPIKey,
	fiberlog.TagCSRFToken,
	fiberlog.TagRequestIDDashed,
	fiberlog.TagRequestID,
	fiberlog.TagSessionID,
	fiberlog.TagUsername,
}

func init() {
	for _, name := range fiberMiddlewareTags {
		registeredTags.m[name] = emptyLogTag
	}
}

func emptyLogTag(_ Buffer, _ fiber.Ctx, _ *Data, _ string) (int, error) {
	return 0, nil
}

// RegisterContextTag registers a string-valued tag in both the logger
// middleware tag registry and the package-level fiberlog context tag
// registry, so that the same name can be used in a logger.Config.Format and
// in fiberlog.SetContextTemplate. extract receives the raw context value
// (fiber.Ctx, *fasthttp.RequestCtx, or context.Context) and returns the
// rendered string; an empty return renders nothing.
//
// Re-registering a name replaces both renderers. Panics if name is empty or
// extract is nil — registration is expected at init time.
func RegisterContextTag(name string, extract func(ctx any) string) {
	if name == "" || extract == nil {
		panic("logger: RegisterContextTag requires a non-empty name and extractor")
	}

	fiberlog.MustRegisterContextTag(name, func(output fiberlog.Buffer, ctx any, _ *fiberlog.ContextData, _ string) (int, error) {
		v := extract(ctx)
		if v == "" {
			return 0, nil
		}
		return output.WriteString(v)
	})

	MustRegisterTag(name, func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
		v := extract(c)
		if v == "" {
			return 0, nil
		}
		return output.WriteString(v)
	})
}
