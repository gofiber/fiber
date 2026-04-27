package logtemplate

import (
	"errors"
)

// ErrParameterMissing indicates that a template parameter was referenced but not provided.
var ErrParameterMissing = errors.New("logtemplate: template parameter missing")
