package fiber

import (
	"fmt"
	"strconv"
)

// assertValueType asserts the type of the result to the type of the value
func assertValueType[V QueryType, T any](result T) V {
	v, ok := any(result).(V)
	if !ok {
		panic(fmt.Errorf("failed to type-assert to %T", v))
	}
	return v
}

func queryParseDefault[V QueryType](err error, parser func() V, defaultValue ...V) V {
	var v V
	if err != nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return v
	}
	return parser()
}

func queryParseInt[V QueryType](q string, bitSize int, parser func(int64) V, defaultValue ...V) V {
	result, err := strconv.ParseInt(q, 10, bitSize)
	return queryParseDefault[V](err, func() V { return parser(result) }, defaultValue...)
}

func queryParseUint[V QueryType](q string, bitSize int, parser func(uint64) V, defaultValue ...V) V {
	result, err := strconv.ParseUint(q, 10, bitSize)
	return queryParseDefault[V](err, func() V { return parser(result) }, defaultValue...)
}

func queryParseFloat[V QueryType](q string, bitSize int, parser func(float64) V, defaultValue ...V) V {
	result, err := strconv.ParseFloat(q, bitSize)
	return queryParseDefault[V](err, func() V { return parser(result) }, defaultValue...)
}

func queryParseBool[V QueryType](q string, parser func(bool) V, defaultValue ...V) V {
	result, err := strconv.ParseBool(q)
	return queryParseDefault[V](err, func() V { return parser(result) }, defaultValue...)
}
