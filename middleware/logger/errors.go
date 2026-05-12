package logger

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3/internal/logtemplate"
)

// ErrUnknownTag indicates that the logger middleware was configured with a
// format that references a tag with no registered renderer.
var ErrUnknownTag = errors.New("logger: unknown template tag")

// UnknownTagError is the typed error panicked from New when Config.Format
// references an unknown tag. Tag is the offending tag including any
// parametric suffix; Param is the parameter portion when the tag was
// parametric (empty for bare tags); Hint, when non-empty, is a human-
// readable suggestion (e.g. parametric form for a likely-mistyped bare tag).
type UnknownTagError struct {
	Tag   string
	Param string
	Hint  string
}

func (e *UnknownTagError) Error() string {
	msg := ErrUnknownTag.Error() + ": " + strconv.Quote(e.Tag)
	if e.Hint != "" {
		msg += " (" + e.Hint + ")"
	}
	return msg
}

func (*UnknownTagError) Unwrap() error {
	return ErrUnknownTag
}

// translateBuildError converts an internal logtemplate build error into the
// public middleware/logger error surface. Returns nil for non-template errors.
func translateBuildError(err error) error {
	var tagErr *logtemplate.UnknownTagError
	if errors.As(err, &tagErr) {
		return &UnknownTagError{Tag: tagErr.Tag, Param: tagErr.Param, Hint: tagErr.Hint}
	}
	return nil
}
