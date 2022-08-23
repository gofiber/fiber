package bind

import (
	"reflect"
	"strconv"
)

type boolDecoder struct {
}

func (d *boolDecoder) UnmarshalString(s string, fieldValue reflect.Value) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	fieldValue.SetBool(v)
	return nil
}
