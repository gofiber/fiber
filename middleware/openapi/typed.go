package openapi

import (
	"errors"
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
// body (`json`, `form`, `xml` or untagged), and the success response from Res. The request is bound as it is
// documented: a field is filled from the query, the path, a header or a cookie only when it carries that tag, and
// a field tagged only for a parameter is neither part of the body schema nor filled from the body. That is
// stricter than c.Bind().All, which also matches a field by its Go name in every source. A request that does
// not bind or validate is answered with 400. The result is written as JSON.
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

	binding := newTypedBinding(reflect.TypeFor[Req](), method)
	wrapped := func(c fiber.Ctx) error {
		req := new(Req)
		if err := binding.bind(c, req); err != nil {
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
		for _, location := range [...]fiber.ParamLocation{fiber.ParamInPath, fiber.ParamInQuery, fiber.ParamInHeader, fiber.ParamInCookie} {
			if hasTaggedField(reqType, modelTagByLocation[location]) {
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

// typedBinding is how a typed route fills its request: the locations that have tagged fields, and the body.
type typedBinding struct {
	locations []fiber.ParamLocation
	body      bool
}

func newTypedBinding(t reflect.Type, method string) *typedBinding {
	binding := &typedBinding{}
	t = derefType(t)
	if t.Kind() != reflect.Struct {
		return binding
	}
	for _, location := range typedLocations {
		if hasTaggedField(t, modelTagByLocation[location]) {
			binding.locations = append(binding.locations, location)
		}
	}
	binding.body = hasBodyFields(t) && !isBodylessMethod(method)
	return binding
}

var typedLocations = [...]fiber.ParamLocation{fiber.ParamInPath, fiber.ParamInQuery, fiber.ParamInHeader, fiber.ParamInCookie}

// bodyTags are the tags of the body decoders.
var bodyTags = [...]string{tagJSON, tagForm, tagXML, "cbor", "msgpack"}

// bind fills req from the request. Fiber's binders match an untagged field by its Go name, in every source, so
// each source is bound into a scratch value and only the fields tagged for it are taken over: a client cannot
// set a body field from a header, a cookie or the query, nor a parameter from the body.
func (b *typedBinding) bind(c fiber.Ctx, req any) error {
	dst := reflect.ValueOf(req).Elem()
	if dst.Kind() != reflect.Struct {
		return nil
	}
	binder := c.Bind().SkipValidation(true)
	defer binder.SkipValidation(false)

	if b.body && len(c.Request().Body()) > 0 {
		if err := binder.Body(req); err != nil {
			return typedBadRequest(err)
		}
		clearParameterFields(dst)
	}
	for _, location := range b.locations {
		scratch := reflect.New(dst.Type())
		var err error
		switch location {
		case fiber.ParamInPath:
			err = binder.URI(scratch.Interface())
		case fiber.ParamInQuery:
			err = binder.Query(scratch.Interface())
		case fiber.ParamInHeader:
			err = binder.Header(scratch.Interface())
		case fiber.ParamInCookie:
			err = binder.Cookie(scratch.Interface())
		default:
		}
		if err != nil {
			return typedBadRequest(err)
		}
		takeTagged(dst, scratch.Elem(), modelTagByLocation[location])
	}

	if validator := c.App().Config().StructValidator; validator != nil {
		if err := validator.Validate(req); err != nil {
			return typedBadRequest(err)
		}
	}
	return nil
}

func typedBadRequest(err error) error {
	if _, ok := errors.AsType[*fiber.Error](err); ok {
		return err
	}
	return fiber.NewError(fiber.StatusBadRequest, "Bad request: "+err.Error())
}

func hasBodyTag(field *reflect.StructField) bool {
	for _, tag := range bodyTags {
		if value := field.Tag.Get(tag); value != "" && value != "-" {
			return true
		}
	}
	return false
}

// embeddedStruct returns the struct an embedded field holds; a nil pointer holds none.
func embeddedStruct(field *reflect.StructField, value reflect.Value) (reflect.Value, bool) {
	if !field.Anonymous || derefType(field.Type).Kind() != reflect.Struct || field.Tag.Get(tagJSON) != "" {
		return reflect.Value{}, false
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, false
		}
		value = value.Elem()
	}
	return value, value.Kind() == reflect.Struct
}

// clearParameterFields zeroes what a body decoder put into the fields that are parameters only.
func clearParameterFields(dst reflect.Value) {
	t := dst.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		if inner, ok := embeddedStruct(&field, dst.Field(i)); ok {
			clearParameterFields(inner)
			continue
		}
		if field.IsExported() && !hasBodyTag(&field) && taggedForAnotherLocation(&field, tagJSON) {
			dst.Field(i).SetZero()
		}
	}
}

// takeTagged copies the fields tagged for a source from the value bound from it. A value already set, by the
// body or an earlier source, is kept.
func takeTagged(dst, src reflect.Value, tag string) {
	t := dst.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		if source, ok := embeddedStruct(&field, src.Field(i)); ok {
			target := dst.Field(i)
			if target.Kind() == reflect.Pointer && target.IsNil() && target.CanSet() && field.Type.Elem().Kind() == reflect.Struct {
				target.Set(reflect.New(field.Type.Elem()))
			}
			if inner, held := embeddedStruct(&field, target); held {
				takeTagged(inner, source, tag)
			}
			continue
		}
		if !field.IsExported() || field.Tag.Get(tag) == "" || field.Tag.Get(tag) == "-" {
			continue
		}
		if dst.Field(i).IsZero() && !src.Field(i).IsZero() {
			dst.Field(i).Set(src.Field(i))
		}
	}
}
