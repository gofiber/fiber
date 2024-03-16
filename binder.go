package fiber

import (
	"bytes"
	"net/http"
	"reflect"
	"sync"

	"github.com/gofiber/fiber/v3/internal/bind"
	"github.com/gofiber/utils/v2"
)

var binderPool = sync.Pool{New: func() any {
	return &Bind{}
}}

type Bind struct {
	err    error
	ctx    Ctx
	val    any // last decoded val
	strict bool
}

func (c *DefaultCtx) Bind() *Bind {
	b := binderPool.Get().(*Bind)
	b.ctx = c
	return b
}

func (b *Bind) Strict() *Bind {
	b.strict = true
	return b
}

func (b *Bind) setErr(err error) *Bind {
	b.err = err
	return b
}

func (b *Bind) reset() {
	b.ctx = nil
	b.val = nil
	b.err = nil
	b.strict = false
}

// HTTPErr return a wrapped fiber.Error for 400 http bad request.
// it's not safe to use after HTTPErr is called.
func (b *Bind) HTTPErr() error {
	err := b.Err()

	if err != nil {
		if fe, ok := err.(*Error); ok {
			return fe
		}

		return NewError(http.StatusBadRequest, err.Error())
	}

	return nil
}

// Err return binding error and put binder back to pool
// it's not safe to use after Err is called.
func (b *Bind) Err() error {
	err := b.err

	b.reset()
	binderPool.Put(b)

	return err
}

// JSON unmarshal body as json
func (b *Bind) JSON(v any) *Bind {
	if b.err != nil {
		return b
	}

	if b.strict {
		if !bytes.HasPrefix(b.ctx.Request().Header.ContentType(), utils.UnsafeBytes(MIMEApplicationJSON)) {
			return b.setErr(NewError(http.StatusUnsupportedMediaType, "expecting content-type \"application/json\""))
		}
	}

	if err := b.ctx.App().config.JSONDecoder(b.ctx.Body(), v); err != nil {
		return b.setErr(err)
	}

	b.val = v
	return b
}

// XML unmarshal body as xml
func (b *Bind) XML(v any) *Bind {
	if b.err != nil {
		return b
	}

	if b.strict {
		if !bytes.HasPrefix(b.ctx.Request().Header.ContentType(), utils.UnsafeBytes(MIMEApplicationXML)) {
			return b.setErr(NewError(http.StatusUnsupportedMediaType, "expecting content-type \"application/xml\""))
		}
	}

	if err := b.ctx.App().config.XMLDecoder(b.ctx.Body(), v); err != nil {
		return b.setErr(err)
	}

	b.val = v
	return b
}

// Form unmarshal body as form
func (b *Bind) Form(v any) *Bind {
	if b.err != nil {
		return b
	}

	if b.strict {
		if !bytes.HasPrefix(b.ctx.Request().Header.ContentType(), utils.UnsafeBytes(MIMEApplicationForm)) {
			return b.setErr(NewError(http.StatusUnsupportedMediaType, "expecting content-type \"application/x-www-form-urlencoded\""))
		}
	}

	if err := b.formDecode(v); err != nil {
		return b.setErr(err)
	}

	b.val = v
	return b
}

// Multipart unmarshal body as multipart/form-data
// TODO: handle multipart files.
func (b *Bind) Multipart(v any) *Bind {
	if b.err != nil {
		return b
	}

	if b.strict {
		if !bytes.HasPrefix(b.ctx.Request().Header.ContentType(), utils.UnsafeBytes(MIMEMultipartForm)) {
			return b.setErr(NewError(http.StatusUnsupportedMediaType, "expecting content-type \"multipart/form-data\""))
		}
	}

	if err := b.multipartDecode(v); err != nil {
		return b.setErr(err)
	}

	b.val = v
	return b
}

func (b *Bind) Req(v any) *Bind {
	if b.err != nil {
		return b
	}

	if err := b.reqDecode(v); err != nil {
		return b.setErr(err)
	}

	b.val = v
	return b
}

func (b *Bind) Validate() *Bind {
	if b.err != nil {
		return b
	}

	if b.val == nil {
		return b
	}

	if err := b.ctx.Validate(b.val); err != nil {
		return b.setErr(err)
	}

	return b
}

func (b *Bind) reqDecode(v any) error {
	rv, typeID := bind.ValueAndTypeID(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &InvalidBinderError{Type: reflect.TypeOf(v)}
	}

	cached, ok := b.ctx.App().bindDecoderCache.Load(typeID)
	if ok {
		// cached decoder, fast path
		decoder := cached.(Decoder)
		return decoder(b.ctx, rv.Elem())
	}

	decoder, err := compileReqParser(rv.Type(), bindCompileOption{reqDecoder: true})
	if err != nil {
		return err
	}

	b.ctx.App().bindDecoderCache.Store(typeID, decoder)
	return decoder(b.ctx, rv.Elem())
}

func (b *Bind) formDecode(v any) error {
	rv, typeID := bind.ValueAndTypeID(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &InvalidBinderError{Type: reflect.TypeOf(v)}
	}

	cached, ok := b.ctx.App().formDecoderCache.Load(typeID)
	if ok {
		// cached decoder, fast path
		decoder := cached.(Decoder)
		return decoder(b.ctx, rv.Elem())
	}

	decoder, err := compileReqParser(rv.Type(), bindCompileOption{bodyDecoder: true})
	if err != nil {
		return err
	}

	b.ctx.App().formDecoderCache.Store(typeID, decoder)
	return decoder(b.ctx, rv.Elem())
}

func (b *Bind) multipartDecode(v any) error {
	rv, typeID := bind.ValueAndTypeID(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &InvalidBinderError{Type: reflect.TypeOf(v)}
	}

	cached, ok := b.ctx.App().multipartDecoderCache.Load(typeID)
	if ok {
		// cached decoder, fast path
		decoder := cached.(Decoder)
		return decoder(b.ctx, rv.Elem())
	}

	decoder, err := compileReqParser(rv.Type(), bindCompileOption{bodyDecoder: true})
	if err != nil {
		return err
	}

	b.ctx.App().multipartDecoderCache.Store(typeID, decoder)
	return decoder(b.ctx, rv.Elem())
}
