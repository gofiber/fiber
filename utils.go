package fiber

import (
	"fmt"
	"strconv"
)

// assertValueType asserts the type of the result to the type of the value
func assertValueType[V GenericType, T any](result T) V {
	v, ok := any(result).(V)
	if !ok {
		panic(fmt.Errorf("failed to type-assert to %T", v))
	}
	return v
}

func genericParseDefault[V GenericType](err error, parser func() V, defaultValue ...V) V {
	var v V
	if err != nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return v
	}
	return parser()
}

func genericParseInt[V GenericType](str string, bitSize int, parser func(int64) V, defaultValue ...V) V {
	result, err := strconv.ParseInt(str, 10, bitSize)
	return genericParseDefault[V](err, func() V { return parser(result) }, defaultValue...)
}

func genericParseUint[V GenericType](str string, bitSize int, parser func(uint64) V, defaultValue ...V) V {
	result, err := strconv.ParseUint(str, 10, bitSize)
	return genericParseDefault[V](err, func() V { return parser(result) }, defaultValue...)
}

func genericParseFloat[V GenericType](str string, bitSize int, parser func(float64) V, defaultValue ...V) V {
	result, err := strconv.ParseFloat(str, bitSize)
	return genericParseDefault[V](err, func() V { return parser(result) }, defaultValue...)
}

func genericParseBool[V GenericType](str string, parser func(bool) V, defaultValue ...V) V {
	result, err := strconv.ParseBool(str)
	return genericParseDefault[V](err, func() V { return parser(result) }, defaultValue...)
}

func genericParseType[V GenericType](ctx *DefaultCtx, str string, v V, defaultValue ...V) V {
	switch any(v).(type) {
	case int:
		return genericParseInt[V](str, 32, func(i int64) V { return assertValueType[V, int](int(i)) }, defaultValue...)
	case int8:
		return genericParseInt[V](str, 8, func(i int64) V { return assertValueType[V, int8](int8(i)) }, defaultValue...)
	case int16:
		return genericParseInt[V](str, 16, func(i int64) V { return assertValueType[V, int16](int16(i)) }, defaultValue...)
	case int32:
		return genericParseInt[V](str, 32, func(i int64) V { return assertValueType[V, int32](int32(i)) }, defaultValue...)
	case int64:
		return genericParseInt[V](str, 64, func(i int64) V { return assertValueType[V, int64](i) }, defaultValue...)
	case uint:
		return genericParseUint[V](str, 32, func(i uint64) V { return assertValueType[V, uint](uint(i)) }, defaultValue...)
	case uint8:
		return genericParseUint[V](str, 8, func(i uint64) V { return assertValueType[V, uint8](uint8(i)) }, defaultValue...)
	case uint16:
		return genericParseUint[V](str, 16, func(i uint64) V { return assertValueType[V, uint16](uint16(i)) }, defaultValue...)
	case uint32:
		return genericParseUint[V](str, 32, func(i uint64) V { return assertValueType[V, uint32](uint32(i)) }, defaultValue...)
	case uint64:
		return genericParseUint[V](str, 64, func(i uint64) V { return assertValueType[V, uint64](i) }, defaultValue...)
	case float32:
		return genericParseFloat[V](str, 32, func(i float64) V { return assertValueType[V, float32](float32(i)) }, defaultValue...)
	case float64:
		return genericParseFloat[V](str, 64, func(i float64) V { return assertValueType[V, float64](i) }, defaultValue...)
	case bool:
		return genericParseBool[V](str, func(b bool) V { return assertValueType[V, bool](b) }, defaultValue...)
	case string:
		if str == "" && len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return assertValueType[V, string](str)
	case []byte:
		if str == "" && len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return assertValueType[V, []byte](ctx.app.getBytes(str))
	default:
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return v
	}
}
