package logger

import (
	"errors"
)

// ErrTemplateParameterMissing indicates that a template parameter was referenced but not provided.
var ErrTemplateParameterMissing = errors.New("logger: template parameter missing")
