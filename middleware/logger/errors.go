package logger

import (
	"errors"
)

// ErrTemplateParameterMissing indicates that a template parameter was referenced but not provided.
var ErrTemplateParameterMissing = errors.New("logger: template parameter missing")

type templateParameterMissingError struct {
	param string
}

func errTemplateParameterMissing(param string) error {
	return templateParameterMissingError{param: param}
}

func (e templateParameterMissingError) Error() string {
	return ErrTemplateParameterMissing.Error() + ": " + e.param
}

func (templateParameterMissingError) Unwrap() error {
	return ErrTemplateParameterMissing
}
