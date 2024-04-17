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
	ctx    Ctx
	should bool
}

// Should To handle binder errors manually, you can prefer Should method.
// It's default behavior of binder.
func (b *Bind) Should() *Bind {
	b.should = true

	return b
}

// Must If you want to handle binder errors automatically, you can use Must.
// If there's an error it'll return error and 400 as HTTP status.
func (b *Bind) Must() *Bind {
	b.should = false

	return b
}

// Check Should/Must errors and return it by usage.
func (b *Bind) returnErr(err error) error {
	if !b.should {
		b.ctx.Status(StatusBadRequest)
		return NewError(StatusBadRequest, "Bad request: "+err.Error())
	}

	return err
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
// NOTE: Should/Must is still valid for Custom binders.
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
	if err := b.returnErr(binder.HeaderBinder.Bind(b.ctx.Request(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// RespHeader binds the response header strings into the struct, map[string]string and map[string][]string.
func (b *Bind) RespHeader(out any) error {
	if err := b.returnErr(binder.RespHeaderBinder.Bind(b.ctx.Response(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// Cookie binds the requesr cookie strings into the struct, map[string]string and map[string][]string.
// NOTE: If your cookie is like key=val1,val2; they'll be binded as an slice if your map is map[string][]string. Else, it'll use last element of cookie.
func (b *Bind) Cookie(out any) error {
	if err := b.returnErr(binder.CookieBinder.Bind(b.ctx.Context(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// Query binds the query string into the struct, map[string]string and map[string][]string.
func (b *Bind) Query(out any) error {
	if err := b.returnErr(binder.QueryBinder.Bind(b.ctx.Context(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// JSON binds the body string into the struct.
func (b *Bind) JSON(out any) error {
	if err := b.returnErr(binder.JSONBinder.Bind(b.ctx.Body(), b.ctx.App().Config().JSONDecoder, out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// XML binds the body string into the struct.
func (b *Bind) XML(out any) error {
	if err := b.returnErr(binder.XMLBinder.Bind(b.ctx.Body(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// Form binds the form into the struct, map[string]string and map[string][]string.
func (b *Bind) Form(out any) error {
	if err := b.returnErr(binder.FormBinder.Bind(b.ctx.Context(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// URI binds the route parameters into the struct, map[string]string and map[string][]string.
func (b *Bind) URI(out any) error {
	if err := b.returnErr(binder.URIBinder.Bind(b.ctx.Route().Params, b.ctx.Params, out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// MultipartForm binds the multipart form into the struct, map[string]string and map[string][]string.
func (b *Bind) MultipartForm(out any) error {
	if err := b.returnErr(binder.FormBinder.BindMultipart(b.ctx.Context(), out)); err != nil {
		return err
	}

	return b.validateStruct(out)
}

// Body binds the request body into the struct, map[string]string and map[string][]string.
// It supports decoding the following content types based on the Content-Type header:
// application/json, application/xml, application/x-www-form-urlencoded, multipart/form-data
// If none of the content types above are matched, it'll take a look custom binders by checking the MIMETypes() method of custom binder.
// If there're no custom binder for mşme type of body, it will return a ErrUnprocessableEntity error.
func (b *Bind) Body(out any) error {
	// Get content-type
	ctype := utils.ToLower(utils.UnsafeString(b.ctx.Context().Request.Header.ContentType()))
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
	case MIMEApplicationForm:
		return b.Form(out)
	case MIMEMultipartForm:
		return b.MultipartForm(out)
	}

	// No suitable content type found
	return ErrUnprocessableEntity
}
