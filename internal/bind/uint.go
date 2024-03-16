package bind

import (
	"reflect"
	"strconv"
)

type uintDecoder struct {
	bitSize int
}

func (d *uintDecoder) UnmarshalString(s string, fieldValue reflect.Value) error {
	v, err := strconv.ParseUint(s, 10, d.bitSize)
	if err != nil {
		return err
	}
	fieldValue.SetUint(v)
	return nil
}
