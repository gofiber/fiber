package bind

import (
	"reflect"

	"github.com/gofiber/utils/v2"
)

type stringDecoder struct {
}

func (d *stringDecoder) UnmarshalString(s string, fieldValue reflect.Value) error {
	fieldValue.SetString(utils.CopyString(s))
	return nil
}
