package fiber

import (
	"reflect"

	"github.com/gofiber/fiber/v3/internal/bind"
	"github.com/gofiber/utils/v2"
)

var _ decoder = (*fieldSliceDecoder)(nil)

type fieldSliceDecoder struct {
	fieldIndex int
	fieldName  string
	fieldType  reflect.Type
	reqKey     []byte
	// [utils.EqualFold] for headers and [bytes.Equal] for query/params.
	eqBytes        func([]byte, []byte) bool
	elementType    reflect.Type
	elementDecoder bind.TextDecoder
	visitAll       func(Ctx, func(key []byte, value []byte))
}

func (d *fieldSliceDecoder) Decode(ctx Ctx, reqValue reflect.Value) error {
	count := 0
	d.visitAll(ctx, func(key, value []byte) {
		if d.eqBytes(key, d.reqKey) {
			count++
		}
	})

	rv := reflect.MakeSlice(d.fieldType, 0, count)

	if count == 0 {
		reqValue.Field(d.fieldIndex).Set(rv)
		return nil
	}

	var err error
	d.visitAll(ctx, func(key, value []byte) {
		if err != nil {
			return
		}
		if d.eqBytes(key, d.reqKey) {
			ev := reflect.New(d.elementType)
			if ee := d.elementDecoder.UnmarshalString(utils.UnsafeString(value), ev.Elem()); ee != nil {
				err = ee
			}

			rv = reflect.Append(rv, ev.Elem())
		}
	})

	if err != nil {
		return err
	}

	reqValue.Field(d.fieldIndex).Set(rv)
	return nil
}

func visitQuery(ctx Ctx, f func(key []byte, value []byte)) {
	ctx.Context().QueryArgs().VisitAll(f)
}

func visitHeader(ctx Ctx, f func(key []byte, value []byte)) {
	ctx.Request().Header.VisitAll(f)
}

func visitResHeader(ctx Ctx, f func(key []byte, value []byte)) {
	ctx.Response().Header.VisitAll(f)
}

func visitCookie(ctx Ctx, f func(key []byte, value []byte)) {
	ctx.Request().Header.VisitAllCookie(f)
}

func visitForm(ctx Ctx, f func(key []byte, value []byte)) {
	ctx.Request().PostArgs().VisitAll(f)
}

func visitMultipart(ctx Ctx, f func(key []byte, value []byte)) {
	mp, err := ctx.Request().MultipartForm()
	if err != nil {
		return
	}

	for key, values := range mp.Value {
		for _, value := range values {
			f(utils.UnsafeBytes(key), utils.UnsafeBytes(value))
		}
	}
}
