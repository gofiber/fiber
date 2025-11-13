package fiber

import (
	"reflect"
	"slices"
	"sync"

	"github.com/gofiber/fiber/v3/binder"
	"github.com/gofiber/utils/v2"
)

// CustomBinder An interface to register custom binders.
type CustomBinder interface {
	Name() string
	MIMETypes() []string
	Parse(c Ctx, out any) error
}

// StructValidator is an interface to register custom struct validator for binding.
type StructValidator interface {
	Validate(out any) error
}

var bindPool = sync.Pool{
	New: func() any {
		return &Bind{
			dontHandleErrs: true,
		}
	},
}

// Bind provides helper methods for binding request data to Go values.
type Bind struct {
	ctx            Ctx
	dontHandleErrs bool
	skipValidation bool
}

// AcquireBind returns Bind reference from bind pool.
func AcquireBind() *Bind {
	b, ok := bindPool.Get().(*Bind)
	if !ok {
		panic(errBindPoolTypeAssertion)
	}

	return b
}

// ReleaseBind returns b acquired via Bind to bind pool.
func ReleaseBind(b *Bind) {
	b.release()
	bindPool.Put(b)
}

// releasePooledBinder resets a binder and returns it to its pool.
// It should be used with defer to ensure proper cleanup of pooled binders.
func releasePooledBinder[T interface{ Reset() }](pool *sync.Pool, bind T) {
	bind.Reset()
	binder.PutToThePool(pool, bind)
}

func (b *Bind) release() {
	b.ctx = nil
	b.dontHandleErrs = true
	b.skipValidation = false
}

// WithoutAutoHandling If you want to handle binder errors manually, you can use `WithoutAutoHandling`.
// It's default behavior of binder.
func (b *Bind) WithoutAutoHandling() *Bind {
	b.dontHandleErrs = true

	return b
}

// WithAutoHandling If you want to handle binder errors automatically, you can use `WithAutoHandling`.
// If there's an error, it will return the error and set HTTP status to `400 Bad Request`.
// You must still return on error explicitly
func (b *Bind) WithAutoHandling() *Bind {
	b.dontHandleErrs = false

	return b
}

// Check WithAutoHandling/WithoutAutoHandling errors and return it by usage.
func (b *Bind) returnErr(err error) error {
	if err == nil || b.dontHandleErrs {
		return err
	}

	b.ctx.Status(StatusBadRequest)
	return NewError(StatusBadRequest, "Bad request: "+err.Error())
}

// Struct validation.
func (b *Bind) validateStruct(out any) error {
	if b.skipValidation {
		return nil
	}
	validator := b.ctx.App().config.StructValidator
	if validator != nil {
		return validator.Validate(out)
	}

	return nil
}

// Custom To use custom binders, you have to use this method.
// You can register them from RegisterCustomBinder method of Fiber instance.
// They're checked by name, if it's not found, it will return an error.
// NOTE: WithAutoHandling/WithAutoHandling is still valid for Custom binders.
func (b *Bind) Custom(name string, dest any) error {
	binders := b.ctx.App().customBinders
	for _, customBinder := range binders {
		if customBinder.Name() == name {
			return b.returnErr(customBinder.Parse(b.ctx, dest))
		}
	}

	return ErrCustomBinderNotFound
}

// Header binds the request header strings into the struct, map[string]string and map[string][]string.
func (b *Bind) Header(out any) error {
	bind := binder.GetFromThePool[*binder.HeaderBinding](&binder.HeaderBinderPool)
	bind.EnableSplitting = b.ctx.App().config.EnableSplittingOnParsers

	defer releasePooledBinder(&binder.HeaderBinderPool, bind)

	if err := b.returnErr(bind.Bind(b.ctx.Request(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// RespHeader binds the response header strings into the struct, map[string]string and map[string][]string.
func (b *Bind) RespHeader(out any) error {
	bind := binder.GetFromThePool[*binder.RespHeaderBinding](&binder.RespHeaderBinderPool)
	bind.EnableSplitting = b.ctx.App().config.EnableSplittingOnParsers

	defer releasePooledBinder(&binder.RespHeaderBinderPool, bind)

	if err := b.returnErr(bind.Bind(b.ctx.Response(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// Cookie binds the request cookie strings into the struct, map[string]string and map[string][]string.
// NOTE: If your cookie is like key=val1,val2; they'll be bound as a slice if your map is map[string][]string. Else, it'll use last element of cookie.
func (b *Bind) Cookie(out any) error {
	bind := binder.GetFromThePool[*binder.CookieBinding](&binder.CookieBinderPool)
	bind.EnableSplitting = b.ctx.App().config.EnableSplittingOnParsers

	defer releasePooledBinder(&binder.CookieBinderPool, bind)

	if err := b.returnErr(bind.Bind(&b.ctx.RequestCtx().Request, out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// Query binds the query string into the struct, map[string]string and map[string][]string.
func (b *Bind) Query(out any) error {
	bind := binder.GetFromThePool[*binder.QueryBinding](&binder.QueryBinderPool)
	bind.EnableSplitting = b.ctx.App().config.EnableSplittingOnParsers

	defer releasePooledBinder(&binder.QueryBinderPool, bind)

	if err := b.returnErr(bind.Bind(&b.ctx.RequestCtx().Request, out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// JSON binds the body string into the struct.
func (b *Bind) JSON(out any) error {
	bind := binder.GetFromThePool[*binder.JSONBinding](&binder.JSONBinderPool)
	bind.JSONDecoder = b.ctx.App().Config().JSONDecoder

	defer releasePooledBinder(&binder.JSONBinderPool, bind)

	if err := b.returnErr(bind.Bind(b.ctx.Body(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// CBOR binds the body string into the struct.
func (b *Bind) CBOR(out any) error {
	bind := binder.GetFromThePool[*binder.CBORBinding](&binder.CBORBinderPool)
	bind.CBORDecoder = b.ctx.App().Config().CBORDecoder

	defer releasePooledBinder(&binder.CBORBinderPool, bind)

	if err := b.returnErr(bind.Bind(b.ctx.Body(), out)); err != nil {
		return err
	}
	return b.validateStruct(out)
}

// XML binds the body string into the struct.
func (b *Bind) XML(out any) error {
	bind := binder.GetFromThePool[*binder.XMLBinding](&binder.XMLBinderPool)
	bind.XMLDecoder = b.ctx.App().config.XMLDecoder

	defer releasePooledBinder(&binder.XMLBinderPool, bind)

	if err := b.returnErr(bind.Bind(b.ctx.Body(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// Form binds the form into the struct, map[string]string and map[string][]string.
// If Content-Type is "application/x-www-form-urlencoded" or "multipart/form-data", it will bind the form values.
//
// Binding multipart files is not supported yet.
func (b *Bind) Form(out any) error {
	bind := binder.GetFromThePool[*binder.FormBinding](&binder.FormBinderPool)
	bind.EnableSplitting = b.ctx.App().config.EnableSplittingOnParsers

	defer releasePooledBinder(&binder.FormBinderPool, bind)

	if err := b.returnErr(bind.Bind(&b.ctx.RequestCtx().Request, out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// URI binds the route parameters into the struct, map[string]string and map[string][]string.
func (b *Bind) URI(out any) error {
	bind := binder.GetFromThePool[*binder.URIBinding](&binder.URIBinderPool)

	defer releasePooledBinder(&binder.URIBinderPool, bind)

	if err := b.returnErr(bind.Bind(b.ctx.Route().Params, b.ctx.Params, out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// MsgPack binds the body string into the struct.
func (b *Bind) MsgPack(out any) error {
	bind := binder.GetFromThePool[*binder.MsgPackBinding](&binder.MsgPackBinderPool)
	bind.MsgPackDecoder = b.ctx.App().Config().MsgPackDecoder

	defer releasePooledBinder(&binder.MsgPackBinderPool, bind)

	if err := b.returnErr(bind.Bind(b.ctx.Body(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// Body binds the request body into the struct, map[string]string and map[string][]string.
// It supports decoding the following content types based on the Content-Type header:
// application/json, application/xml, application/x-www-form-urlencoded, multipart/form-data
// If none of the content types above are matched, it'll take a look custom binders by checking the MIMETypes() method of custom binder.
// If there is no custom binder for mime type of body, it will return a ErrUnprocessableEntity error.
func (b *Bind) Body(out any) error {
	// Get content-type
	ctype := utils.ToLower(utils.UnsafeString(b.ctx.RequestCtx().Request.Header.ContentType()))
	ctype = binder.FilterFlags(utils.ParseVendorSpecificContentType(ctype))

	// Check custom binders
	binders := b.ctx.App().customBinders
	for _, customBinder := range binders {
		if slices.Contains(customBinder.MIMETypes(), ctype) {
			return b.returnErr(customBinder.Parse(b.ctx, out))
		}
	}

	// Parse body accordingly
	switch ctype {
	case MIMEApplicationJSON:
		return b.JSON(out)
	case MIMEApplicationMsgPack:
		return b.MsgPack(out)
	case MIMETextXML, MIMEApplicationXML:
		return b.XML(out)
	case MIMEApplicationCBOR:
		return b.CBOR(out)
	case MIMEApplicationForm, MIMEMultipartForm:
		return b.Form(out)
	}

	// No suitable content type found
	return ErrUnprocessableEntity
}

// All binds values from URI params, the request body, the query string,
// headers, and cookies into the provided struct in precedence order.
func (b *Bind) All(out any) error {
	outVal := reflect.ValueOf(out)
	if outVal.Kind() != reflect.Ptr || outVal.Elem().Kind() != reflect.Struct {
		return ErrUnprocessableEntity
	}

	outElem := outVal.Elem()

	// Precedence: URL Params -> Body -> Query -> Headers -> Cookies
	sources := []func(any) error{b.URI}

	// Check if both Body and Content-Type are set
	if len(b.ctx.Request().Body()) > 0 && len(b.ctx.RequestCtx().Request.Header.ContentType()) > 0 {
		sources = append(sources, b.Body)
	}
	sources = append(sources, b.Query, b.Header, b.Cookie)
	prevSkip := b.skipValidation
	b.skipValidation = true

	// TODO: Support custom precedence with an optional binding_source tag
	// TODO: Create WithOverrideEmptyValues
	// Bind from each source, but only update unset fields
	for _, bindFunc := range sources {
		tempStruct := reflect.New(outElem.Type()).Interface()
		if err := bindFunc(tempStruct); err != nil {
			b.skipValidation = prevSkip
			return err
		}

		tempStructVal := reflect.ValueOf(tempStruct).Elem()
		mergeStruct(outElem, tempStructVal)
	}

	b.skipValidation = prevSkip
	return b.returnErr(b.validateStruct(out))
}

func mergeStruct(dst, src reflect.Value) {
	dstFields := dst.NumField()
	for i := range dstFields {
		dstField := dst.Field(i)
		srcField := src.Field(i)

		// Skip if the destination field is already set
		if isZero(dstField.Interface()) {
			if dstField.CanSet() && srcField.IsValid() {
				dstField.Set(srcField)
			}
		}
	}
}

func isZero(value any) bool {
	v := reflect.ValueOf(value)
	return v.IsZero()
}
