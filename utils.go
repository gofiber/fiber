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

func parseIntWithDefault[V QueryType](q string, bitSize int, defaultValue ...V) int64 {
	result, err := strconv.ParseInt(q, 10, bitSize)
	if err != nil {
		if len(defaultValue) > 0 {
			return assertValueType[int64, V](defaultValue[0])
		}
		return int64(0)
	}
	return result
}

func parseUintWithDefault[V QueryType](q string, bitSize int, defaultValue ...V) uint64 {
	result, err := strconv.ParseUint(q, 10, bitSize)
	if err != nil {
		if len(defaultValue) > 0 {
			return assertValueType[uint64, V](defaultValue[0])
		}
		return uint64(0)
	}
	return result
}

func parseFloatWithDefault[V QueryType](q string, bitSize int, defaultValue ...V) float64 {
	result, err := strconv.ParseFloat(q, bitSize)
	if err != nil {
		if len(defaultValue) > 0 {
			return assertValueType[float64, V](defaultValue[0])
		}
		return float64(0)
	}
	return result
}

func parseStringWithDefault[V QueryType](q string, defaultValue ...V) string {
	if q == "" && len(defaultValue) > 0 {
		return assertValueType[string, V](defaultValue[0])
	}
	return q
}

func parseBoolWithDefault[V QueryType](q string, defaultValue ...V) bool {
	result, err := strconv.ParseBool(q)
	if err != nil {
		if len(defaultValue) > 0 {
			return assertValueType[bool, V](defaultValue[0])
		}
		return false
	}
	return result
}
