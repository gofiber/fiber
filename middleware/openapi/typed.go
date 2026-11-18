package openapi

import (
	"reflect"
	"slices"

	"github.com/gofiber/fiber/v3"
)

// Empty is the response type of a typed route that answers with no body.
type Empty struct{}

// TypedOption configures [Typed].
type TypedOption func(*typedConfig)

type typedConfig struct {
	middleware []any
	status     int
}

// WithStatus sets the status a typed route answers with and documents. Default: 200, or 204 for an [Empty] response.
func WithStatus(status int) TypedOption {
	return func(c *typedConfig) { c.status = status }
}

// WithMiddleware runs handlers before a typed route's handler, in order.
func WithMiddleware(handlers ...any) TypedOption {
	return func(c *typedConfig) { c.middleware = append(c.middleware, handlers...) }
}

// Typed registers a route whose handler takes its request as a struct and returns its response as a struct, and
// documents the route from those two types instead of from calls you repeat for each route: the parameters
// from the fields tagged `uri`, `query`, `header` and `cookie`, the request body from the fields bound from the
// body (`json`, `form`, `xml` or untagged), and the success response from Res. The handler gets the request
// bound with c.Bind().All, and its result is written as JSON. A fields tagged only for a parameter is not part
// of the body schema.
//
// Handlers are not inspected for their types, so a plain handler has to be documented with Accepts, Returns and
// Params; Typed is the registration that does it for you. It returns the route, to document it further:
//
//	openapi.Typed(app, fiber.MethodPost, "/users", createUser, openapi.WithStatus(fiber.StatusCreated)).
//	    Summary("Create a user")
func Typed[Req, Res any](router fiber.Router, method, path string, handler func(c fiber.Ctx, req *Req) (*Res, error), options ...TypedOption) fiber.Router {
	cfg := typedConfig{}
	for _, option := range options {
		option(&cfg)
	}
	empty := reflect.TypeFor[Res]() == reflect.TypeFor[Empty]()
	if cfg.status == 0 {
		cfg.status = fiber.StatusOK
		if empty {
			cfg.status = fiber.StatusNoContent
		}
	}

	wrapped := func(c fiber.Ctx) error {
		req := new(Req)
		if err := c.Bind().All(req); err != nil {
			return err
		}
		res, err := handler(c, req)
		if err != nil {
			return err
		}
		if empty || res == nil {
			return c.SendStatus(cfg.status)
		}
		return c.Status(cfg.status).JSON(res)
	}

	handlers := append(slices.Clone(cfg.middleware), wrapped)
	route := router.Add([]string{method}, path, handlers[0], handlers[1:]...)
	documentTyped(route, method, reflect.TypeFor[Req](), reflect.TypeFor[Res](), cfg.status)
	return route
}

// documentTyped documents a route from its request and response types.
func documentTyped(route fiber.Router, method string, req, res reflect.Type, status int) {
	if reqType := derefType(req); reqType.Kind() == reflect.Struct {
		model := reflect.New(reqType).Interface()
		for location, tag := range modelTagByLocation {
			if hasTaggedField(reqType, tag) {
				route.Params(location, model)
			}
		}
		if hasBodyFields(reqType) && !isBodylessMethod(method) {
			route.Accepts(model)
		}
	}
	if res == reflect.TypeFor[Empty]() {
		route.Response(status, "")
		return
	}
	route.Returns(status, reflect.New(derefType(res)).Interface())
}

func isBodylessMethod(method string) bool {
	switch method {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodTrace, fiber.MethodOptions, fiber.MethodConnect:
		return true
	default:
		return false
	}
}

// hasTaggedField reports whether a struct, or one it embeds, has an exported field with the tag.
func hasTaggedField(t reflect.Type, tag string) bool {
	return anyField(t, map[reflect.Type]bool{}, func(field *reflect.StructField) bool { return field.Tag.Get(tag) != "" })
}

// hasBodyFields reports whether a struct has a field bound from the body: one with a body tag, or one with no
// tag for another location.
func hasBodyFields(t reflect.Type) bool {
	return anyField(t, map[reflect.Type]bool{}, func(field *reflect.StructField) bool {
		if field.Tag.Get(tagJSON) == "-" {
			return false
		}
		for _, tag := range [...]string{tagJSON, tagForm, tagXML} {
			if field.Tag.Get(tag) != "" {
				return true
			}
		}
		return !taggedForAnotherLocation(field, tagJSON)
	})
}

func anyField(t reflect.Type, seen map[reflect.Type]bool, match func(*reflect.StructField) bool) bool {
	if t.Kind() != reflect.Struct || seen[t] {
		return false
	}
	seen[t] = true
	for i := range t.NumField() {
		field := t.Field(i)
		if field.Anonymous && derefType(field.Type).Kind() == reflect.Struct && field.Tag.Get(tagJSON) == "" {
			if anyField(derefType(field.Type), seen, match) {
				return true
			}
			continue
		}
		if field.IsExported() && match(&field) {
			return true
		}
	}
	return false
}
