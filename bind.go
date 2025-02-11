package fiber

import (
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

// Bind struct
type Bind struct {
	ctx            Ctx
	dontHandleErrs bool
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

	// Reset & put binder
	defer func() {
		bind.Reset()
		binder.PutToThePool(&binder.HeaderBinderPool, bind)
	}()

	if err := b.returnErr(bind.Bind(b.ctx.Request(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// RespHeader binds the response header strings into the struct, map[string]string and map[string][]string.
func (b *Bind) RespHeader(out any) error {
	bind := binder.GetFromThePool[*binder.RespHeaderBinding](&binder.RespHeaderBinderPool)
	bind.EnableSplitting = b.ctx.App().config.EnableSplittingOnParsers

	// Reset & put binder
	defer func() {
		bind.Reset()
		binder.PutToThePool(&binder.RespHeaderBinderPool, bind)
	}()

	if err := b.returnErr(bind.Bind(b.ctx.Response(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// Cookie binds the request cookie strings into the struct, map[string]string and map[string][]string.
// NOTE: If your cookie is like key=val1,val2; they'll be binded as an slice if your map is map[string][]string. Else, it'll use last element of cookie.
func (b *Bind) Cookie(out any) error {
	bind := binder.GetFromThePool[*binder.CookieBinding](&binder.CookieBinderPool)
	bind.EnableSplitting = b.ctx.App().config.EnableSplittingOnParsers

	// Reset & put binder
	defer func() {
		bind.Reset()
		binder.PutToThePool(&binder.CookieBinderPool, bind)
	}()

	if err := b.returnErr(bind.Bind(&b.ctx.RequestCtx().Request, out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// Query binds the query string into the struct, map[string]string and map[string][]string.
func (b *Bind) Query(out any) error {
	bind := binder.GetFromThePool[*binder.QueryBinding](&binder.QueryBinderPool)
	bind.EnableSplitting = b.ctx.App().config.EnableSplittingOnParsers

	// Reset & put binder
	defer func() {
		bind.Reset()
		binder.PutToThePool(&binder.QueryBinderPool, bind)
	}()

	if err := b.returnErr(bind.Bind(&b.ctx.RequestCtx().Request, out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// JSON binds the body string into the struct.
func (b *Bind) JSON(out any) error {
	bind := binder.GetFromThePool[*binder.JSONBinding](&binder.JSONBinderPool)
	bind.JSONDecoder = b.ctx.App().Config().JSONDecoder

	// Reset & put binder
	defer func() {
		bind.Reset()
		binder.PutToThePool(&binder.JSONBinderPool, bind)
	}()

	if err := b.returnErr(bind.Bind(b.ctx.Body(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// CBOR binds the body string into the struct.
func (b *Bind) CBOR(out any) error {
	bind := binder.GetFromThePool[*binder.CBORBinding](&binder.CBORBinderPool)
	bind.CBORDecoder = b.ctx.App().Config().CBORDecoder

	// Reset & put binder
	defer func() {
		bind.Reset()
		binder.PutToThePool(&binder.CBORBinderPool, bind)
	}()

	if err := b.returnErr(bind.Bind(b.ctx.Body(), out)); err != nil {
		return err
	}
	return b.validateStruct(out)
}

// XML binds the body string into the struct.
func (b *Bind) XML(out any) error {
	bind := binder.GetFromThePool[*binder.XMLBinding](&binder.XMLBinderPool)
	bind.XMLDecoder = b.ctx.App().config.XMLDecoder

	// Reset & put binder
	defer func() {
		bind.Reset()
		binder.PutToThePool(&binder.XMLBinderPool, bind)
	}()

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

	// Reset & put binder
	defer func() {
		bind.Reset()
		binder.PutToThePool(&binder.FormBinderPool, bind)
	}()

	if err := b.returnErr(bind.Bind(&b.ctx.RequestCtx().Request, out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// URI binds the route parameters into the struct, map[string]string and map[string][]string.
func (b *Bind) URI(out any) error {
	bind := binder.GetFromThePool[*binder.URIBinding](&binder.URIBinderPool)

	// Reset & put binder
	defer func() {
		binder.PutToThePool(&binder.URIBinderPool, bind)
	}()

	if err := b.returnErr(bind.Bind(b.ctx.Route().Params, b.ctx.Params, out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// Body binds the request body into the struct, map[string]string and map[string][]string.
// It supports decoding the following content types based on the Content-Type header:
// application/json, application/xml, application/x-www-form-urlencoded, multipart/form-data
// If none of the content types above are matched, it'll take a look custom binders by checking the MIMETypes() method of custom binder.
// If there're no custom binder for mime type of body, it will return a ErrUnprocessableEntity error.
func (b *Bind) Body(out any) error {
	// Get content-type
	ctype := utils.ToLower(utils.UnsafeString(b.ctx.RequestCtx().Request.Header.ContentType()))
	ctype = binder.FilterFlags(utils.ParseVendorSpecificContentType(ctype))

	// Check custom binders
	binders := b.ctx.App().customBinders
	for _, customBinder := range binders {
		for _, mime := range customBinder.MIMETypes() {
			if mime == ctype {
				return b.returnErr(customBinder.Parse(b.ctx, out))
			}
		}
	}

	// Parse body accordingly
	switch ctype {
	case MIMEApplicationJSON:
		return b.JSON(out)
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
