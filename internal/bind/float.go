package bind

import (
	"reflect"
	"strconv"
)

type floatDecoder struct {
	bitSize int
}

func (d *floatDecoder) UnmarshalString(s string, fieldValue reflect.Value) error {
	v, err := strconv.ParseFloat(s, d.bitSize)
	if err != nil {
		return err
	}
	fieldValue.SetFloat(v)
	return nil
}
