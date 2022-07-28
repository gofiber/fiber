package fiber

import (
	"errors"

	"github.com/gofiber/fiber/v3/binder"
	"github.com/gofiber/fiber/v3/utils"
)

var ErrCustomBinderNotFound = errors.New("fiber: custom binder not found, please be sure to enter right name!")

type CustomBinder interface {
	Name() string
	MIMETypes() []string
	Parse(Ctx, any) error
}

type Bind struct {
	ctx    *DefaultCtx
	should bool
}

func (b *Bind) Should() {
	b.should = true
}

func (b *Bind) Must() {
	b.should = false
}

func (b *Bind) Custom(name string, dest any) error {
	binders := b.ctx.App().customBinders
	for _, binder := range binders {
		if binder.Name() == name {
			return binder.Parse(b.ctx, dest)
		}
	}

	return ErrCustomBinderNotFound
}

func (b *Bind) Header(out any) error {
	return binder.HeaderBinder.Bind(b.ctx.Request(), out)
}

func (b *Bind) RespHeader(out any) error {
	return binder.RespHeaderBinder.Bind(b.ctx.Response(), out)
}

func (b *Bind) Cookie(out any) error {
	return binder.CookieBinder.Bind(b.ctx.Context(), out)
}

func (b *Bind) Query(out any) error {
	return binder.QueryBinder.Bind(b.ctx.Context(), out)
}

func (b *Bind) JSON(out any) error {
	return binder.JSONBinder.Bind(b.ctx.Body(), b.ctx.App().Config().JSONDecoder, out)
}

func (b *Bind) XML(out any) error {
	return binder.XMLBinder.Bind(b.ctx.Body(), out)
}

func (b *Bind) Form(out any) error {
	return binder.FormBinder.Bind(b.ctx.Context(), out)
}

func (b *Bind) URI(out any) error {
	return binder.URIBinder.Bind(b.ctx.route.Params, b.ctx.Params, out)
}

func (b *Bind) MultipartForm(out any) error {
	return binder.FormBinder.BindMultipart(b.ctx.Context(), out)
}

func (b *Bind) Body(out any) error {
	// Get content-type
	ctype := utils.ToLower(utils.UnsafeString(b.ctx.Context().Request.Header.ContentType()))
	ctype = binder.FilterFlags(utils.ParseVendorSpecificContentType(ctype))

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

	// Check custom binders
	binders := b.ctx.App().customBinders
	for _, binder := range binders {
		for _, mime := range binder.MIMETypes() {
			if mime == ctype {
				return binder.Parse(b.ctx, out)
			}
		}
	}

	// No suitable content type found
	return ErrUnprocessableEntity
}
