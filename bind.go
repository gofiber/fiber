package fiber

import (
	"encoding"
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
	Kind() string
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

func (d *fieldCtxDecoder) Kind() string {
	return "ctx"
}

type fieldTextDecoder struct {
	fieldIndex       int
	fieldName        string
	tag              string // query,param,header,respHeader ...
	reqKey           string
	dec              bind.TextDecoder
	get              func(c Ctx, key string, defaultValue ...string) string
	subFieldDecoders []decoder
	isTextMarshaler  bool
	fragments        []requestKeyFragment
}

func (d *fieldTextDecoder) Decode(ctx Ctx, reqValue reflect.Value) error {
	field := reqValue.Field(d.fieldIndex)

	// Support for sub fields
	if len(d.subFieldDecoders) > 0 {
		for _, subFieldDecoder := range d.subFieldDecoders {
			err := subFieldDecoder.Decode(ctx, field)
			if err != nil {
				return err
			}
		}
		return nil
	}

	text := d.get(ctx, d.reqKey)
	if text == "" {
		return nil
	}

	if d.isTextMarshaler {
		unmarshaler, ok := field.Addr().Interface().(encoding.TextUnmarshaler)
		if !ok {
			return fmt.Errorf("field %s does not implement encoding.TextUnmarshaler", d.fieldName)
		}

		err := unmarshaler.UnmarshalText([]byte(text))
		if err != nil {
			return fmt.Errorf("unable to decode '%s' as %s: %w", text, d.reqKey, err)
		}

		return nil
	}

	err := d.dec.UnmarshalString(text, field)
	if err != nil {
		return fmt.Errorf("unable to decode '%s' as %s: %w", text, d.reqKey, err)
	}

	return nil
}

func (d *fieldTextDecoder) Kind() string {
	return "text"
}
