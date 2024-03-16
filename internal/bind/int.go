package bind

import (
	"reflect"
	"strconv"
)

type intDecoder struct {
	bitSize int
}

func (d *intDecoder) UnmarshalString(s string, fieldValue reflect.Value) error {
	v, err := strconv.ParseInt(s, 10, d.bitSize)
	if err != nil {
		return err
	}
	fieldValue.SetInt(v)
	return nil
}
