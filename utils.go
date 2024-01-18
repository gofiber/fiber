package fiber

import (
	"fmt"
)

// assertValueType asserts the type of the result to the type of the value
func assertValueType[V QueryType, T any](result T) V {
	v, ok := any(result).(V)
	if !ok {
		panic(fmt.Errorf("failed to type-assert to %T", v))
	}
	return v
}
