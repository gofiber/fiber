package fiber

import (
	goErrors "errors"
)

// Range errors
var (
	ErrRangeMalformed     = goErrors.New("range: malformed range header string")
	ErrRangeUnsatisfiable = goErrors.New("range: unsatisfiable range")
)
