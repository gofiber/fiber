package fiber

import (
	"fmt"
	"reflect"

	"github.com/gofiber/fiber/v3/internal/bind"
)

type Binder interface {
	UnmarshalFiberCtx(ctx Ctx) error
}

// decoder should set a field on reqValue
// it's created with field index
type decoder interface {
	Decode(ctx Ctx, reqValue reflect.Value) error
}

type fieldCtxDecoder struct {
	index     int
	fieldName string
	fieldType reflect.Type
}

func (d *fieldCtxDecoder) Decode(ctx Ctx, reqValue reflect.Value) error {
	v := reflect.New(d.fieldType)
	unmarshaler := v.Interface().(Binder)

	if err := unmarshaler.UnmarshalFiberCtx(ctx); err != nil {
		return err
	}

	reqValue.Field(d.index).Set(v.Elem())
	return nil
}

type fieldTextDecoder struct {
	index       int
	parentIndex []int
	fieldName   string
	tag         string // query,param,header,respHeader ...
	reqField    string
	et          reflect.Type
	dec         bind.TextDecoder
	get         func(c Ctx, key string, defaultValue ...string) string
}

func (d *fieldTextDecoder) Decode(ctx Ctx, reqValue reflect.Value) error {
	text := d.get(ctx, d.reqField)
	if text == "" {
		return nil
	}

	var err error
	for _, i := range d.parentIndex {
		reqValue = reqValue.Field(i)
	}

	// Pointer support for struct elems
	field := reqValue.Field(d.index)
	if field.Kind() == reflect.Ptr {
		elem := reflect.New(d.et)
		err = d.dec.UnmarshalString(text, elem.Elem())
		if err != nil {
			return fmt.Errorf("unable to decode '%s' as %s: %w", text, d.reqField, err)
		}

		field.Set(elem)

		return nil
	}

	// Non-pointer elems
	err = d.dec.UnmarshalString(text, field)
	if err != nil {
		return fmt.Errorf("unable to decode '%s' as %s: %w", text, d.reqField, err)
	}

	return nil
}
