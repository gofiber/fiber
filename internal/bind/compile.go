package bind

import (
	"encoding"
	"errors"
	"reflect"
)

type TextDecoder interface {
	UnmarshalString(s string, fieldValue reflect.Value) error
}

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

func CompileTextDecoder(rt reflect.Type) (TextDecoder, error) {
	// encoding.TextUnmarshaler
	if reflect.PtrTo(rt).Implements(textUnmarshalerType) {
		return &textUnmarshalEncoder{fieldType: rt}, nil
	}

	switch rt.Kind() {
	case reflect.Bool:
		return &boolDecoder{}, nil
	case reflect.Uint8:
		return &uintDecoder{bitSize: 8}, nil
	case reflect.Uint16:
		return &uintDecoder{bitSize: 16}, nil
	case reflect.Uint32:
		return &uintDecoder{bitSize: 32}, nil
	case reflect.Uint64:
		return &uintDecoder{bitSize: 64}, nil
	case reflect.Uint:
		return &uintDecoder{}, nil
	case reflect.Int8:
		return &intDecoder{bitSize: 8}, nil
	case reflect.Int16:
		return &intDecoder{bitSize: 16}, nil
	case reflect.Int32:
		return &intDecoder{bitSize: 32}, nil
	case reflect.Int64:
		return &intDecoder{bitSize: 64}, nil
	case reflect.Int:
		return &intDecoder{}, nil
	case reflect.String:
		return &stringDecoder{}, nil
	case reflect.Float32:
		return &floatDecoder{bitSize: 32}, nil
	case reflect.Float64:
		return &floatDecoder{bitSize: 64}, nil
	}

	return nil, errors.New("unsupported type " + rt.String())
}
