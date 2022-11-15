package bind

import (
	"encoding"
	"reflect"

	"github.com/gofiber/utils/v2"
)

type textUnmarshalEncoder struct {
	fieldType reflect.Type
}

func (d *textUnmarshalEncoder) UnmarshalString(s string, fieldValue reflect.Value) error {
	if s == "" {
		return nil
	}

	v := reflect.New(d.fieldType)
	unmarshaler := v.Interface().(encoding.TextUnmarshaler)

	if err := unmarshaler.UnmarshalText(utils.UnsafeBytes(s)); err != nil {
		return err
	}

	fieldValue.Set(v.Elem())

	return nil
}
